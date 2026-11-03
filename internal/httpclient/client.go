package httpclient

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/textproto"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"golang.org/x/text/encoding/charmap"
)

// Client is an HTTP client executing Hurl requests, on top of net/http.
// Redirections are followed by the client itself (net/http never follows
// them), so every response of a redirection chain is kept.
//
// A Client is not safe for concurrent use.
type Client struct {
	jar        *cookieJar
	transports map[transportConfig]*http.Transport
}

func NewClient() *Client {
	return &Client{jar: newCookieJar(), transports: map[transportConfig]*http.Transport{}}
}

// Close closes the idle connections of this client.
func (c *Client) Close() {
	for _, t := range c.transports {
		t.CloseIdleConnections()
	}
}

// ExecuteWithRedirect executes spec, optionally follows redirections, and
// returns the list of calls.
func (c *Client) ExecuteWithRedirect(spec RequestSpec, options ClientOptions, l *logger.Logger) ([]Call, HttpError) {
	var calls []Call
	originalUrl := spec.Url

	// We keep the original credentials (headers, cookies, user): on each
	// redirection, they are stripped or restored by comparing the original
	// URL and the redirection target, so a chain A -> B -> A forwards
	// credentials again once back on A, as libcurl does.
	originalHeaders := spec.Headers.Clone()
	originalCookies := slices.Clone(spec.Cookies)
	originalOptionsHeaders := options.Headers.Clone()
	originalOptionsUser := options.User

	redirectCount := 0
	for {
		call, err := c.Execute(spec, options, l)
		if err != nil {
			return nil, err
		}
		if !options.FollowLocation.Follow {
			calls = append(calls, *call)
			break
		}
		requestUrl := call.Request.Url
		status := call.Response.Status
		redirectUrl, err := followLocation(requestUrl, &call.Response)
		calls = append(calls, *call)
		if err != nil {
			return nil, err
		}
		if redirectUrl == nil {
			break
		}
		l.Debug("")
		l.Debug(fmt.Sprintf("=> Redirect to %s", redirectUrl))
		l.Debug("")
		redirectCount++
		if max, ok := options.MaxRedirect.(ast.CountFinite); ok && redirectCount > max.Value {
			return nil, HttpErrorTooManyRedirect{}
		}

		method := redirectMethod(status, spec.Method)

		// Authorization and Cookie headers are filtered if the host changes,
		// unless the user trusts the redirected host (--location-trusted).
		headers := originalHeaders.Clone()
		cookies := slices.Clone(originalCookies)
		options.Headers = originalOptionsHeaders.Clone()
		options.User = originalOptionsUser
		if shouldStripCredentialsOnRedirect(originalUrl, *redirectUrl, options.FollowLocation) {
			headers.RemoveCredentials()
			cookies = nil
			options.Headers.RemoveCredentials()
			options.User = nil
		}

		// If the method changes, the body is dropped, like libcurl.
		next := RequestSpec{
			Method:  method,
			Url:     *redirectUrl,
			Headers: headers,
			Cookies: cookies,
			Body:    BodyBinary{},
		}
		if method == spec.Method {
			next.Form = spec.Form
			next.Multipart = spec.Multipart
			next.Body = spec.Body
			next.ImplicitContentType = spec.ImplicitContentType
		}
		spec = next
	}
	return calls, nil
}

func (c *Client) transport(o ClientOptions) (*http.Transport, bool, HttpError) {
	if !o.AllowReuse {
		t, err := newTransport(o, false)
		return t, true, err
	}
	key := newTransportConfig(o)
	if t, ok := c.transports[key]; ok {
		return t, false, nil
	}
	t, err := newTransport(o, true)
	if err != nil {
		return nil, false, err
	}
	c.transports[key] = t
	return t, false, nil
}

func checkSupported(o ClientOptions) HttpError {
	switch o.HttpVersion {
	case RequestedHttpVersionHttp10, RequestedHttpVersionHttp3:
		return HttpErrorUnsupportedHttpVersion{Version: o.HttpVersion}
	}
	switch {
	case o.AwsSigv4 != "":
		return HttpErrorUnsupportedFeature{Feature: "AWS SigV4 (--aws-sigv4)"}
	case o.Digest:
		return HttpErrorUnsupportedFeature{Feature: "Digest authentication (--digest)"}
	case o.Ntlm:
		return HttpErrorUnsupportedFeature{Feature: "NTLM authentication (--ntlm)"}
	case o.Negotiate:
		return HttpErrorUnsupportedFeature{Feature: "Negotiate authentication (--negotiate)"}
	}
	return nil
}

// Execute executes spec, without following redirections.
func (c *Client) Execute(spec RequestSpec, o ClientOptions, l *logger.Logger) (*Call, HttpError) {
	if err := checkSupported(o); err != nil {
		return nil, err
	}
	// libcurl reads the cookie file at each transfer; file cookies never
	// replace "live" cookies received from servers.
	if o.UseCookieStore && o.CookieInputFile != "" {
		c.jar.loadFile(o.CookieInputFile)
	}
	if !o.AllowReuse {
		l.Debug("Force refreshing connections because requested HTTP version change")
	}
	transport, oneShot, herr := c.transport(o)
	if herr != nil {
		return nil, herr
	}
	if oneShot {
		defer transport.CloseIdleConnections()
	}

	verbose := o.Verbosity != logger.VerbosityNone
	veryVerbose := o.Verbosity == logger.VerbosityVeryVerbose

	rawUrl := generateUrl(spec.Url, spec.Querystring)
	// libcurl rejects spaces and control characters, net/url doesn't.
	malformed := strings.ContainsFunc(rawUrl, func(r rune) bool { return r <= ' ' || r == 0x7f })
	reqUrl, err := url.Parse(rawUrl)
	if err != nil || malformed {
		return nil, HttpErrorLibcurl{Code: 3, Detail: "URL rejected: Malformed input to a URL function"}
	}
	normalizeRequestUrl(reqUrl, o.PathAsIs)

	req, body, herr := c.newRequest(spec, o, reqUrl)
	if herr != nil {
		return nil, herr
	}
	hasBodyData := len(spec.bodyBytes()) > 0 || len(spec.Form) > 0 || len(spec.Multipart) > 0

	start := time.Now()
	tr := &transferTrace{start: start}
	var proxyUrl *url.URL
	if transport.Proxy != nil {
		proxyUrl, _ = proxyFunc(o)(req)
		if proxyUrl != nil {
			tr.proxyAddr = canonicalAddr(proxyUrl)
			if reqUrl.Scheme == "https" && tunnelsHTTPS(proxyUrl) {
				tr.tunnelProxy = proxyUrl
			}
			if reqUrl.Scheme == "http" {
				for _, h := range o.ProxyHeaders.All() {
					req.Header[h.Name] = append(req.Header[h.Name], h.Value)
				}
			}
		}
	}

	ctx := context.Background()
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
	}
	ctx = context.WithValue(ctx, traceKey{}, tr)
	ctx = httptrace.WithClientTrace(ctx, tr.clientTrace())
	req = req.WithContext(ctx)

	client := &http.Client{
		Transport:     transport,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, doErr := client.Do(req)

	requestHeaders := c.replayRequestLog(tr, req, proxyUrl, body, hasBodyData, veryVerbose, l)
	if doErr != nil {
		return nil, mapTransferError(doErr, req, start, 0)
	}
	defer resp.Body.Close()

	statusLine := statusLine(resp)
	responseHeaders := responseHeaderVec(resp)
	l.DebugImportant("Response:")
	l.Debug("")
	l.DebugStatusVersionIn(statusLine)
	for _, h := range responseHeaders.All() {
		l.DebugHeaderIn(h.Name, h.Value)
	}
	l.DebugHeaderInEnd()

	responseBody, received, readErr := readBody(resp, o)
	if readErr != nil {
		return nil, mapTransferError(readErr, req, start, received)
	}
	duration := time.Since(start)

	// The maximum file size is enforced while reading; this check matches
	// Hurl's extra check (and its behaviour with --max-filesize 0).
	if o.MaxFilesize != nil && uint64(len(responseBody)) > *o.MaxFilesize {
		return nil, HttpErrorAllowedResponseSizeExceeded{MaxSize: *o.MaxFilesize}
	}

	if o.UseCookieStore {
		secure := isSecureContext(reqUrl.Scheme, reqUrl.Hostname())
		for _, v := range resp.Header.Values("Set-Cookie") {
			c.jar.setCookie(v, strings.ToLower(reqUrl.Hostname()), reqUrl.EscapedPath(), secure)
		}
	}

	version, herr := parseResponseVersion(statusLine)
	if herr != nil {
		return nil, herr
	}

	var certificate *Certificate
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		certificate = certificateFromCertInfo(newCertInfo(resp.TLS.PeerCertificates[0]))
	}

	tr.mu.Lock()
	timings := Timings{
		BeginCall:     start.UTC(),
		EndCall:       start.Add(duration).UTC(),
		NameLookup:    tr.nameLookup,
		Connect:       tr.connect,
		AppConnect:    tr.appConnect,
		PreTransfer:   tr.preTransfer,
		StartTransfer: tr.startTransfer,
		Total:         duration,
	}
	gotConn, remoteAddr := tr.gotConn, tr.remoteAddr
	tr.mu.Unlock()

	u, uerr := ParseUrl(rawUrl)
	if uerr != nil {
		return nil, httpErrorFromUrlError(uerr)
	}
	if !gotConn {
		return nil, HttpErrorNoPrimaryIp{}
	}
	request := NewRequest(string(spec.Method), u, requestHeaders, body)
	response := Response{
		Version:     version,
		Status:      uint32(resp.StatusCode),
		Headers:     responseHeaders,
		Body:        responseBody,
		Duration:    duration,
		Url:         u,
		Certificate: certificate,
		IpAddr:      NewIpAddr(primaryIp(remoteAddr)),
	}

	if verbose {
		l.Debug(fmt.Sprintf("Received %d bytes in %d ms", len(responseBody), duration.Milliseconds()))
		if veryVerbose {
			l.Debug("")
			l.DebugImportant("Response body:")
			response.LogBody(true, l)
			l.Debug("")
			timings.Log(l)
		}
	}
	return &Call{Request: request, Response: response, Timings: timings}, nil
}

// newRequest builds the net/http request for spec, and returns the request
// body bytes really sent.
func (c *Client) newRequest(spec RequestSpec, o ClientOptions, reqUrl *url.URL) (*http.Request, []byte, HttpError) {
	method := string(spec.Method)
	req := &http.Request{
		Method:     method,
		URL:        reqUrl,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
		Host:       reqUrl.Host,
	}

	var body []byte
	multipartContentType := ""
	if len(spec.Form) > 0 {
		body = []byte(urlEncodeParams(spec.Form))
	}
	if len(spec.Multipart) > 0 {
		body, multipartContentType = multipartBody(spec.Multipart)
	}
	if b := spec.bodyBytes(); len(b) > 0 {
		body, multipartContentType = b, ""
	}
	if method == "HEAD" {
		body = nil
	}

	headers := spec.Headers.Clone()
	headers.Extend(o.Headers)
	toAdd := headers.Clone()
	toRemove := slices.Clone(o.NoHeaders)

	if !headers.ContainsKey(HeaderContentType) {
		if spec.ImplicitContentType != "" {
			toAdd.Push(NewHeader(HeaderContentType, spec.ImplicitContentType))
		} else if multipartContentType != "" {
			toAdd.Push(NewHeader(HeaderContentType, "multipart/form-data"))
		}
	}
	if !headers.ContainsKey(HeaderUserAgent) {
		ua := DefaultUserAgent
		if o.UserAgent != nil {
			ua = *o.UserAgent
		}
		toAdd.Push(NewHeader(HeaderUserAgent, ua))
	}
	if o.User != nil && !headers.ContainsKey(HeaderAuthorization) {
		auth := base64.StdEncoding.EncodeToString([]byte(*o.User))
		toAdd.Push(NewHeader(HeaderAuthorization, "Basic "+auth))
	}
	if o.Compressed && !headers.ContainsKey(HeaderAcceptEncoding) {
		// Brotli is not supported by go-hurl, so we don't ask for it.
		toAdd.Push(NewHeader(HeaderAcceptEncoding, "gzip, deflate"))
	}
	removed := func(name string) bool {
		return slices.ContainsFunc(toRemove, func(r string) bool { return strings.EqualFold(r, name) })
	}
	toAdd.Retain(func(h Header) bool { return !removed(h.Name) })

	if o.Netrc || o.NetrcOptional || o.NetrcFile != "" {
		if !toAdd.ContainsKey(HeaderAuthorization) {
			auth, err := netrcAuthorization(o, reqUrl)
			if err != nil {
				return nil, nil, err
			}
			if auth != "" {
				toAdd.Push(NewHeader(HeaderAuthorization, auth))
			}
		}
	}

	// libcurl's default Accept header.
	if !toAdd.ContainsKey("Accept") && !removed("Accept") {
		req.Header["Accept"] = []string{"*/*"}
	}

	// Cookies: the cookie engine's ones, then the request ones (unless a
	// Cookie header is explicitly set), like libcurl.
	var cookies []string
	if o.UseCookieStore {
		secure := isSecureContext(reqUrl.Scheme, reqUrl.Hostname())
		for _, ck := range c.jar.cookiesFor(strings.ToLower(reqUrl.Hostname()), reqUrl.EscapedPath(), secure) {
			cookies = append(cookies, ck.name+"="+ck.value)
		}
	}
	if !toAdd.ContainsKey(HeaderCookie) {
		for _, ck := range spec.Cookies {
			cookies = append(cookies, ck.String())
		}
	}
	if len(cookies) > 0 {
		req.Header["Cookie"] = []string{strings.Join(cookies, "; ")}
	}

	for _, h := range toAdd.All() {
		switch {
		case strings.EqualFold(h.Name, "Host"):
			req.Host = h.Value
		case strings.EqualFold(h.Name, "Content-Length"):
		case strings.EqualFold(h.Name, "Transfer-Encoding"):
			if strings.EqualFold(h.Value, "chunked") {
				req.TransferEncoding = []string{"chunked"}
			}
		case strings.EqualFold(h.Name, HeaderContentType) && multipartContentType != "" &&
			strings.HasPrefix(strings.ToLower(h.Value), "multipart/") && !strings.Contains(h.Value, "boundary="):
			req.Header[h.Name] = append(req.Header[h.Name], h.Value+multipartContentType)
		default:
			req.Header[h.Name] = append(req.Header[h.Name], h.Value)
		}
	}
	if !toAdd.ContainsKey(HeaderUserAgent) {
		// An empty User-Agent stops net/http from sending its default one.
		req.Header[HeaderUserAgent] = []string{""}
	}

	if len(body) > 0 {
		req.ContentLength = int64(len(body))
		if req.TransferEncoding != nil {
			req.ContentLength = -1
		}
		getBody := func() (io.ReadCloser, error) {
			var r io.Reader = bytes.NewReader(body)
			if o.MaxSendSpeed > 0 {
				r = newRateLimitedReader(r, o.MaxSendSpeed)
			}
			return io.NopCloser(r), nil
		}
		req.Body, _ = getBody()
		req.GetBody = getBody
	} else if method == "POST" || method == "PUT" || method == "PATCH" {
		// net/http sends "Content-Length: 0" for these methods without a
		// body, libcurl doesn't. An empty body of unknown length with the
		// identity encoding is sent with neither Content-Length nor chunking.
		getBody := func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(nil)), nil }
		req.Body, _ = getBody()
		req.GetBody = getBody
		if req.TransferEncoding == nil {
			req.TransferEncoding = []string{"identity"}
		}
	}
	return req, body, nil
}

// replayRequestLog logs the request part of the transfer (the "**" engine
// lines, request headers and body) on the caller goroutine, and returns the
// request headers really sent.
func (c *Client) replayRequestLog(tr *transferTrace, req *http.Request, proxyUrl *url.URL, body []byte, hasBodyData, veryVerbose bool, l *logger.Logger) HeaderVec {
	tr.mu.Lock()
	events := slices.Clone(tr.events)
	raw := slices.Clone(tr.reqHeaders)
	h2 := tr.h2
	tr.mu.Unlock()

	var headers HeaderVec
	names := map[string]string{"user-agent": HeaderUserAgent, "content-length": "Content-Length", ":authority": "Host"}
	for k := range req.Header {
		names[strings.ToLower(k)] = k
	}
	for _, h := range raw {
		if h2 {
			if n, ok := names[strings.ToLower(h.Name)]; ok {
				h.Name = n
			}
		}
		headers.Push(h)
	}

	for _, e := range events {
		switch e.kind {
		case eventChatter:
			if veryVerbose {
				l.DebugCurl(e.text)
			}
		case eventConnectOut:
			l.DebugMethodVersionOut(e.text)
			for _, h := range e.headers {
				l.DebugHeaderOut(h.Name, h.Value)
			}
			l.DebugHeaderOutEnd()
		case eventConnectIn:
			l.DebugStatusVersionIn(e.text)
			for _, h := range e.headers {
				l.DebugHeaderIn(h.Name, h.Value)
			}
			l.DebugHeaderInEnd()
		case eventHeaders:
			target := req.URL.RequestURI()
			if proxyUrl != nil && req.URL.Scheme == "http" {
				target = req.URL.String()
			}
			version := "HTTP/1.1"
			if h2 {
				version = "HTTP/2"
			}
			l.DebugMethodVersionOut(req.Method + " " + target + " " + version)
			for _, h := range headers.All() {
				l.DebugHeaderOut(h.Name, h.Value)
			}
			l.DebugHeaderOutEnd()
			if !hasBodyData && veryVerbose {
				l.DebugImportant("Request body:")
				LogBody(nil, headers, true, l)
			}
		case eventRequestSent:
			if len(body) > 0 && veryVerbose {
				l.DebugImportant("Request body:")
				LogBody(body, headers, true, l)
			}
		}
	}
	return headers
}

// canonicalAddr returns the host:port of a proxy URL, with the default port
// of its scheme.
func canonicalAddr(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

func primaryIp(addr net.Addr) string {
	switch a := addr.(type) {
	case *net.TCPAddr:
		return a.IP.String()
	case nil, *net.UnixAddr:
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func statusLine(resp *http.Response) string {
	if resp.ProtoMajor == 2 {
		return "HTTP/2 " + strconv.Itoa(resp.StatusCode)
	}
	return decodeHeaderValue(strings.TrimSpace(resp.Proto + " " + resp.Status))
}

// parseResponseVersion parses the HTTP version of a status line.
func parseResponseVersion(line string) (HttpVersion, HttpError) {
	switch {
	case strings.HasPrefix(line, "HTTP/1.0"):
		return HttpVersionHttp10, nil
	case strings.HasPrefix(line, "HTTP/1.1"):
		return HttpVersionHttp11, nil
	case strings.HasPrefix(line, "HTTP/2"):
		return HttpVersionHttp2, nil
	case strings.HasPrefix(line, "HTTP/3"):
		return HttpVersionHttp3, nil
	}
	return 0, HttpErrorCouldNotParseResponse{}
}

// responseHeaderVec converts the response headers. net/http doesn't keep the
// order and the case of names: headers are sorted by name, canonicalized
// for HTTP/1 and lower cased for HTTP/2 (as on the wire).
func responseHeaderVec(resp *http.Response) HeaderVec {
	h := resp.Header.Clone()
	if h == nil {
		h = http.Header{}
	}
	if len(resp.TransferEncoding) > 0 && h.Get("Transfer-Encoding") == "" {
		h["Transfer-Encoding"] = []string{strings.Join(resp.TransferEncoding, ", ")}
	}
	// net/http removes "Connection: close" from HTTP/1.1 responses.
	if resp.Close && resp.ProtoMajor == 1 && resp.ProtoMinor == 1 && h.Get("Connection") == "" {
		h["Connection"] = []string{"close"}
	}
	var v HeaderVec
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for k := range h {
			if !yield(k) {
				return
			}
		}
	}) {
		display := decodeHeaderValue(unescapeHeaderName(name))
		if resp.ProtoMajor == 2 {
			display = strings.ToLower(display)
		}
		for _, value := range h[name] {
			v.Push(NewHeader(display, decodeHeaderValue(value)))
		}
	}
	return v
}

// decodeHeaderValue decodes a header received as bytes: UTF-8, or
// windows-1252 (the WHATWG ISO-8859-1) like Hurl.
func decodeHeaderValue(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	if d, err := charmap.Windows1252.NewDecoder().String(s); err == nil {
		return d
	}
	return s
}

// generateUrl appends the query string params to url.
func generateUrl(u Url, params []Param) string {
	raw := u.Raw()
	if len(params) == 0 {
		return raw
	}
	switch {
	case strings.HasSuffix(raw, "?"):
	case strings.Contains(raw, "?"):
		raw += "&"
	default:
		raw += "?"
	}
	return raw + urlEncodeParams(params)
}

// urlEncodeParams encodes params like libcurl's curl_easy_escape (names are
// not encoded, as in Hurl).
func urlEncodeParams(params []Param) string {
	var parts []string
	for _, p := range params {
		parts = append(parts, p.Name+"="+curlEscape(p.Value))
	}
	return strings.Join(parts, "&")
}

func curlEscape(s string) string {
	var b strings.Builder
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// normalizeRequestUrl removes dot segments from the path (unless pathAsIs)
// and percent-encodes the characters of the query that can't be sent as is,
// as libcurl's URL parser does.
func normalizeRequestUrl(u *url.URL, pathAsIs bool) {
	if !pathAsIs {
		escaped := u.EscapedPath()
		cleaned := removeDotSegments(escaped)
		if cleaned != escaped {
			if p, err := url.PathUnescape(cleaned); err == nil {
				u.Path, u.RawPath = p, cleaned
			}
		}
	}
	u.RawQuery = escapeQuery(u.RawQuery)
	u.Fragment, u.RawFragment = "", ""
}

func escapeQuery(q string) string {
	var b strings.Builder
	for i := range len(q) {
		c := q[i]
		if c <= ' ' || c >= 0x7f || c == '"' || c == '<' || c == '>' || c == '`' {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// removeDotSegments implements RFC 3986 section 5.2.4.
func removeDotSegments(path string) string {
	if !strings.Contains(path, ".") {
		return path
	}
	var out []string
	segments := strings.Split(path, "/")
	for i, s := range segments {
		last := i == len(segments)-1
		switch s {
		case ".":
			if last {
				out = append(out, "")
			}
		case "..":
			if len(out) > 1 {
				out = out[:len(out)-1]
			}
			if last {
				out = append(out, "")
			}
		default:
			out = append(out, s)
		}
	}
	r := strings.Join(out, "/")
	if strings.HasPrefix(path, "/") && !strings.HasPrefix(r, "/") {
		r = "/" + r
	}
	return r
}

// multipartBody encodes params as multipart/form-data, and returns the
// boundary parameter to append to the content type.
func multipartBody(params []MultipartParam) ([]byte, string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	boundary := curlBoundary()
	_ = w.SetBoundary(boundary)
	for _, p := range params {
		h := textproto.MIMEHeader{}
		switch p := p.(type) {
		case Param:
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, escapeFormName(p.Name)))
			part, _ := w.CreatePart(h)
			part.Write([]byte(p.Value))
		case FileParam:
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, escapeFormName(p.Name), escapeFormName(p.Filename)))
			h.Set("Content-Type", p.ContentType)
			part, _ := w.CreatePart(h)
			part.Write(p.Data)
		}
	}
	w.Close()
	return buf.Bytes(), "; boundary=" + boundary
}

// curlBoundary returns a boundary like libcurl's: 24 dashes and 22 random
// alphanumeric chars.
func curlBoundary() string {
	const chars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 22)
	rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return strings.Repeat("-", 24) + string(b)
}

func escapeFormName(s string) string {
	return strings.NewReplacer(`"`, "%22", "\r", "%0D", "\n", "%0A").Replace(s)
}

// followLocation returns the URL to follow, for a 3xx response with a
// Location header.
func followLocation(requestUrl Url, response *Response) (*Url, HttpError) {
	if response.Status < 300 || response.Status >= 400 {
		return nil, nil
	}
	location, ok := response.Headers.Get(HeaderLocation)
	if !ok {
		return nil, nil
	}
	u, err := requestUrl.Join(location.Value)
	if err != nil {
		return nil, httpErrorFromUrlError(err)
	}
	return &u, nil
}

// shouldStripCredentialsOnRedirect reports whether credentials
// (Authorization and Cookie headers) are filtered on a redirection from
// originalUrl to redirectUrl.
func shouldStripCredentialsOnRedirect(originalUrl, redirectUrl Url, follow FollowLocation) bool {
	if follow.Follow && follow.Forwarding == AllHosts {
		return false
	}
	if originalUrl.Scheme() != redirectUrl.Scheme() {
		return true
	}
	if originalUrl.Host() != redirectUrl.Host() {
		return true
	}
	return originalUrl.Port() != redirectUrl.Port()
}

// redirectMethod returns the method of a redirection, like curl: 301, 302
// and 303 switch to GET.
func redirectMethod(status uint32, original Method) Method {
	if status >= 301 && status <= 303 {
		return "GET"
	}
	return original
}
