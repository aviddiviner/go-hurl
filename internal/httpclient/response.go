package httpclient

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"io"
	"strconv"
	"strings"
	"time"
)

// Response is a runtime HTTP response, really executed by our HTTP client.
type Response struct {
	Version  HttpVersion
	Status   uint32
	Headers  HeaderVec
	Body     []byte
	Duration time.Duration
	Url      Url
	// Certificate is the end-user certificate of the response certificate
	// chain, nil for plain HTTP.
	Certificate *Certificate
	IpAddr      IpAddr
}

// HasTrailingNewline reports whether the body ends with a newline.
func (r *Response) HasTrailingNewline() bool { return bytes.HasSuffix(r.Body, []byte("\n")) }

// HttpVersion is the HTTP version of an HTTP transaction.
type HttpVersion int

const (
	HttpVersionHttp10 HttpVersion = iota
	HttpVersionHttp11
	HttpVersionHttp2
	HttpVersionHttp3
)

func (v HttpVersion) String() string {
	switch v {
	case HttpVersionHttp10:
		return "HTTP/1.0"
	case HttpVersionHttp11:
		return "HTTP/1.1"
	case HttpVersionHttp2:
		return "HTTP/2"
	}
	return "HTTP/3"
}

// IpAddr is an IP address (IPv4 or IPv6), kept as a raw string.
type IpAddr struct{ raw string }

func NewIpAddr(raw string) IpAddr { return IpAddr{raw: raw} }

func (a IpAddr) String() string { return a.raw }

// ResponseCookie is a cookie returned by a Set-Cookie response header, with
// arbitrary attributes.
type ResponseCookie struct {
	Name       string
	Value      string
	Attributes []CookieAttribute
}

type CookieAttribute struct {
	Name string
	// Value is nil for attributes without value (like HttpOnly).
	Value *string
}

// ParseResponseCookie parses a Set-Cookie header value.
func ParseResponseCookie(s string) (ResponseCookie, bool) {
	name, remaining, ok := strings.Cut(s, "=")
	if !ok {
		return ResponseCookie{}, false
	}
	tokens := strings.Split(remaining, ";")
	c := ResponseCookie{Name: name, Value: tokens[0]}
	for _, t := range tokens[1:] {
		if a, ok := parseCookieAttribute(t); ok {
			c.Attributes = append(c.Attributes, a)
		}
	}
	return c, true
}

func parseCookieAttribute(s string) (CookieAttribute, bool) {
	if s == "" {
		return CookieAttribute{}, false
	}
	tokens := strings.Split(s, "=")
	a := CookieAttribute{Name: strings.TrimSpace(tokens[0])}
	if len(tokens) > 1 {
		v := tokens[1]
		a.Value = &v
	}
	return a, true
}

func (c ResponseCookie) Expires() (string, bool)  { return c.attrAsStr("Expires") }
func (c ResponseCookie) Domain() (string, bool)   { return c.attrAsStr("Domain") }
func (c ResponseCookie) Path() (string, bool)     { return c.attrAsStr("Path") }
func (c ResponseCookie) SameSite() (string, bool) { return c.attrAsStr("SameSite") }
func (c ResponseCookie) HasSecure() bool          { return c.attrAsBool("Secure") }
func (c ResponseCookie) HasHttpOnly() bool        { return c.attrAsBool("HttpOnly") }

// MaxAge returns the Max-Age attribute. An invalid integer is ignored.
func (c ResponseCookie) MaxAge() (int64, bool) {
	for _, a := range c.Attributes {
		if strings.EqualFold(a.Name, "Max-Age") && a.Value != nil {
			if v, err := strconv.ParseInt(*a.Value, 10, 64); err == nil {
				return v, true
			}
		}
	}
	return 0, false
}

func (c ResponseCookie) attrAsStr(name string) (string, bool) {
	for _, a := range c.Attributes {
		if strings.EqualFold(a.Name, name) {
			if a.Value == nil {
				return "", false
			}
			return *a.Value, true
		}
	}
	return "", false
}

func (c ResponseCookie) attrAsBool(name string) bool {
	for _, a := range c.Attributes {
		if strings.EqualFold(a.Name, name) && a.Value == nil {
			return true
		}
	}
	return false
}

// Cookies returns the cookies of the Set-Cookie headers.
func (r *Response) Cookies() []ResponseCookie {
	var cookies []ResponseCookie
	for _, h := range r.Headers.GetAll(HeaderSetCookie) {
		if c, ok := ParseResponseCookie(h.Value); ok {
			cookies = append(cookies, c)
		}
	}
	return cookies
}

// GetCookie returns the response cookie named name.
func (r *Response) GetCookie(name string) (ResponseCookie, bool) {
	for _, c := range r.Cookies() {
		if c.Name == name {
			return c, true
		}
	}
	return ResponseCookie{}, false
}

// ContentEncoding is a Content-Encoding value.
type ContentEncoding int

const (
	// ContentEncodingBrotli is recognised but can't be decoded by go-hurl.
	ContentEncodingBrotli ContentEncoding = iota
	ContentEncodingGzip
	ContentEncodingDeflate
	ContentEncodingIdentity
)

// ParseContentEncoding parses a Content-Encoding header value.
func ParseContentEncoding(s string) (ContentEncoding, HttpError) {
	switch s {
	case "br":
		return ContentEncodingBrotli, nil
	case "gzip":
		return ContentEncodingGzip, nil
	case "deflate":
		return ContentEncodingDeflate, nil
	case "identity":
		return ContentEncodingIdentity, nil
	}
	return 0, HttpErrorUnsupportedContentEncoding{Detail: s}
}

// Decode decompresses data.
func (e ContentEncoding) Decode(data []byte) ([]byte, HttpError) {
	switch e {
	case ContentEncodingGzip:
		return uncompressGzip(data)
	case ContentEncodingDeflate:
		return uncompressZlib(data)
	case ContentEncodingBrotli:
		// Brotli is dropped: no decoder in the standard library or x/.
		return nil, HttpErrorUnsupportedContentEncoding{Detail: "br"}
	}
	return data, nil
}

func uncompressGzip(data []byte) ([]byte, HttpError) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, HttpErrorCouldNotUncompressResponse{Detail: "gzip"}
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, HttpErrorCouldNotUncompressResponse{Detail: "gzip"}
	}
	return b, nil
}

func uncompressZlib(data []byte) ([]byte, HttpError) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, HttpErrorCouldNotUncompressResponse{Detail: "zlib"}
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, HttpErrorCouldNotUncompressResponse{Detail: "zlib"}
	}
	return b, nil
}

// Text returns the response body as text, decompressed and decoded with the
// charset of the Content-Type header.
func (r *Response) Text() (string, HttpError) {
	encodings, err := r.Headers.ContentEncodings()
	if err != nil {
		return "", err
	}
	body := r.Body
	if len(encodings) > 0 {
		if body, err = r.UncompressBody(); err != nil {
			return "", err
		}
	}
	charset, err := r.Headers.CharacterEncoding()
	if err != nil {
		return "", err
	}
	s, ok := charset.Decode(body)
	if !ok {
		return "", HttpErrorInvalidDecoding{Charset: charset.Name()}
	}
	return s, nil
}

func (r *Response) IsHTML() bool {
	ct, ok := r.Headers.ContentType()
	return ok && IsHTML(ct)
}

func (r *Response) IsJSON() bool {
	ct, ok := r.Headers.ContentType()
	return ok && IsJSON(ct)
}

func (r *Response) IsXML() bool {
	ct, ok := r.Headers.ContentType()
	return ok && IsXML(ct)
}

// UncompressBody decompresses the response body, using the Content-Encoding
// header.
func (r *Response) UncompressBody() ([]byte, HttpError) {
	encodings, err := r.Headers.ContentEncodings()
	if err != nil {
		return nil, err
	}
	data := r.Body
	for _, e := range encodings {
		if data, err = e.Decode(data); err != nil {
			return nil, err
		}
	}
	return data, nil
}
