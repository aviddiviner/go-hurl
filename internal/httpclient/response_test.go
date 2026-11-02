package httpclient

import (
	"reflect"
	"testing"
	"time"
)

func strp(s string) *string { return &s }

func TestParseCookieAttribute(t *testing.T) {
	tests := []struct {
		in   string
		want CookieAttribute
	}{
		{"Expires=Wed, 21 Oct 2015 07:28:00 GMT", CookieAttribute{"Expires", strp("Wed, 21 Oct 2015 07:28:00 GMT")}},
		{"HttpOnly", CookieAttribute{"HttpOnly", nil}},
		{"httponly", CookieAttribute{"httponly", nil}},
	}
	for _, tt := range tests {
		got, ok := parseCookieAttribute(tt.in)
		if !ok || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: got %+v", tt.in, got)
		}
	}
	if _, ok := parseCookieAttribute(""); ok {
		t.Error("empty")
	}
}

type cookieExpect struct {
	expires, domain, path, samesite *string
	maxAge                          *int64
	secure, httpOnly                bool
}

func checkResponseCookie(t *testing.T, c ResponseCookie, e cookieExpect) {
	t.Helper()
	opt := func(v string, ok bool) *string {
		if !ok {
			return nil
		}
		return &v
	}
	eq := func(name string, got, want *string) {
		if (got == nil) != (want == nil) || (got != nil && *got != *want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
	eq("expires", opt(c.Expires()), e.expires)
	eq("domain", opt(c.Domain()), e.domain)
	eq("path", opt(c.Path()), e.path)
	eq("samesite", opt(c.SameSite()), e.samesite)
	maxAge, ok := c.MaxAge()
	if ok != (e.maxAge != nil) || (ok && maxAge != *e.maxAge) {
		t.Errorf("max-age: got %d %v", maxAge, ok)
	}
	if c.HasSecure() != e.secure || c.HasHttpOnly() != e.httpOnly {
		t.Errorf("secure/httponly: %v %v", c.HasSecure(), c.HasHttpOnly())
	}
}

func mustParseResponseCookie(t *testing.T, s string) ResponseCookie {
	t.Helper()
	c, ok := ParseResponseCookie(s)
	if !ok {
		t.Fatalf("can't parse %q", s)
	}
	return c
}

func TestSessionCookie(t *testing.T) {
	cookie := ResponseCookie{Name: "sessionId", Value: "38afes7a8"}
	if got := mustParseResponseCookie(t, "sessionId=38afes7a8"); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	checkResponseCookie(t, cookie, cookieExpect{})
}

func TestPermanentCookie(t *testing.T) {
	cookie := ResponseCookie{Name: "id", Value: "a3fWa", Attributes: []CookieAttribute{{"Expires", strp("Wed, 21 Oct 2015 07:28:00 GMT")}}}
	if got := mustParseResponseCookie(t, "id=a3fWa; Expires=Wed, 21 Oct 2015 07:28:00 GMT"); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	checkResponseCookie(t, cookie, cookieExpect{expires: strp("Wed, 21 Oct 2015 07:28:00 GMT")})
}

func TestPermanent2Cookie(t *testing.T) {
	cookie := ResponseCookie{Name: "id", Value: "a3fWa", Attributes: []CookieAttribute{{"Max-Age", strp("2592000")}}}
	if got := mustParseResponseCookie(t, "id=a3fWa; Max-Age=2592000"); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	maxAge := int64(2592000)
	checkResponseCookie(t, cookie, cookieExpect{maxAge: &maxAge})
}

func TestLsidCookie(t *testing.T) {
	cookie := ResponseCookie{Name: "LSID", Value: "DQAAAK…Eaem_vYg", Attributes: []CookieAttribute{
		{"Path", strp("/accounts")},
		{"Expires", strp("Wed, 13 Jan 2021 22:23:01 GMT")},
		{"Secure", nil},
		{"HttpOnly", nil},
	}}
	if got := mustParseResponseCookie(t, "LSID=DQAAAK…Eaem_vYg; Path=/accounts; Expires=Wed, 13 Jan 2021 22:23:01 GMT; Secure; HttpOnly"); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	checkResponseCookie(t, cookie, cookieExpect{expires: strp("Wed, 13 Jan 2021 22:23:01 GMT"), path: strp("/accounts"), secure: true, httpOnly: true})
}

func TestHsidCookie(t *testing.T) {
	cookie := ResponseCookie{Name: "HSID", Value: "AYQEVn…DKrdst", Attributes: []CookieAttribute{
		{"Domain", strp(".foo.com")},
		{"Path", strp("/")},
		{"Expires", strp("Wed, 13 Jan 2021 22:23:01 GMT")},
		{"HttpOnly", nil},
	}}
	if got := mustParseResponseCookie(t, "HSID=AYQEVn…DKrdst; Domain=.foo.com; Path=/; Expires=Wed, 13 Jan 2021 22:23:01 GMT; HttpOnly"); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	checkResponseCookie(t, cookie, cookieExpect{expires: strp("Wed, 13 Jan 2021 22:23:01 GMT"), domain: strp(".foo.com"), path: strp("/"), httpOnly: true})
}

func TestTrailingSemicolon(t *testing.T) {
	if got := mustParseResponseCookie(t, "xx=yy;"); !reflect.DeepEqual(got, ResponseCookie{Name: "xx", Value: "yy"}) {
		t.Errorf("got %+v", got)
	}
}

func TestInvalidCookie(t *testing.T) {
	if _, ok := ParseResponseCookie("xx"); ok {
		t.Error("xx")
	}
}

func TestCookieWithInvalidAttributes(t *testing.T) {
	cookie := ResponseCookie{Name: "id", Value: "a3fWa", Attributes: []CookieAttribute{{"Secure", strp("0")}, {"Max-Age", strp("")}}}
	if got := mustParseResponseCookie(t, "id=a3fWa; Secure=0; Max-Age="); !reflect.DeepEqual(got, cookie) {
		t.Errorf("got %+v", got)
	}
	checkResponseCookie(t, cookie, cookieExpect{})
}

func TestParseContentEncoding(t *testing.T) {
	if e, err := ParseContentEncoding("br"); err != nil || e != ContentEncodingBrotli {
		t.Error(e, err)
	}
	if _, err := ParseContentEncoding("xx"); err != (HttpErrorUnsupportedContentEncoding{Detail: "xx"}) {
		t.Error(err)
	}
}

func TestResponseContentEncoding(t *testing.T) {
	r := defaultResponse(t)
	if e, err := r.Headers.ContentEncodings(); err != nil || len(e) != 0 {
		t.Error(e, err)
	}
	r.Headers = NewHeaderVec(NewHeader("Content-Encoding", "xx"))
	if _, err := r.Headers.ContentEncodings(); err != (HttpErrorUnsupportedContentEncoding{Detail: "xx"}) {
		t.Error(err)
	}
	r.Headers = NewHeaderVec(NewHeader("Content-Encoding", "br"))
	if e, _ := r.Headers.ContentEncodings(); !reflect.DeepEqual(e, []ContentEncoding{ContentEncodingBrotli}) {
		t.Error(e)
	}
	r.Headers = NewHeaderVec(NewHeader("Content-Encoding", "br, identity"))
	if e, _ := r.Headers.ContentEncodings(); !reflect.DeepEqual(e, []ContentEncoding{ContentEncodingBrotli, ContentEncodingIdentity}) {
		t.Error(e)
	}
}

var brotliHelloWorld = []byte{0x21, 0x2c, 0x00, 0x04, 0x48, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x57, 0x6f, 0x72, 0x6c, 0x64, 0x21, 0x03}

// Brotli is dropped in go-hurl: the Rust tests decoding brotli become tests
// of the unsupported content encoding error.
func TestUncompressBody(t *testing.T) {
	for _, enc := range []string{"br", "br, identity"} {
		r := defaultResponse(t)
		r.Headers = NewHeaderVec(NewHeader("Content-Encoding", enc))
		r.Body = brotliHelloWorld
		_, err := r.UncompressBody()
		if err != (HttpErrorUnsupportedContentEncoding{Detail: "br"}) {
			t.Errorf("%s: got %v", enc, err)
		}
		if err != nil && err.Message() != "compression br is not supported" {
			t.Error(err.Message())
		}
	}
	r := defaultResponse(t)
	r.Body = []byte("Hello World!")
	if b, err := r.UncompressBody(); err != nil || string(b) != "Hello World!" {
		t.Error(b, err)
	}
}

func TestUncompressGzip(t *testing.T) {
	data := []byte{
		0x1f, 0x8b, 0x08, 0x08, 0xa7, 0x52, 0x85, 0x5f, 0x00, 0x03, 0x64, 0x61, 0x74, 0x61,
		0x2e, 0x74, 0x78, 0x74, 0x00, 0xf3, 0x48, 0xcd, 0xc9, 0xc9, 0x57, 0x08, 0xcf, 0x2f,
		0xca, 0x49, 0x51, 0x04, 0x00, 0xa3, 0x1c, 0x29, 0x1c, 0x0c, 0x00, 0x00, 0x00,
	}
	if b, err := uncompressGzip(data); err != nil || string(b) != "Hello World!" {
		t.Error(b, err)
	}
}

func TestUncompressZlib(t *testing.T) {
	data := []byte{
		0x78, 0x9c, 0xf3, 0x48, 0xcd, 0xc9, 0xc9, 0x57, 0x08, 0xcf, 0x2f, 0xca, 0x49, 0x51,
		0x04, 0x00, 0x1c, 0x49, 0x04, 0x3e,
	}
	if b, err := uncompressZlib(data); err != nil || string(b) != "Hello World!" {
		t.Error(b, err)
	}
}

func TestUncompressError(t *testing.T) {
	if _, err := uncompressGzip([]byte{0x21}); err != (HttpErrorCouldNotUncompressResponse{Detail: "gzip"}) {
		t.Error(err)
	}
	if _, err := uncompressZlib([]byte{0x21}); err != (HttpErrorCouldNotUncompressResponse{Detail: "zlib"}) {
		t.Error(err)
	}
}

func helloResponse(t *testing.T) Response {
	r := defaultResponse(t)
	r.Body = []byte("Hello World!")
	return r
}

func utf8EncodingResponse(t *testing.T) Response {
	r := defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Type", "text/plain; charset=utf-8"))
	r.Body = []byte{0x63, 0x61, 0x66, 0xc3, 0xa9}
	return r
}

func latin1EncodingResponse(t *testing.T) Response {
	r := defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Type", "text/plain; charset=ISO-8859-1"))
	r.Body = []byte{0x63, 0x61, 0x66, 0xe9}
	return r
}

func TestResponseContentType(t *testing.T) {
	hello, utf8r, latin1 := helloResponse(t), utf8EncodingResponse(t), latin1EncodingResponse(t)
	if _, ok := hello.Headers.ContentType(); ok {
		t.Error("hello")
	}
	if ct, _ := utf8r.Headers.ContentType(); ct != "text/plain; charset=utf-8" {
		t.Error(ct)
	}
	if ct, _ := latin1.Headers.ContentType(); ct != "text/plain; charset=ISO-8859-1" {
		t.Error(ct)
	}
}

func TestResponseCharacterEncoding(t *testing.T) {
	for _, tt := range []struct {
		r    Response
		want string
	}{{helloResponse(t), "UTF-8"}, {utf8EncodingResponse(t), "UTF-8"}, {latin1EncodingResponse(t), "windows-1252"}} {
		if enc, err := tt.r.Headers.CharacterEncoding(); err != nil || enc.Name() != tt.want {
			t.Error(enc, err)
		}
	}
}

func TestResponseText(t *testing.T) {
	for _, tt := range []struct {
		r    Response
		want string
	}{{helloResponse(t), "Hello World!"}, {utf8EncodingResponse(t), "café"}, {latin1EncodingResponse(t), "café"}} {
		if s, err := tt.r.Text(); err != nil || s != tt.want {
			t.Error(s, err)
		}
	}
}

func TestInvalidCharset(t *testing.T) {
	r := defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Type", "test/plain; charset=xxx"))
	r.Body = []byte("Hello World!")
	if _, err := r.Headers.CharacterEncoding(); err != (HttpErrorInvalidCharset{Charset: "xxx"}) {
		t.Error(err)
	}
}

func TestInvalidDecoding(t *testing.T) {
	r := defaultResponse(t)
	r.Body = []byte{0x63, 0x61, 0x66, 0xe9}
	if _, err := r.Text(); err != (HttpErrorInvalidDecoding{Charset: "UTF-8"}) {
		t.Error(err)
	}
	r = defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Type", "text/plain; charset=ISO-8859-1"))
	r.Body = []byte{0x63, 0x61, 0x66, 0xc3, 0xa9}
	if s, err := r.Text(); err != nil || s != "cafÃ©" {
		t.Error(s, err)
	}
}

func TestGzipText(t *testing.T) {
	r := defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Encoding", "gzip"))
	r.Body = []byte{
		0x1f, 0x8b, 0x08, 0x08, 0xa7, 0x52, 0x85, 0x5f, 0x00, 0x03, 0x64, 0x61, 0x74, 0x61,
		0x2e, 0x74, 0x78, 0x74, 0x00, 0xf3, 0x48, 0xcd, 0xc9, 0xc9, 0x57, 0x08, 0xcf, 0x2f,
		0xca, 0x49, 0x51, 0x04, 0x00, 0xa3, 0x1c, 0x29, 0x1c, 0x0c, 0x00, 0x00, 0x00,
	}
	if s, err := r.Text(); err != nil || s != "Hello World!" {
		t.Error(s, err)
	}
}

func TestParseSubject(t *testing.T) {
	attributes := map[string]string{"subject": "C=US, ST=Denial, L=Springfield, O=Dis, CN=localhost"}
	if s := parseCertString(attributes, subjectAttribute); s == nil || *s != "C=US, ST=Denial, L=Springfield, O=Dis, CN=localhost" {
		t.Error(s)
	}
}

func TestParseStartDate(t *testing.T) {
	want := time.Date(2023, 1, 10, 8, 29, 52, 0, time.UTC)
	for _, v := range []string{"Jan 10 08:29:52 2023 GMT", "2023-01-10 08:29:52 GMT"} {
		d := parseCertDate(map[string]string{"start date": v}, startDateAttribute)
		if d == nil || !d.Equal(want) {
			t.Errorf("%s: %v", v, d)
		}
	}
}

func TestParseSerialNumber(t *testing.T) {
	want := "1e:e8:b1:7f:1b:64:d8:d6:b3:de:87:01:03:d2:a4:f5:33:53:5a:b0"
	for _, v := range []string{"1e:e8:b1:7f:1b:64:d8:d6:b3:de:87:01:03:d2:a4:f5:33:53:5a:b0:", "1ee8b17f1b64d8d6b3de870103d2a4f533535ab0"} {
		if s := parseSerialNumber(map[string]string{"serial number": v}); s == nil || *s != want {
			t.Errorf("%s: %v", v, s)
		}
	}
}

func TestParseSubjectAltName(t *testing.T) {
	v := "DNS:localhost, IP address:127.0.0.1, IP address:0:0:0:0:0:0:0:1"
	if s := parseCertString(map[string]string{"x509v3 subject alternative name": v}, subjectAltNameAttribute); s == nil || *s != v {
		t.Error(s)
	}
}

func TestCertificateFromCertInfo(t *testing.T) {
	got := certificateFromCertInfo(certInfo{data: []string{
		"Subject:C = US, ST = Denial, L = Springfield, O = Dis, CN = localhost",
		"Issuer:C = US, ST = Denial, L = Springfield, O = Dis, CN = localhost",
		"Serial Number:1ee8b17f1b64d8d6b3de870103d2a4f533535ab0",
		"Start date:Jan 10 08:29:52 2023 GMT",
		"Expire date:Oct 30 08:29:52 2025 GMT",
		"x509v3 subject alternative name:DNS:localhost, IP address:127.0.0.1, IP address:0:0:0:0:0:0:0:1",
	}})
	start := time.Date(2023, 1, 10, 8, 29, 52, 0, time.UTC)
	expire := time.Date(2025, 10, 30, 8, 29, 52, 0, time.UTC)
	want := NewCertificate(
		strp("C = US, ST = Denial, L = Springfield, O = Dis, CN = localhost"),
		strp("C = US, ST = Denial, L = Springfield, O = Dis, CN = localhost"),
		&start, &expire,
		strp("1e:e8:b1:7f:1b:64:d8:d6:b3:de:87:01:03:d2:a4:f5:33:53:5a:b0"),
		strp("DNS:localhost, IP address:127.0.0.1, IP address:0:0:0:0:0:0:0:1"),
		nil,
	)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestCharsetDecode(t *testing.T) {
	gbk := CharsetForLabel("gb2312")
	if gbk == nil || gbk.Name() != "GBK" {
		t.Fatal(gbk)
	}
	if s, ok := gbk.Decode([]byte{0xc4, 0xe3, 0xba, 0xc3}); !ok || s != "你好" {
		t.Error(s, ok)
	}
	if _, ok := gbk.Decode([]byte{0xc4}); ok {
		t.Error("truncated GBK should fail")
	}
	if CharsetForLabel("xxx") != nil {
		t.Error("xxx")
	}
}
