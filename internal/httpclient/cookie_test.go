package httpclient

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseCookieFromStr(t *testing.T) {
	tests := []struct {
		in   string
		want Cookie
	}{
		{"httpbin.org\tFALSE\t/\tFALSE\t0\tcookie1\tvalueA", NewCookie("httpbin.org", false, "/", false, 0, "cookie1", "valueA", false)},
		{"localhost\tFALSE\t/\tFALSE\t1\tcookie2\t", NewCookie("localhost", false, "/", false, 1, "cookie2", "", false)},
		{"localhost FALSE / FALSE 1 cookie3 value3", NewCookie("localhost", false, "/", false, 1, "cookie3", "value3", false)},
		{"#HttpOnly_localhost FALSE / FALSE 1 cookie3 a b c", NewCookie("localhost", false, "/", false, 1, "cookie3", "a b c", true)},
	}
	for _, tt := range tests {
		got, err := CookieFromNetscape(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("%q: got %+v, %v", tt.in, got, err)
		}
	}
	if _, err := CookieFromNetscape("xxx"); err != (ParseCookieError{}) {
		t.Error(err)
	}
}

func TestMatchCookie(t *testing.T) {
	cookie := NewCookie("example.com", false, "/", true, 0, "", "", false)
	if !cookie.MatchDomain(mustUrl(t, "http://example.com/toto")) ||
		cookie.MatchDomain(mustUrl(t, "http://sub.example.com/tata")) ||
		cookie.MatchDomain(mustUrl(t, "http://toto/tata")) {
		t.Error("cookie 1")
	}

	cookie = NewCookie("example.com", true, "/toto", true, 0, "", "", false)
	if !cookie.MatchDomain(mustUrl(t, "http://example.com/toto")) ||
		!cookie.MatchDomain(mustUrl(t, "http://sub.example.com/toto")) ||
		cookie.MatchDomain(mustUrl(t, "http://example.com/tata")) {
		t.Error("cookie 2")
	}

	// Legacy cookie domain with dot prefix
	cookie = NewCookie(".example.com", true, "/foo", true, 0, "", "", false)
	if !cookie.MatchDomain(mustUrl(t, "http://example.com/foo")) ||
		!cookie.MatchDomain(mustUrl(t, "http://sub.example.com/foo")) ||
		cookie.MatchDomain(mustUrl(t, "http://example.com/tata")) ||
		cookie.MatchDomain(mustUrl(t, "http://sub.example.com/tata")) {
		t.Error("cookie 3")
	}
}

func newTestStore(t *testing.T) *CookieStore {
	store := NewCookieStore()
	if err := store.AddCookie("localhost  TRUE    /   FALSE   0   cookie1 valueA"); err != nil {
		t.Fatal(err)
	}
	if err := store.AddCookie("#HttpOnly_example.com\t\t   FALSE\t\t  \t/\tFALSE\t1\tcookie2\tfoo bar baz"); err != nil {
		t.Fatal(err)
	}
	return store
}

func TestAddCookie(t *testing.T) {
	store := newTestStore(t)
	if store.Len() != 2 || store.IsEmpty() {
		t.Fatal("len")
	}
	want := []Cookie{
		NewCookie("localhost", true, "/", false, 0, "cookie1", "valueA", false),
		NewCookie("example.com", false, "/", false, 1, "cookie2", "foo bar baz", true),
	}
	if !reflect.DeepEqual(store.Cookies(), want) {
		t.Errorf("got %+v", store.Cookies())
	}
}

func TestCookieStoreToNetscape(t *testing.T) {
	want := "localhost\tTRUE\t/\tFALSE\t0\tcookie1\tvalueA\n#HttpOnly_example.com\tFALSE\t/\tFALSE\t1\tcookie2\tfoo bar baz\n"
	if got := newTestStore(t).ToNetscape(); got != want {
		t.Errorf("got %q", got)
	}
}

func newTestJar(now time.Time) *cookieJar {
	j := newCookieJar()
	j.now = func() time.Time { return now }
	return j
}

func jarValues(cookies []*jarCookie) []string {
	var r []string
	for _, c := range cookies {
		r = append(r, c.name+"="+c.value)
	}
	return r
}

func TestCookieJarDomainAndPath(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	j := newTestJar(now)
	j.setCookie("host=1", "www.example.com", "/a/b/page", false)
	j.setCookie("dom=2; Domain=example.com; Path=/", "www.example.com", "/a/b/page", false)
	j.setCookie("other=3; Domain=other.com", "www.example.com", "/", false)
	j.setCookie("tld=4; Domain=com", "www.example.com", "/", false)
	j.setCookie("sec=5; Secure", "www.example.com", "/", false)
	j.setCookie("path=6; Path=/a", "www.example.com", "/", false)

	want := []string{
		".example.com\tTRUE\t/\tFALSE\t0\tdom\t2",
		"www.example.com\tFALSE\t/a\tFALSE\t0\tpath\t6",
		"www.example.com\tFALSE\t/a/b/\tFALSE\t0\thost\t1",
	}
	got := j.list()
	if !reflect.DeepEqual(sortedCopy(got), sortedCopy(want)) {
		t.Errorf("list: got %q", got)
	}

	tests := []struct {
		host, path string
		want       []string
	}{
		{"www.example.com", "/a/b/c", []string{"host=1", "path=6", "dom=2"}},
		{"www.example.com", "/a", []string{"path=6", "dom=2"}},
		{"www.example.com", "/ab", []string{"dom=2"}},
		{"sub.example.com", "/a/b/c", []string{"dom=2"}},
		{"example.com", "/", []string{"dom=2"}},
		{"badexample.com", "/", nil},
	}
	for _, tt := range tests {
		if got := jarValues(j.cookiesFor(tt.host, tt.path, false)); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s%s: got %v, want %v", tt.host, tt.path, got, tt.want)
		}
	}
}

func sortedCopy(s []string) []string {
	c := append([]string(nil), s...)
	for i := range c {
		for j := i + 1; j < len(c); j++ {
			if c[j] < c[i] {
				c[i], c[j] = c[j], c[i]
			}
		}
	}
	return c
}

func TestCookieJarSecureAndLocalhost(t *testing.T) {
	j := newTestJar(time.Now())
	j.setCookie("a=1; Secure", "localhost", "/", isSecureContext("http", "localhost"))
	j.setCookie("b=2; Secure", "example.com", "/", isSecureContext("https", "example.com"))
	if got := jarValues(j.cookiesFor("localhost", "/", true)); !reflect.DeepEqual(got, []string{"a=1"}) {
		t.Error(got)
	}
	if got := jarValues(j.cookiesFor("example.com", "/", false)); len(got) != 0 {
		t.Error(got)
	}
	if got := jarValues(j.cookiesFor("example.com", "/", true)); !reflect.DeepEqual(got, []string{"b=2"}) {
		t.Error(got)
	}
}

func TestCookieJarExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	j := newTestJar(now)
	j.setCookie("a=1; Max-Age=100", "localhost", "/", false)
	j.setCookie("b=2; Expires=Thu, 13 Jan 2078 22:23:01 GMT", "localhost", "/", false)
	j.setCookie("c=3; Expires=Wed, 13-Jan-2021 22:23:01 GMT", "localhost", "/", false)
	j.setCookie("d=4", "localhost", "/", false)

	cookies := map[string]int64{}
	for _, c := range j.cookies {
		cookies[c.name] = c.expires
	}
	if cookies["a"] != now.Unix()+100 || cookies["b"] != 3409338181 || cookies["d"] != 0 {
		t.Errorf("got %v", cookies)
	}
	if got := jarValues(j.cookiesFor("localhost", "/", false)); !reflect.DeepEqual(got, []string{"a=1", "b=2", "d=4"}) {
		t.Error(got)
	}

	// Deleting a cookie with Max-Age=0: like libcurl, it's kept with
	// expires=1 until the next purge.
	j.setCookie("d=4; Max-Age=0", "localhost", "/", false)
	if got := j.list(); got[len(got)-1] != "localhost\tFALSE\t/\tFALSE\t1\td\t4" {
		t.Errorf("got %q", got)
	}
	if got := jarValues(j.cookiesFor("localhost", "/", false)); !reflect.DeepEqual(got, []string{"a=1", "b=2"}) {
		t.Error(got)
	}
}

func TestCookieJarReplace(t *testing.T) {
	j := newTestJar(time.Now())
	j.setCookie("a=1", "localhost", "/", false)
	j.setCookie("b=1", "localhost", "/", false)
	j.setCookie("a=2", "localhost", "/", false)
	if got := j.list(); !reflect.DeepEqual(got, []string{"localhost\tFALSE\t/\tFALSE\t0\ta\t2", "localhost\tFALSE\t/\tFALSE\t0\tb\t1"}) {
		t.Errorf("got %q", got)
	}

	// File cookies don't replace live cookies.
	dir := t.TempDir()
	file := filepath.Join(dir, "cookies.txt")
	os.WriteFile(file, []byte("# comment\nlocalhost\tFALSE\t/\tFALSE\t0\ta\tfile\nlocalhost\tFALSE\t/\tFALSE\t0\tc\tfile\n"), 0o644)
	j.loadFile(file)
	if got := jarValues(j.cookiesFor("localhost", "/", false)); !reflect.DeepEqual(got, []string{"a=2", "b=1", "c=file"}) {
		t.Error(got)
	}

	j.addNetscape("localhost\tFALSE\t/\tFALSE\t0\tc\tset")
	if got := jarValues(j.cookiesFor("localhost", "/", false)); !reflect.DeepEqual(got, []string{"a=2", "b=1", "c=set"}) {
		t.Error(got)
	}
	j.clear()
	if len(j.list()) != 0 {
		t.Error("clear")
	}
}

func TestParseCookieDate(t *testing.T) {
	want := time.Date(2021, 1, 13, 22, 23, 1, 0, time.UTC)
	for _, s := range []string{
		"Wed, 13 Jan 2021 22:23:01 GMT",
		"Wed, 13-Jan-2021 22:23:01 GMT",
		"Wednesday, 13-Jan-21 22:23:01 GMT",
		"Wed Jan 13 22:23:01 2021",
	} {
		if d, ok := parseCookieDate(s); !ok || !d.Equal(want) {
			t.Errorf("%s: %v %v", s, d, ok)
		}
	}
	if _, ok := parseCookieDate("not a date"); ok {
		t.Error("invalid date")
	}
}

func TestPathMatch(t *testing.T) {
	tests := []struct {
		cookie, uri string
		want        bool
	}{
		{"/", "/anything", true},
		{"/a", "/a", true},
		{"/a", "/a/b", true},
		{"/a", "/ab", false},
		{"/a/b", "/a", false},
		{"/A", "/a", false},
	}
	for _, tt := range tests {
		if got := pathMatch(tt.cookie, tt.uri); got != tt.want {
			t.Errorf("pathMatch(%q, %q) = %v", tt.cookie, tt.uri, got)
		}
	}
}
