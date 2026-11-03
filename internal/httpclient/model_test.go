package httpclient

import (
	"reflect"
	"testing"
	"time"
)

func mustUrl(t *testing.T, s string) Url {
	t.Helper()
	u, err := ParseUrl(s)
	if err != nil {
		t.Fatalf("ParseUrl(%q): %v", s, err)
	}
	return u
}

func TestSimpleHeaderMap(t *testing.T) {
	var headers HeaderVec
	headers.Push(NewHeader("foo", "xxx"))
	headers.Push(NewHeader("bar", "yyy0"))
	headers.Push(NewHeader("bar", "yyy1"))
	headers.Push(NewHeader("bar", "yyy2"))
	headers.Push(NewHeader("baz", "zzz"))

	if headers.Len() != 5 || headers.IsEmpty() {
		t.Fatal("len")
	}
	get := func(name string) *Header {
		h, ok := headers.Get(name)
		if !ok {
			return nil
		}
		return &h
	}
	if h := get("foo"); h == nil || *h != NewHeader("foo", "xxx") {
		t.Error("foo")
	}
	if h := get("FOO"); h == nil || *h != NewHeader("foo", "xxx") {
		t.Error("FOO")
	}
	if h := get("bar"); h == nil || *h != NewHeader("bar", "yyy0") {
		t.Error("bar")
	}
	if get("qux") != nil {
		t.Error("qux")
	}
	if got := headers.GetAll("bar"); !reflect.DeepEqual(got, []Header{
		NewHeader("bar", "yyy0"), NewHeader("bar", "yyy1"), NewHeader("bar", "yyy2"),
	}) {
		t.Errorf("GetAll(bar) = %v", got)
	}
	if got := headers.GetAll("BAZ"); !reflect.DeepEqual(got, []Header{NewHeader("baz", "zzz")}) {
		t.Errorf("GetAll(BAZ) = %v", got)
	}
	if got := headers.GetAll("qux"); len(got) != 0 {
		t.Errorf("GetAll(qux) = %v", got)
	}
	if !headers.ContainsKey("FOO") || headers.ContainsKey("fuu") {
		t.Error("ContainsKey")
	}
	headers.Retain(func(h Header) bool { return h.NameEq("Bar") })
	if headers.Len() != 3 {
		t.Error("Retain")
	}
}

func TestHeaderIter(t *testing.T) {
	data := [][2]string{{"foo", "xxx"}, {"bar", "yyy0"}, {"baz", "yyy1"}}
	var headers HeaderVec
	for _, d := range data {
		headers.Push(NewHeader(d[0], d[1]))
	}
	for i, h := range headers.All() {
		if h.Name != data[i][0] || h.Value != data[i][1] {
			t.Errorf("%d: %v", i, h)
		}
	}
}

func TestParseHeader(t *testing.T) {
	if h, _ := ParseHeader("Foo: Bar\r\n"); h != NewHeader("Foo", "Bar") {
		t.Errorf("got %v", h)
	}
	if h, _ := ParseHeader("Location: http://localhost:8000/redirected\r\n"); h != NewHeader("Location", "http://localhost:8000/redirected") {
		t.Errorf("got %v", h)
	}
	if _, ok := ParseHeader("Foo"); ok {
		t.Error("Foo should not parse")
	}
}

func TestContentTypeBasic(t *testing.T) {
	headers := NewHeaderVec(
		NewHeader("Host", "localhost:8000"),
		NewHeader("Accept", "*/*"),
		NewHeader("User-Agent", "hurl/1.0"),
		NewHeader("content-type", "application/json"),
	)
	if ct, ok := headers.ContentType(); !ok || ct != "application/json" {
		t.Errorf("got %q", ct)
	}
	if _, ok := NewHeaderVec(NewHeader("foo", "bar")).ContentType(); ok {
		t.Error("no content type expected")
	}
}

func TestHeadersContentEncoding(t *testing.T) {
	headers := NewHeaderVec(NewHeader("Content-Encoding", "deflate, gzip"))
	got, err := headers.ContentEncodings()
	if err != nil || !reflect.DeepEqual(got, []ContentEncoding{ContentEncodingDeflate, ContentEncodingGzip}) {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestCharacterEncoding(t *testing.T) {
	tests := []struct{ contentType, want string }{
		{"text/html; charset=utf-8", "UTF-8"},
		{"text/plain; charset=us-ascii", "windows-1252"},
		{"text/plain", "UTF-8"},
	}
	for _, tt := range tests {
		enc, err := NewHeaderVec(NewHeader("content-type", tt.contentType)).CharacterEncoding()
		if err != nil || enc.Name() != tt.want {
			t.Errorf("%s: got %v, %v", tt.contentType, enc, err)
		}
	}
}

func helloRequest(t *testing.T) Request {
	headers := NewHeaderVec(
		NewHeader("Host", "localhost:8000"),
		NewHeader("Accept", "*/*"),
		NewHeader("User-Agent", "hurl/1.0"),
		NewHeader("content-type", "application/json"),
	)
	return NewRequest("GET", mustUrl(t, "http://localhost:8000/hello"), headers, nil)
}

func queryStringRequest(t *testing.T) Request {
	u := mustUrl(t, "http://localhost:8000/querystring-params?param1=value1&param2=&param3=a%3Db&param4=1%2C2%2C3")
	return NewRequest("GET", u, HeaderVec{}, nil)
}

func cookiesRequest(t *testing.T) Request {
	headers := NewHeaderVec(NewHeader("Cookie", "cookie1=value1; cookie2=value2"))
	return NewRequest("GET", mustUrl(t, "http://localhost:8000/cookies"), headers, nil)
}

func TestRequestContentType(t *testing.T) {
	if ct, _ := helloRequest(t).Headers.ContentType(); ct != "application/json" {
		t.Error(ct)
	}
	if _, ok := queryStringRequest(t).Headers.ContentType(); ok {
		t.Error("query")
	}
	if _, ok := cookiesRequest(t).Headers.ContentType(); ok {
		t.Error("cookies")
	}
}

func TestRequestCookies(t *testing.T) {
	if len(helloRequest(t).Cookies()) != 0 {
		t.Error("hello")
	}
	want := []RequestCookie{{"cookie1", "value1"}, {"cookie2", "value2"}}
	if got := cookiesRequest(t).Cookies(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
	if got := parseCookies("cookie1=value1; cookie2=value2"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
	if got := parseCookie("cookie1=value1"); got != (RequestCookie{"cookie1", "value1"}) {
		t.Errorf("got %v", got)
	}
}

func TestTlsVersion(t *testing.T) {
	for s, want := range map[string]TlsVersion{"1.0": TlsVersionTls10, "1.1": TlsVersionTls11, "1.2": TlsVersionTls12, "1.3": TlsVersionTls13} {
		if got, err := ParseTlsVersion(s); err != nil || got != want {
			t.Errorf("%s: %v %v", s, got, err)
		}
	}
	if _, err := ParseTlsVersion("1.4"); err == nil {
		t.Error("1.4")
	}
	if TlsVersionTls12.String() != "1.2" {
		t.Error("String")
	}
}

func TestParseUrlOk(t *testing.T) {
	for _, u := range []string{
		"http://localhost:8000/hello",
		"http://localhost:8000/querystring-params?param1=value1&param2=&param3=a%3Db&param4=1%2C2%2C3",
		"http://localhost:8000/cookies",
		"http://localhost",
		"https://localhost:8000",
		"http://localhost:8000/path-as-is/../resource",
	} {
		if _, err := ParseUrl(u); err != nil {
			t.Errorf("%s: %v", u, err)
		}
	}
}

func TestQueryParams(t *testing.T) {
	if got := mustUrl(t, "http://localhost:8000/hello").QueryParams(); len(got) != 0 {
		t.Errorf("got %v", got)
	}
	got := mustUrl(t, "http://localhost:8000/querystring-params?param1=value1&param2=&param3=a%3Db&param4=1%2C2%2C3").QueryParams()
	want := []Param{NewParam("param1", "value1"), NewParam("param2", ""), NewParam("param3", "a=b"), NewParam("param4", "1,2,3")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestUrlJoin(t *testing.T) {
	base := mustUrl(t, "http://example.net/foo/index.html")
	tests := []struct{ input, want string }{
		{"http://bar.com/redirected", "http://bar.com/redirected"},
		{"/redirected", "http://example.net/redirected"},
		{"../bar/index.html", "http://example.net/bar/index.html"},
		{"//example.org/baz/index.html", "http://example.org/baz/index.html"},
	}
	for _, tt := range tests {
		got, err := base.Join(tt.input)
		if err != nil {
			t.Fatal(err)
		}
		want := mustUrl(t, tt.want)
		if got.Raw() != want.Raw() || got.String() != want.String() {
			t.Errorf("Join(%q) = %v, want %v", tt.input, got, want)
		}
	}

	_, err := base.Join("market://details?id=com.example.package")
	if err == nil || *err != (UrlError{"market://details?id=com.example.package", "Only <http://> and <https://> schemes are supported"}) {
		t.Errorf("got %v", err)
	}
}

func TestUrlParsingError(t *testing.T) {
	_, err := ParseUrl("localhost:8000")
	if *err != (UrlError{"localhost:8000", "Missing scheme <http://> or <https://>"}) {
		t.Errorf("got %v", err)
	}
	_, err = ParseUrl("file://localhost:8000")
	if *err != (UrlError{"file://localhost:8000", "Only <http://> and <https://> schemes are supported"}) {
		t.Errorf("got %v", err)
	}
}

func TestExtractScheme(t *testing.T) {
	for _, s := range []string{"localhost:8000", "http1://localhost:8000", "://localhost:8000"} {
		if _, ok := trySchemeRe(s); ok {
			t.Errorf("%s", s)
		}
	}
	if s, _ := trySchemeRe("file://data"); s != "file://" {
		t.Error(s)
	}
	if s, _ := trySchemeRe("http://data"); s != "http://" {
		t.Error(s)
	}
}

func TestUrlAccessors(t *testing.T) {
	u := mustUrl(t, "https://127.0.0.1/index.html")
	if u.Host() != "127.0.0.1" || u.Domain() != "" {
		t.Error(u.Host(), u.Domain())
	}
	if u := mustUrl(t, "http://foo.com/index.html"); u.Host() != "foo.com" || u.Scheme() != "http" {
		t.Error(u.Host())
	}
	if p := mustUrl(t, "https://bar.com:8081/foo").Port(); p != 8081 {
		t.Error(p)
	}
	if p := mustUrl(t, "https://baz.com").Port(); p != 443 {
		t.Error(p)
	}
	if s := mustUrl(t, "http://localhost").String(); s != "http://localhost/" {
		t.Error(s)
	}
}

func defaultResponse(t *testing.T) Response {
	return Response{Version: HttpVersionHttp10, Status: 200, Url: mustUrl(t, "http://localhost")}
}

func TestGetHeaderValues(t *testing.T) {
	r := defaultResponse(t)
	r.Headers = NewHeaderVec(NewHeader("Content-Length", "12"))
	if got := r.Headers.Values("Content-Length"); !reflect.DeepEqual(got, []string{"12"}) {
		t.Error(got)
	}
	if got := r.Headers.Values("Unknown"); len(got) != 0 {
		t.Error(got)
	}
}

func TestHasTrailingNewline(t *testing.T) {
	tests := []struct {
		body string
		want bool
	}{{"", false}, {"Hello World!\n", true}, {"Hello World!", false}, {"\n", true}}
	for _, tt := range tests {
		r := defaultResponse(t)
		r.Body = []byte(tt.body)
		if r.HasTrailingNewline() != tt.want {
			t.Errorf("%q", tt.body)
		}
	}
}

func TestTimingsLog(t *testing.T) {
	tm := time.Date(2024, 1, 2, 3, 4, 5, 123_456_000, time.UTC)
	if s := FormatUtc(tm); s != "2024-01-02 03:04:05.123456 UTC" {
		t.Error(s)
	}
	if s := FormatUtc(tm.Truncate(time.Second)); s != "2024-01-02 03:04:05 UTC" {
		t.Error(s)
	}
	if s := FormatUtc(tm.Truncate(time.Millisecond)); s != "2024-01-02 03:04:05.123 UTC" {
		t.Error(s)
	}
	if s := FormatUtc(time.Date(2024, 1, 2, 3, 4, 5, 120_000_000, time.UTC)); s != "2024-01-02 03:04:05.120 UTC" {
		t.Error(s)
	}
}
