package httpclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var errMaxFilesize = errors.New("Maximum file size exceeded")

// readBody reads the response body, enforcing --limit-rate and
// --max-filesize. It returns the number of bytes received, even on error.
func readBody(resp *http.Response, o ClientOptions) ([]byte, int64, error) {
	ctx := resp.Request.Context()
	var r io.Reader = resp.Body
	if o.MaxRecvSpeed > 0 {
		lr := newRateLimitedReader(r, o.MaxRecvSpeed)
		lr.ctx = ctx
		r = lr
	}
	if o.MaxFilesize != nil && *o.MaxFilesize > 0 {
		max := *o.MaxFilesize
		if resp.ContentLength > 0 && uint64(resp.ContentLength) > max {
			return nil, 0, errMaxFilesize
		}
		r = &maxSizeReader{r: r, remaining: int64(min(max, 1<<62))}
	}
	cr := &countingReader{r: r}
	b, err := io.ReadAll(cr)
	return b, cr.n, err
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

type maxSizeReader struct {
	r         io.Reader
	remaining int64
}

func (m *maxSizeReader) Read(p []byte) (int, error) {
	n, err := m.r.Read(p)
	m.remaining -= int64(n)
	if m.remaining < 0 {
		return n, errMaxFilesize
	}
	return n, err
}

// rateLimitedReader limits the average transfer speed to rate bytes per
// second, like --limit-rate.
type rateLimitedReader struct {
	r     io.Reader
	rate  uint64
	start time.Time
	n     uint64
	ctx   context.Context
}

func newRateLimitedReader(r io.Reader, rate uint64) *rateLimitedReader {
	return &rateLimitedReader{r: r, rate: rate, ctx: context.Background()}
}

func (l *rateLimitedReader) Read(p []byte) (int, error) {
	if l.start.IsZero() {
		l.start = time.Now()
	}
	chunk := max(1, int(min(l.rate/8, 1<<20)))
	if len(p) > chunk {
		p = p[:chunk]
	}
	n, err := l.r.Read(p)
	l.n += uint64(n)
	expected := time.Duration(float64(l.n) / float64(l.rate) * float64(time.Second))
	if wait := expected - time.Since(l.start); wait > 0 {
		t := time.NewTimer(wait)
		select {
		case <-t.C:
		case <-l.ctx.Done():
			t.Stop()
			return n, l.ctx.Err()
		}
	}
	return n, err
}

// mapTransferError maps a net/http error onto a libcurl error code and
// message, so errors read like Hurl's.
func mapTransferError(err error, req *http.Request, start time.Time, received int64) HttpError {
	elapsed := time.Since(start).Milliseconds()
	if ue, ok := errors.AsType[*url.Error](err); ok {
		err = ue.Err
	}
	if he, ok := err.(HttpError); ok {
		return he
	}
	host, port := req.URL.Hostname(), req.URL.Port()
	if port == "" {
		port = "80"
		if req.URL.Scheme == "https" {
			port = "443"
		}
	}
	curl := func(code int, format string, args ...any) HttpError {
		return HttpErrorLibcurl{Code: code, Detail: fmt.Sprintf(format, args...)}
	}

	if tr := traceFromContext(req.Context()); tr != nil {
		tr.mu.Lock()
		status := tr.proxyConnectStatus
		tr.mu.Unlock()
		if status != 0 && status != http.StatusOK {
			return curl(56, "CONNECT tunnel failed, response %d", status)
		}
	}

	if de, ok := errors.AsType[*dialError](err); ok {
		switch {
		case de.dns && de.proxy:
			return curl(5, "Could not resolve proxy: %s", de.host)
		case de.dns:
			return curl(6, "Could not resolve host: %s", de.host)
		case de.timeout:
			return curl(28, "Failed to connect to %s port %s after %d ms: Timeout was reached", de.host, de.port, de.elapsed.Milliseconds())
		}
		return curl(7, "Failed to connect to %s port %s after %d ms: Couldn't connect to server", de.host, de.port, de.elapsed.Milliseconds())
	}
	if errors.Is(err, errPinnedPubKey) {
		return curl(90, "%s", errPinnedPubKey)
	}
	if errors.Is(err, errMaxFilesize) {
		return curl(63, "Maximum file size exceeded")
	}
	if msg, ok := certificateProblem(err, host); ok {
		return curl(60, "%s", msg)
	}
	if errors.Is(err, context.DeadlineExceeded) || isTimeout(err) {
		return curl(28, "Operation timed out after %d milliseconds with %d bytes received", elapsed, received)
	}
	if _, ok := errors.AsType[tls.RecordHeaderError](err); ok {
		return curl(35, "TLS connect error: %v", err)
	}
	if _, ok := errors.AsType[tls.AlertError](err); ok {
		return curl(35, "TLS connect error: %v", err)
	}
	if strings.Contains(err.Error(), "tls: ") {
		return curl(35, "TLS connect error: %v", err)
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, syscall.ECONNRESET) {
		if received == 0 && strings.Contains(err.Error(), "EOF") {
			return curl(52, "Empty reply from server")
		}
		return curl(56, "Failure when receiving data from the peer")
	}
	if strings.Contains(err.Error(), "http2:") {
		return curl(16, "%v", err)
	}
	return curl(56, "%v", err)
}

// certificateProblem returns the libcurl message of a certificate
// verification error.
func certificateProblem(err error, host string) (string, bool) {
	if _, ok := errors.AsType[x509.HostnameError](err); ok {
		return fmt.Sprintf("SSL: no alternative certificate subject name matches target host name '%s'", host), true
	}
	if ua, ok := errors.AsType[x509.UnknownAuthorityError](err); ok {
		if c := ua.Cert; c != nil && string(c.RawIssuer) == string(c.RawSubject) {
			return "SSL certificate problem: self-signed certificate", true
		}
		return "SSL certificate problem: unable to get local issuer certificate", true
	}
	if ci, ok := errors.AsType[x509.CertificateInvalidError](err); ok {
		if ci.Reason == x509.Expired {
			return "SSL certificate problem: certificate has expired", true
		}
		return "SSL certificate problem: " + ci.Error(), true
	}
	if cv, ok := errors.AsType[*tls.CertificateVerificationError](err); ok {
		return "SSL certificate problem: " + cv.Err.Error(), true
	}
	return "", false
}

// netrcAuthorization returns the basic Authorization header value from the
// netrc file for u, or "" if there is no matching machine.
func netrcAuthorization(o ClientOptions, u *url.URL) (string, HttpError) {
	if o.NetrcOptional && u.User != nil {
		return "", nil
	}
	file := o.NetrcFile
	if file == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", nil
		}
		file = filepath.Join(home, ".netrc")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		if o.NetrcOptional {
			return "", nil
		}
		return "", HttpErrorLibcurl{Code: 26, Detail: fmt.Sprintf("Couldn't read the netrc file %s", file)}
	}
	login, password, ok := parseNetrc(string(data), u.Hostname())
	if !ok {
		return "", nil
	}
	return basicAuth(login, password), nil
}

func basicAuth(login, password string) string {
	r := (&http.Request{Header: http.Header{}})
	r.SetBasicAuth(login, password)
	return r.Header.Get("Authorization")
}

// parseNetrc returns the login and password of host (or of the default
// machine) in a netrc file.
func parseNetrc(data, host string) (login, password string, found bool) {
	tokens := netrcTokens(data)
	type entry struct{ login, password string }
	var def *entry
	for i := 0; i < len(tokens); i++ {
		var e entry
		var match bool
		switch tokens[i] {
		case "machine":
			if i+1 >= len(tokens) {
				return "", "", false
			}
			i++
			match = strings.EqualFold(tokens[i], host)
		case "default":
			match = false
		case "macdef":
			continue
		default:
			continue
		}
		isDefault := tokens[i] == "default"
		for i+1 < len(tokens) && tokens[i+1] != "machine" && tokens[i+1] != "default" {
			i++
			switch tokens[i] {
			case "login":
				if i+1 < len(tokens) {
					i++
					e.login = tokens[i]
				}
			case "password":
				if i+1 < len(tokens) {
					i++
					e.password = tokens[i]
				}
			case "account":
				i++
			}
		}
		if match {
			return e.login, e.password, true
		}
		if isDefault && def == nil {
			def = &e
		}
	}
	if def != nil {
		return def.login, def.password, true
	}
	return "", "", false
}

// netrcTokens splits a netrc file into tokens, handling quoted strings and
// skipping macdef bodies.
func netrcTokens(data string) []string {
	var tokens []string
	lines := strings.Split(data, "\n")
	for li := 0; li < len(lines); li++ {
		line := lines[li]
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		rest := line
		for {
			rest = strings.TrimLeft(rest, " \t\r")
			if rest == "" {
				break
			}
			var tok string
			if rest[0] == '"' {
				var b strings.Builder
				i := 1
				for ; i < len(rest) && rest[i] != '"'; i++ {
					if rest[i] == '\\' && i+1 < len(rest) {
						i++
					}
					b.WriteByte(rest[i])
				}
				tok, rest = b.String(), rest[min(i+1, len(rest)):]
			} else {
				end := strings.IndexAny(rest, " \t\r")
				if end < 0 {
					end = len(rest)
				}
				tok, rest = rest[:end], rest[end:]
			}
			tokens = append(tokens, tok)
			if tok == "macdef" {
				for li+1 < len(lines) && strings.TrimSpace(lines[li+1]) != "" {
					li++
				}
				break
			}
		}
	}
	return tokens
}
