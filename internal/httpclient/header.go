package httpclient

import "strings"

const (
	HeaderAcceptEncoding  = "Accept-Encoding"
	HeaderAuthorization   = "Authorization"
	HeaderCookie          = "Cookie"
	HeaderContentEncoding = "Content-Encoding"
	HeaderContentType     = "Content-Type"
	HeaderExpect          = "Expect"
	HeaderLocation        = "Location"
	HeaderSetCookie       = "Set-Cookie"
	HeaderUserAgent       = "User-Agent"
)

// Header is an HTTP header.
type Header struct {
	Name  string
	Value string
}

func NewHeader(name, value string) Header { return Header{Name: name, Value: value} }

func (h Header) String() string { return h.Name + ": " + h.Value }

// NameEq reports whether this header name is equal to name, case
// insensitively.
func (h Header) NameEq(name string) bool { return strings.EqualFold(h.Name, name) }

// ParseHeader parses an HTTP header line received from the server. It
// returns false if line can't be parsed.
func ParseHeader(line string) (Header, bool) {
	name, value, ok := strings.Cut(line, ":")
	if !ok {
		return Header{}, false
	}
	return NewHeader(strings.TrimSpace(name), strings.TrimSpace(value)), true
}

// HeaderVec is a list of headers, in insertion order.
type HeaderVec struct {
	headers []Header
}

func NewHeaderVec(headers ...Header) HeaderVec {
	return HeaderVec{headers: headers}
}

// Get returns the first header named name.
func (v HeaderVec) Get(name string) (Header, bool) {
	for _, h := range v.headers {
		if h.NameEq(name) {
			return h, true
		}
	}
	return Header{}, false
}

// GetAll returns all the headers named name.
func (v HeaderVec) GetAll(name string) []Header {
	var r []Header
	for _, h := range v.headers {
		if h.NameEq(name) {
			r = append(r, h)
		}
	}
	return r
}

// ContainsKey reports whether there is at least one header named name.
func (v HeaderVec) ContainsKey(name string) bool {
	_, ok := v.Get(name)
	return ok
}

// Retain keeps only the headers for which f returns true.
func (v *HeaderVec) Retain(f func(Header) bool) {
	r := v.headers[:0:0]
	for _, h := range v.headers {
		if f(h) {
			r = append(r, h)
		}
	}
	v.headers = r
}

// All returns all the headers. The slice must not be modified.
func (v HeaderVec) All() []Header { return v.headers }

func (v HeaderVec) Len() int { return len(v.headers) }

func (v HeaderVec) IsEmpty() bool { return len(v.headers) == 0 }

func (v *HeaderVec) Push(h Header) { v.headers = append(v.headers, h) }

// Extend appends all the headers of other.
func (v *HeaderVec) Extend(other HeaderVec) {
	v.headers = append(v.headers, other.headers...)
}

// Values returns the values of all the headers named name.
func (v HeaderVec) Values(name string) []string {
	var r []string
	for _, h := range v.GetAll(name) {
		r = append(r, h.Value)
	}
	return r
}

func (v *HeaderVec) Clear() { v.headers = nil }

// Clone returns an independent copy.
func (v HeaderVec) Clone() HeaderVec {
	return HeaderVec{headers: append([]Header(nil), v.headers...)}
}

// RemoveCredentials removes the Authorization and Cookie headers.
func (v *HeaderVec) RemoveCredentials() {
	v.Retain(func(h Header) bool { return !h.NameEq(HeaderAuthorization) && !h.NameEq(HeaderCookie) })
}

// ContentType returns the Content-Type header value.
func (v HeaderVec) ContentType() (string, bool) {
	h, ok := v.Get(HeaderContentType)
	return h.Value, ok
}

// CharacterEncoding returns the character encoding from the Content-Type
// header, UTF-8 by default.
func (v HeaderVec) CharacterEncoding() (*Charset, HttpError) {
	contentType, ok := v.ContentType()
	if !ok {
		return CharsetUTF8, nil
	}
	label, ok := MimeCharset(contentType)
	if !ok {
		return CharsetUTF8, nil
	}
	enc := CharsetForLabel(label)
	if enc == nil {
		return nil, HttpErrorInvalidCharset{Charset: label}
	}
	return enc, nil
}

// ContentEncodings returns the list of content encodings from the
// Content-Encoding header.
func (v HeaderVec) ContentEncodings() ([]ContentEncoding, HttpError) {
	for _, h := range v.headers {
		if h.NameEq(HeaderContentEncoding) {
			var encodings []ContentEncoding
			for value := range strings.SplitSeq(h.Value, ",") {
				e, err := ParseContentEncoding(strings.TrimSpace(value))
				if err != nil {
					return nil, err
				}
				encodings = append(encodings, e)
			}
			return encodings, nil
		}
	}
	return nil, nil
}
