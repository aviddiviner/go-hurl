package httpclient

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// CurlCmd is a curl command, with arguments, equivalent to an HTTP request
// (the --curl export).
type CurlCmd struct {
	args []arg
	// stdin is the request body fed to curl's stdin, used when the body
	// contains a NUL byte (which can't be embedded in a shell string).
	stdin []byte
}

// arg is a curl argument, stored unencoded: shell encoding is only done
// when the command is rendered.
type arg interface{ encode(secrets []string) string }

// argPlain is rendered as-is if it only contains shell safe chars, quoted
// otherwise (option names, file paths, host:port values...).
type argPlain string

// argQuoted is always rendered quoted (URL, headers, data...).
type argQuoted string

// argBytes is rendered as $'\x..' (binary body).
type argBytes []byte

func (a argPlain) encode(secrets []string) string {
	s := util.Redact(string(a), secrets)
	if isShellSafe(s) {
		return s
	}
	return quote(s)
}

func (a argQuoted) encode(secrets []string) string { return quote(util.Redact(string(a), secrets)) }

func (a argBytes) encode([]string) string { return "$'" + encodeBytes(a) + "'" }

// NewCurlCmd creates a curl command from a request spec, cookies, a context
// directory, an optional output and client options.
func NewCurlCmd(spec RequestSpec, cookieStore *CookieStore, contextDir util.ContextDir, output util.Output, options ClientOptions) CurlCmd {
	args := []arg{argPlain("curl")}
	args = append(args, methodParams(spec, options.FollowLocation)...)

	headers := spec.Headers.Clone()
	headers.Extend(options.Headers)
	headers.Retain(func(h Header) bool {
		for _, name := range options.NoHeaders {
			if h.NameEq(name) {
				return false
			}
		}
		return true
	})
	args = append(args, headersParams(headers, spec.ImplicitContentType, spec.Body)...)
	args = append(args, noHeadersParams(options.NoHeaders)...)

	bodyArgs, stdin := bodyParams(spec, contextDir)
	args = append(args, bodyArgs...)
	args = append(args, cookiesParams(spec, cookieStore)...)
	args = append(args, otherOptionsParams(contextDir, output, options)...)
	args = append(args, urlParam(spec)...)
	return CurlCmd{args: args, stdin: stdin}
}

// ToRedactedString returns the shell command line, with secrets redacted.
// Secrets are redacted on the unencoded arguments, so a secret is redacted
// even if its encoded value differs (if it contains a quote for instance).
func (c CurlCmd) ToRedactedString(secrets []string) string {
	var parts []string
	for _, a := range c.args {
		parts = append(parts, a.encode(secrets))
	}
	args := strings.Join(parts, " ")
	if c.stdin != nil {
		return "printf '" + encodeBytes(c.stdin) + "' | " + args
	}
	return args
}

// CurlCommandLine returns the curl command line for spec.
func (c *Client) CurlCommandLine(spec RequestSpec, contextDir util.ContextDir, output util.Output, options ClientOptions, l *logger.Logger) CurlCmd {
	return NewCurlCmd(spec, c.CookieStore(l), contextDir, output, options)
}

func methodParams(spec RequestSpec, follow FollowLocation) []arg {
	hasBody := len(spec.Multipart) > 0 || len(spec.Form) > 0 || len(spec.bodyBytes()) > 0
	return methodCurlArgs(spec.Method, hasBody, follow)
}

func headersParams(headers HeaderVec, implicitContentType string, body Body) []arg {
	var args []arg
	for _, h := range headers.All() {
		args = append(args, headerCurlArgs(h, "--header")...)
	}
	if headers.ContainsKey(HeaderContentType) {
		return args
	}
	if implicitContentType != "" {
		if implicitContentType != "application/x-www-form-urlencoded" && implicitContentType != "multipart/form-data" {
			args = append(args, argPlain("--header"), argQuoted(HeaderContentType+": "+implicitContentType))
		}
	} else if body != nil && len(body.Bytes()) > 0 {
		contentType := ""
		if _, ok := body.(BodyBinary); ok {
			contentType = " application/octet-stream"
		}
		args = append(args, argPlain("--header"), argQuoted(HeaderContentType+":"+contentType))
	}
	return args
}

func noHeadersParams(noHeaders []string) []arg {
	var args []arg
	for _, name := range noHeaders {
		args = append(args, argPlain("--header"), argQuoted(name+":"))
	}
	return args
}

func bodyParams(spec RequestSpec, contextDir util.ContextDir) ([]arg, []byte) {
	var args []arg
	for _, p := range spec.Form {
		args = append(args, argPlain("--data"), argQuoted(paramCurlArgEscape(p)))
	}
	for _, p := range spec.Multipart {
		args = append(args, argPlain("--form"), argQuoted(multipartCurlArg(p, contextDir)))
	}
	body := spec.bodyBytes()
	if len(body) == 0 {
		return args, nil
	}
	// A body containing a NUL byte can't be passed as a shell string: in
	// this case, and only in this case, we pipe the body to curl's stdin.
	_, isFile := spec.Body.(BodyFile)
	if !isFile && bytes.IndexByte(body, 0) >= 0 {
		return append(args, argPlain("--data-binary"), argPlain("@-")), body
	}
	// With a file (@foo.bin), --data-binary is used so that curl doesn't
	// process the data (--data strips carriage returns and newlines).
	param := "--data"
	if isFile {
		param = "--data-binary"
	}
	return append(args, argPlain(param), bodyCurlArg(spec.Body, contextDir)), nil
}

func cookiesParams(spec RequestSpec, store *CookieStore) []arg {
	var all []string
	for _, c := range spec.Cookies {
		all = append(all, c.Name+"="+c.Value)
	}
	if store != nil {
		for _, c := range store.Cookies() {
			if !c.IsExpired() && c.MatchDomain(spec.Url) {
				all = append(all, c.name+"="+c.value)
			}
		}
	}
	if len(all) == 0 {
		return nil
	}
	return []arg{argPlain("--cookie"), argQuoted(strings.Join(all, "; "))}
}

func otherOptionsParams(contextDir util.ContextDir, output util.Output, options ClientOptions) []arg {
	args := clientOptionsCurlArgs(options)
	switch o := output.(type) {
	case util.OutputFile:
		args = append(args, argPlain("--output"), argPlain(contextDir.ResolvedPath(o.Path)))
	case util.OutputStdout:
		args = append(args, argPlain("--output"), argPlain("-"))
	}
	return args
}

func urlParam(spec RequestSpec) []arg {
	var params []string
	for _, p := range spec.Querystring {
		params = append(params, paramCurlArgEscape(p))
	}
	querystring := strings.Join(params, "&")
	u := spec.Url.Raw()
	if querystring != "" {
		if strings.Contains(u, "?") {
			u += "&" + querystring
		} else {
			u += "?" + querystring
		}
	}
	var args []arg
	// curl uses {}[] for globbing: we disable it with --globoff.
	if strings.ContainsAny(u, "{}[]") {
		args = append(args, argPlain("--globoff"))
	}
	return append(args, argQuoted(u))
}

func encodeByte(b byte) string { return fmt.Sprintf("\\x%02x", b) }

func encodeBytes(bs []byte) string {
	var s strings.Builder
	for _, b := range bs {
		s.WriteString(encodeByte(b))
	}
	return s.String()
}

func methodCurlArgs(m Method, hasBody bool, follow FollowLocation) []arg {
	switch m {
	case "GET":
		if hasBody {
			return []arg{argPlain("--request"), argPlain("GET")}
		}
		return nil
	case "HEAD":
		return []arg{argPlain("--head")}
	case "POST":
		// With --location, --request POST would be used for every request
		// of the redirection chain: we let curl choose the method.
		switch {
		case hasBody:
			return nil
		case follow.Follow:
			return []arg{argPlain("--data"), argQuoted("")}
		default:
			return []arg{argPlain("--request"), argPlain("POST")}
		}
	}
	return []arg{argPlain("--request"), argPlain(string(m))}
}

func headerCurlArgs(h Header, option string) []arg {
	header := h.Name + ": " + h.Value
	if h.Value == "" {
		header = h.Name + ";"
	}
	return []arg{argPlain(option), argQuoted(header)}
}

func paramCurlArgEscape(p Param) string { return p.Name + "=" + escapeUrl(p.Value) }

func paramCurlArg(p Param) string { return p.Name + "=" + p.Value }

func multipartCurlArg(p MultipartParam, contextDir util.ContextDir) string {
	switch p := p.(type) {
	case Param:
		return paramCurlArg(p)
	case FileParam:
		path := contextDir.ResolvedPath(p.Filename)
		return p.Name + "=@" + path + ";type=" + p.ContentType
	}
	return ""
}

func bodyCurlArg(b Body, contextDir util.ContextDir) arg {
	switch b := b.(type) {
	case BodyText:
		return argQuoted(b.Value)
	case BodyBinary:
		return argBytes(b.Value)
	case BodyFile:
		return argQuoted("@" + contextDir.ResolvedPath(b.Filename))
	}
	return argQuoted("")
}

func clientOptionsCurlArgs(o ClientOptions) []arg {
	var args []arg
	plain := func(s ...string) {
		for _, a := range s {
			args = append(args, argPlain(a))
		}
	}
	def := DefaultClientOptions()
	if o.AwsSigv4 != "" {
		plain("--aws-sigv4", o.AwsSigv4)
	}
	if o.CacertFile != "" {
		plain("--cacert", o.CacertFile)
	}
	if o.ClientCertFile != "" {
		plain("--cert", o.ClientCertFile)
	}
	if o.ClientKeyFile != "" {
		plain("--key", o.ClientKeyFile)
	}
	if o.Compressed {
		plain("--compressed")
	}
	if o.ConnectTimeout != def.ConnectTimeout {
		plain("--connect-timeout", strconv.FormatInt(int64(o.ConnectTimeout.Seconds()), 10))
	}
	for _, c := range o.ConnectsTo {
		plain("--connect-to", c)
	}
	if o.CookieInputFile != "" {
		plain("--cookie", o.CookieInputFile)
	}
	if o.Digest {
		plain("--digest")
	}
	switch o.HttpVersion {
	case RequestedHttpVersionHttp10:
		plain("--http1.0")
	case RequestedHttpVersionHttp11:
		plain("--http1.1")
	case RequestedHttpVersionHttp2:
		plain("--http2")
	case RequestedHttpVersionHttp2PriorKnowledge:
		plain("--http2-prior-knowledge")
	case RequestedHttpVersionHttp3:
		plain("--http3")
	}
	if o.Insecure {
		plain("--insecure")
	}
	switch o.IpResolve {
	case IpResolveIpV4:
		plain("--ipv4")
	case IpResolveIpV6:
		plain("--ipv6")
	}
	if o.FollowLocation.Follow {
		if o.FollowLocation.Forwarding == AllHosts {
			plain("--location-trusted")
		} else {
			plain("--location")
		}
	}
	if o.MaxFilesize != nil {
		plain("--max-filesize", strconv.FormatUint(*o.MaxFilesize, 10))
	}
	if o.MaxRecvSpeed != 0 {
		plain("--limit-rate", strconv.FormatUint(o.MaxRecvSpeed, 10))
	}
	// There is no --limit-rate equivalent for MaxSendSpeed: curl's
	// --limit-rate limits both upload and download.
	if o.MaxRedirect != nil && o.MaxRedirect != def.MaxRedirect {
		n := -1
		if f, ok := o.MaxRedirect.(ast.CountFinite); ok {
			n = f.Value
		}
		plain("--max-redirs", strconv.Itoa(n))
	}
	if o.Timeout != def.Timeout {
		plain("--max-time", strconv.FormatInt(int64(o.Timeout.Seconds()), 10))
	}
	if o.TlsMin != TlsVersionDefault {
		// curl has no --tls-min option: the minimum is set with --tlsv1.X.
		plain("--tlsv" + o.TlsMin.String())
	}
	if o.TlsMax != TlsVersionDefault {
		plain("--tls-max", o.TlsMax.String())
	}
	if o.Negotiate {
		plain("--negotiate")
	}
	if o.NetrcFile != "" {
		args = append(args, argPlain("--netrc-file"), argQuoted(o.NetrcFile))
	}
	if o.NetrcOptional {
		plain("--netrc-optional")
	}
	if o.Netrc {
		plain("--netrc")
	}
	if o.Ntlm {
		plain("--ntlm")
	}
	if o.PathAsIs {
		plain("--path-as-is")
	}
	if o.PinnedPubKey != "" {
		plain("--pinnedpubkey", o.PinnedPubKey)
	}
	if o.Proxy != "" {
		args = append(args, argPlain("--proxy"), argQuoted(o.Proxy))
	}
	for _, h := range o.ProxyHeaders.All() {
		args = append(args, headerCurlArgs(h, "--proxy-header")...)
	}
	for _, r := range o.Resolves {
		plain("--resolve", r)
	}
	if o.SslNoRevoke {
		plain("--ssl-no-revoke")
	}
	if o.UnixSocket != "" {
		args = append(args, argPlain("--unix-socket"), argQuoted(o.UnixSocket))
	}
	if o.User != nil {
		args = append(args, argPlain("--user"), argQuoted(*o.User))
	}
	if o.UserAgent != nil {
		args = append(args, argPlain("--user-agent"), argQuoted(*o.UserAgent))
	}
	return args
}

// escapeUrl percent-encodes all the non alphanumeric bytes.
func escapeUrl(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// isShellSafe reports whether s can be used unquoted as a shell argument.
func isShellSafe(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c < 128 && (c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-_./:=@,+%", c))) {
			return false
		}
	}
	return true
}

// quote quotes s as a single shell argument: '...', or $'...' with escape
// sequences if s contains a quote, a newline or a tab.
func quote(s string) string {
	if !strings.ContainsAny(s, "'\n\t") {
		return "'" + s + "'"
	}
	var b strings.Builder
	for _, c := range s {
		switch c {
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		case '\'':
			b.WriteString("\\'")
		case '\\':
			b.WriteString("\\\\")
		default:
			b.WriteRune(c)
		}
	}
	return "$'" + b.String() + "'"
}
