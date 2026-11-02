package httpclient

import (
	"strings"
)

// Param is a key/value pair used for query params, form params and
// multipart-form params.
type Param struct {
	Name  string
	Value string
}

func NewParam(name, value string) Param { return Param{Name: name, Value: value} }

func (p Param) String() string { return p.Name + ": " + p.Value }

// RequestCookie is a cookie sent in a request.
type RequestCookie struct {
	Name  string
	Value string
}

func (c RequestCookie) String() string { return c.Name + "=" + c.Value }

// Request is a runtime HTTP request, really executed by our HTTP client.
// It's different from RequestSpec, the request asked by the user: for
// instance, headers implicitly added by the client are present here.
type Request struct {
	// Url is the absolute URL.
	Url     Url
	Method  string
	Headers HeaderVec
	Body    []byte
}

func NewRequest(method string, url Url, headers HeaderVec, body []byte) Request {
	return Request{Url: url, Method: method, Headers: headers, Body: body}
}

// Cookies returns the request cookies, from the Cookie headers.
func (r Request) Cookies() []RequestCookie {
	var cookies []RequestCookie
	for _, h := range r.Headers.GetAll(HeaderCookie) {
		cookies = append(cookies, parseCookies(strings.TrimSpace(h.Value))...)
	}
	return cookies
}

func parseCookies(s string) []RequestCookie {
	var cookies []RequestCookie
	for t := range strings.SplitSeq(s, ";") {
		cookies = append(cookies, parseCookie(strings.TrimSpace(t)))
	}
	return cookies
}

func parseCookie(s string) RequestCookie {
	name, value, _ := strings.Cut(s, "=")
	return RequestCookie{Name: name, Value: value}
}

// RequestedHttpVersion is the HTTP version requested by the user.
type RequestedHttpVersion int

const (
	// RequestedHttpVersionDefault lets the client choose the best version.
	RequestedHttpVersionDefault RequestedHttpVersion = iota
	RequestedHttpVersionHttp10
	RequestedHttpVersionHttp11
	// RequestedHttpVersionHttp2 uses HTTP/2 (negotiated with ALPN over TLS).
	RequestedHttpVersionHttp2
	// RequestedHttpVersionHttp2PriorKnowledge uses HTTP/2 without HTTP/1.1
	// upgrade.
	RequestedHttpVersionHttp2PriorKnowledge
	RequestedHttpVersionHttp3
)

func (v RequestedHttpVersion) String() string {
	switch v {
	case RequestedHttpVersionHttp10:
		return "HTTP/1.0"
	case RequestedHttpVersionHttp11:
		return "HTTP/1.1"
	case RequestedHttpVersionHttp2:
		return "HTTP/2"
	case RequestedHttpVersionHttp2PriorKnowledge:
		return "HTTP/2 (prior knowledge)"
	case RequestedHttpVersionHttp3:
		return "HTTP/3"
	}
	return "HTTP (default)"
}

// TlsVersion is a Transport Layer Security version.
type TlsVersion int

const (
	TlsVersionDefault TlsVersion = iota
	TlsVersionTls10
	TlsVersionTls11
	TlsVersionTls12
	TlsVersionTls13
)

// ParseTlsVersion parses "1.0", "1.1", "1.2" or "1.3".
func ParseTlsVersion(value string) (TlsVersion, error) {
	switch value {
	case "1.0":
		return TlsVersionTls10, nil
	case "1.1":
		return TlsVersionTls11, nil
	case "1.2":
		return TlsVersionTls12, nil
	case "1.3":
		return TlsVersionTls13, nil
	}
	return TlsVersionDefault, errTlsVersion
}

type parseError string

func (e parseError) Error() string { return string(e) }

const errTlsVersion = parseError("expecting 1.0, 1.1, 1.2, 1.3")

func (v TlsVersion) String() string {
	switch v {
	case TlsVersionTls10:
		return "1.0"
	case TlsVersionTls11:
		return "1.1"
	case TlsVersionTls12:
		return "1.2"
	case TlsVersionTls13:
		return "1.3"
	}
	return "default"
}

type IpResolve int

const (
	// IpResolveDefault can use addresses of all IP versions.
	IpResolveDefault IpResolve = iota
	IpResolveIpV4
	IpResolveIpV6
)

// CredentialForwarding says to which hosts credentials are forwarded when
// following redirections.
type CredentialForwarding int

const (
	// OnlyInitialHost forwards credentials only to the initial host.
	OnlyInitialHost CredentialForwarding = iota
	// AllHosts forwards credentials to all the hosts of the redirections.
	AllHosts
)

// FollowLocation says if redirections are followed. The zero value doesn't
// follow redirections (Rust's FollowLocation::No).
type FollowLocation struct {
	Follow     bool
	Forwarding CredentialForwarding
}

// Method is an HTTP method.
type Method string

func (m Method) String() string { return string(m) }

// RequestSpec is the HTTP request asked to be executed by the user
// (different from the runtime executed Request).
type RequestSpec struct {
	Method      Method
	Url         Url
	Headers     HeaderVec
	Querystring []Param
	Form        []Param
	Multipart   []MultipartParam
	Cookies     []RequestCookie
	Body        Body
	// ImplicitContentType is the content type implicitly set when the
	// request uses a "typed" body (form, JSON, multipart, multiline string
	// with hint). "" means none. It can differ from the Content-Type header
	// set by the user.
	ImplicitContentType string
}

// NewRequestSpec returns the default request spec: GET https://localhost
// with an empty binary body.
func NewRequestSpec() RequestSpec {
	return RequestSpec{Method: "GET", Url: DefaultUrl(), Body: BodyBinary{}}
}

func (r RequestSpec) bodyBytes() []byte {
	if r.Body == nil {
		return nil
	}
	return r.Body.Bytes()
}

// MultipartParam is a Param or a FileParam.
type MultipartParam interface {
	multipartParam()
	String() string
}

func (Param) multipartParam()     {}
func (FileParam) multipartParam() {}

type FileParam struct {
	Name        string
	Filename    string
	Data        []byte
	ContentType string
}

func (p FileParam) String() string {
	return p.Name + ": file," + p.Filename + "; " + p.ContentType
}

// Body is a request body: BodyText, BodyBinary or BodyFile.
type Body interface {
	body()
	Bytes() []byte
}

type BodyText struct{ Value string }

type BodyBinary struct{ Value []byte }

// BodyFile is a body read from Filename.
type BodyFile struct {
	Value    []byte
	Filename string
}

func (BodyText) body()   {}
func (BodyBinary) body() {}
func (BodyFile) body()   {}

func (b BodyText) Bytes() []byte   { return []byte(b.Value) }
func (b BodyBinary) Bytes() []byte { return b.Value }
func (b BodyFile) Bytes() []byte   { return b.Value }
