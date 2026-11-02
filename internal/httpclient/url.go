package httpclient

import (
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// UrlError is an error of the URL module.
type UrlError struct {
	Url    string
	Reason string
}

func (e *UrlError) Error() string { return "invalid URL <" + e.Url + "> (" + e.Reason + ")" }

func newUrlError(u, reason string) *UrlError { return &UrlError{Url: u, Reason: reason} }

// Url is a parsed absolute HTTP or HTTPS URL. It keeps the raw input of the
// user.
type Url struct {
	raw   string
	inner *url.URL
}

// DefaultUrl returns https://localhost.
func DefaultUrl() Url {
	u, _ := ParseUrl("https://localhost")
	return u
}

// ParseUrl parses an absolute URL. Only http:// and https:// schemes are
// supported.
func ParseUrl(value string) (Url, *UrlError) {
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		inner, err := url.Parse(value)
		if err != nil {
			reason := err.Error()
			if ue, ok := err.(*url.Error); ok {
				reason = ue.Err.Error()
			}
			return Url{}, newUrlError(value, reason)
		}
		if inner.Host == "" {
			return Url{}, newUrlError(value, "empty host")
		}
		return Url{raw: value, inner: inner}, nil
	}
	if _, ok := trySchemeRe(value); ok {
		return Url{}, newUrlError(value, "Only <http://> and <https://> schemes are supported")
	}
	return Url{}, newUrlError(value, "Missing scheme <http://> or <https://>")
}

var schemeRe = regexp.MustCompile(`^([a-z]+://).*`)

// trySchemeRe extracts the scheme of value.
func trySchemeRe(value string) (string, bool) {
	m := schemeRe.FindStringSubmatch(value)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// Raw returns the input URL of the user.
func (u Url) Raw() string { return u.raw }

// QueryParams returns the query parameters (values are URL decoded).
func (u Url) QueryParams() []Param {
	var params []Param
	for pair := range strings.SplitSeq(u.inner.RawQuery, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		params = append(params, NewParam(formDecode(k), formDecode(v)))
	}
	return params
}

func formDecode(s string) string {
	s = strings.ReplaceAll(s, "+", " ")
	if d, err := url.PathUnescape(s); err == nil {
		return d
	}
	return s
}

// Host returns the host of this URL (IPv6 addresses are bracketed).
func (u Url) Host() string {
	h := strings.ToLower(u.inner.Hostname())
	if strings.Contains(h, ":") {
		return "[" + h + "]"
	}
	return h
}

// Scheme returns the lower cased scheme, without ':'.
func (u Url) Scheme() string { return strings.ToLower(u.inner.Scheme) }

// Port returns the port of this URL, or the default port of the scheme.
func (u Url) Port() int {
	if p := u.inner.Port(); p != "" {
		n, _ := strconv.Atoi(p)
		return n
	}
	switch u.Scheme() {
	case "http", "ws":
		return 80
	case "https", "wss":
		return 443
	case "ftp":
		return 21
	}
	return 0
}

// Domain returns the domain of this URL, or "" if the host is an IP address.
func (u Url) Domain() string {
	h := u.inner.Hostname()
	if net.ParseIP(h) != nil {
		return ""
	}
	return strings.ToLower(h)
}

// Path returns the percent-encoded path, "/" if empty.
func (u Url) Path() string {
	p := u.inner.EscapedPath()
	if p == "" {
		return "/"
	}
	return p
}

// Join parses input as a URL, with this URL as the base URL.
func (u Url) Join(input string) (Url, *UrlError) {
	ref, err := url.Parse(input)
	if err != nil {
		return Url{}, newUrlError(u.String(), "Can not use relative path '"+input+"'")
	}
	return ParseUrl(serialize(u.inner.ResolveReference(ref)))
}

// String returns the normalized serialization of this URL.
func (u Url) String() string {
	if u.inner == nil {
		return ""
	}
	return serialize(u.inner)
}

// serialize approximates the WHATWG serialization of the url crate: lower
// cased scheme and host, default port removed and "/" for an empty path.
func serialize(in *url.URL) string {
	v := *in
	v.Scheme = strings.ToLower(v.Scheme)
	host := strings.ToLower(v.Hostname())
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	port := v.Port()
	if (v.Scheme == "http" && port == "80") || (v.Scheme == "https" && port == "443") {
		port = ""
	}
	if port != "" {
		host += ":" + port
	}
	v.Host = host
	if v.Path == "" && v.RawPath == "" {
		v.Path = "/"
	}
	return v.String()
}
