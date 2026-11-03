package httpclient

import (
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/term"
)

func newBufferedLogger(v logger.Verbosity) *logger.Logger {
	return logger.New(logger.LoggerOptions{Verbosity: v}, term.NewStderr(term.WriteModeBuffered), nil)
}

func TestRedirectMethod(t *testing.T) {
	data := []struct {
		status             uint32
		original, redirect Method
	}{
		{301, "GET", "GET"}, {301, "POST", "GET"}, {301, "DELETE", "GET"},
		{302, "GET", "GET"}, {302, "POST", "GET"}, {302, "DELETE", "GET"},
		{303, "GET", "GET"}, {303, "POST", "GET"}, {303, "DELETE", "GET"},
		{304, "GET", "GET"}, {304, "POST", "POST"}, {304, "DELETE", "DELETE"},
		{308, "GET", "GET"}, {308, "POST", "POST"}, {308, "DELETE", "DELETE"},
	}
	for _, d := range data {
		if got := redirectMethod(d.status, d.original); got != d.redirect {
			t.Errorf("%d %s: got %s", d.status, d.original, got)
		}
	}
}

func TestShouldStripCredentialsOnRedirect(t *testing.T) {
	url1 := mustUrl(t, "http://example.com")
	others := []Url{mustUrl(t, "http://example.com:8080"), mustUrl(t, "https://example.com"), mustUrl(t, "http://other.com")}
	for _, u := range others {
		if !shouldStripCredentialsOnRedirect(url1, u, FollowLocation{Follow: true, Forwarding: OnlyInitialHost}) {
			t.Errorf("%s should strip", u)
		}
		if shouldStripCredentialsOnRedirect(url1, u, FollowLocation{Follow: true, Forwarding: AllHosts}) {
			t.Errorf("%s should not strip", u)
		}
	}
	if shouldStripCredentialsOnRedirect(url1, mustUrl(t, "http://example.com:80/foo"), FollowLocation{Follow: true}) {
		t.Error("same origin")
	}
}

func TestParseCertOption(t *testing.T) {
	tests := []struct {
		in, cert, password string
		hasPassword        bool
	}{
		{"foobar", "foobar", "", false},
		{"foobar:toto", "foobar", "toto", true},
		{"foobar:toto:tata", "foobar", "toto:tata", true},
		{"foobar:", "foobar", "", true},
		{`foobar\`, `foobar\`, "", false},
		{`foo\:bar\:baz:toto:tata\:tutu`, "foo:bar:baz", `toto:tata\:tutu`, true},
		{`foo\\:toto\:tata:tutu`, `foo\`, `toto\:tata:tutu`, true},
	}
	for _, tt := range tests {
		cert, password, ok := parseCertPassword(tt.in)
		if cert != tt.cert || password != tt.password || ok != tt.hasPassword {
			t.Errorf("%q: got %q %q %v", tt.in, cert, password, ok)
		}
	}
}

func TestParseConnectToAndResolve(t *testing.T) {
	if c, ok := parseConnectTo("example.com:443:host-47.example.com:8443"); !ok || c != (connectTo{"example.com", "443", "host-47.example.com", "8443"}) {
		t.Error(c)
	}
	if c, ok := parseConnectTo("[::1]:80::8080"); !ok || c != (connectTo{"::1", "80", "", "8080"}) {
		t.Error(c)
	}
	if r, ok := parseResolve("foo.com:80:192.168.0.1,[::1]"); !ok || !reflect.DeepEqual(r, resolveEntry{"foo.com", "80", []string{"192.168.0.1", "::1"}}) {
		t.Error(r)
	}
}

func TestMatchNoProxy(t *testing.T) {
	tests := []struct {
		list, host string
		want       bool
	}{
		{"*", "anything", true},
		{"example.com", "example.com", true},
		{"example.com", "www.example.com", true},
		{".example.com", "www.example.com", true},
		{"example.com", "badexample.com", false},
		{"localhost, 127.0.0.1", "127.0.0.1", true},
		{"10.0.0.0/8", "10.1.2.3", true},
		{"10.0.0.0/8", "11.1.2.3", false},
	}
	for _, tt := range tests {
		if got := matchNoProxy(tt.list, tt.host); got != tt.want {
			t.Errorf("matchNoProxy(%q, %q) = %v", tt.list, tt.host, got)
		}
	}
}

func TestParseNetrc(t *testing.T) {
	data := "machine example.com login bob password s3cret\n" +
		"macdef init\ncd /pub\n\n" +
		"default login anon password \"a b\"\n"
	if l, p, ok := parseNetrc(data, "example.com"); !ok || l != "bob" || p != "s3cret" {
		t.Error(l, p, ok)
	}
	if l, p, ok := parseNetrc(data, "other.com"); !ok || l != "anon" || p != "a b" {
		t.Error(l, p, ok)
	}
}

// echo is a test handler writing back the request.
func echo(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "%s %s\n", r.Method, r.URL.RequestURI())
	for _, k := range []string{"Accept", "Accept-Encoding", "Authorization", "Content-Type", "Cookie", "User-Agent", "X-Foo"} {
		if v, ok := r.Header[k]; ok {
			fmt.Fprintf(w, "%s: %s\n", k, strings.Join(v, ", "))
		}
	}
	if r.Host != "" {
		fmt.Fprintf(w, "Host: %s\n", r.Host)
	}
	if len(body) > 0 {
		fmt.Fprintf(w, "\n%s", body)
	}
}

func newServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/", echo)
	mux.HandleFunc("/redirect/{code}", func(w http.ResponseWriter, r *http.Request) {
		var code int
		fmt.Sscan(r.PathValue("code"), &code)
		http.Redirect(w, r, "/echo", code)
	})
	mux.HandleFunc("/redirect-loop", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/redirect-loop", http.StatusFound)
	})
	mux.HandleFunc("/set-cookies", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Set-Cookie", "a=1; Path=/")
		w.Header().Add("Set-Cookie", "b=2; Path=/cookies; HttpOnly")
		w.Header().Add("Set-Cookie", "c=3; Max-Age=0")
	})
	mux.HandleFunc("/gzip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "text/plain")
		gz := gzip.NewWriter(w)
		gz.Write([]byte("Hello World!"))
		gz.Close()
	})
	mux.HandleFunc("/brotli", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		w.Write(brotliHelloWorld)
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/bytes/{n}", func(w http.ResponseWriter, r *http.Request) {
		var n int
		fmt.Sscan(r.PathValue("n"), &n)
		if r.URL.Query().Get("chunked") == "" {
			w.Header().Set("Content-Length", fmt.Sprint(n))
		}
		w.Write(bytes.Repeat([]byte("x"), n))
	})
	s := httptest.NewServer(mux)
	t.Cleanup(s.Close)
	return s
}

func execute(t *testing.T, c *Client, spec RequestSpec, o ClientOptions) *Call {
	t.Helper()
	call, err := c.Execute(spec, o, newBufferedLogger(0))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return call
}

func bodyLines(c *Call) []string { return strings.Split(string(c.Response.Body), "\n") }

func hasLine(lines []string, line string) bool {
	for _, l := range lines {
		if l == line {
			return true
		}
	}
	return false
}

func TestExecuteHello(t *testing.T) {
	s := newServer(t)
	c := NewClient()
	call := execute(t, c, newSpec(t, "GET", s.URL+"/hello"), DefaultClientOptions())

	r := call.Response
	if r.Status != 200 || r.Version != HttpVersionHttp11 {
		t.Errorf("status %d %v", r.Status, r.Version)
	}
	if ct, _ := r.Headers.ContentType(); ct != "text/plain" {
		t.Error(ct)
	}
	if r.IpAddr.String() != "127.0.0.1" {
		t.Error(r.IpAddr)
	}
	if r.Certificate != nil {
		t.Error("certificate on plain http")
	}
	lines := bodyLines(call)
	for _, want := range []string{"GET /hello", "Accept: */*", "User-Agent: hurl/8.1.0 go-hurl/0.1.0"} {
		if !hasLine(lines, want) {
			t.Errorf("missing %q in %q", want, lines)
		}
	}
	host := strings.TrimPrefix(s.URL, "http://")
	want := []Header{{"Host", host}, {"User-Agent", DefaultUserAgent}, {"Accept", "*/*"}}
	if got := call.Request.Headers.All(); !reflect.DeepEqual(got, want) {
		t.Errorf("request headers: %v", got)
	}
	if call.Request.Url.Raw() != s.URL+"/hello" || call.Request.Method != "GET" {
		t.Error(call.Request.Url, call.Request.Method)
	}
	tm := call.Timings
	if tm.NameLookup <= 0 || tm.Connect < tm.NameLookup || tm.PreTransfer < tm.Connect || tm.StartTransfer < tm.PreTransfer || tm.Total < tm.StartTransfer || tm.AppConnect != 0 {
		t.Errorf("timings: %+v", tm)
	}

	// The connection is reused.
	call = execute(t, c, newSpec(t, "GET", s.URL+"/hello"), DefaultClientOptions())
	if call.Timings.Connect != 0 {
		t.Errorf("connection not reused: %+v", call.Timings)
	}
}

func TestVerboseOutput(t *testing.T) {
	s := newServer(t)
	l := newBufferedLogger(logger.VerbosityVeryVerbose)
	o := DefaultClientOptions()
	o.Verbosity = logger.VerbosityVeryVerbose
	spec := newSpec(t, "POST", s.URL+"/hello")
	spec.Body = BodyText{Value: "Hello"}
	if _, err := NewClient().Execute(spec, o, l); err != nil {
		t.Fatal(err)
	}
	host := strings.TrimPrefix(s.URL, "http://")
	_, port, _ := net.SplitHostPort(host)
	out := l.Stderr.Buffer()
	for _, want := range []string{
		"**   Trying 127.0.0.1:" + port + "...\n",
		"** Connected to 127.0.0.1 (127.0.0.1) port " + port + "\n",
		"> POST /hello HTTP/1.1\n> Host: " + host + "\n> User-Agent: " + DefaultUserAgent + "\n> Content-Length: 5\n> Accept: */*\n>\n",
		"* Request body:\n* Hello\n** Request completely sent off\n",
		"* Response:\n*\n< HTTP/1.1 200 OK\n< Content-Length: ",
		"< Content-Type: text/plain\n< Date: ",
		"\n<\n* Received ",
		"* Response body:\n* POST /hello\n",
		"* Timings:\n* begin: ",
		"* namelookup: ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRequestBodies(t *testing.T) {
	s := newServer(t)
	c := NewClient()

	spec := newSpec(t, "GET", s.URL+"/query?x=1")
	spec.Querystring = []Param{NewParam("param1", "value1"), NewParam("param2", "a b")}
	if l := bodyLines(execute(t, c, spec, DefaultClientOptions())); l[0] != "GET /query?x=1&param1=value1&param2=a%20b" {
		t.Error(l[0])
	}

	spec = newSpec(t, "POST", s.URL+"/form")
	spec.Form = []Param{NewParam("param1", "value1"), NewParam("param2", "a b&c")}
	spec.ImplicitContentType = "application/x-www-form-urlencoded"
	call := execute(t, c, spec, DefaultClientOptions())
	if !strings.HasSuffix(string(call.Response.Body), "\nparam1=value1&param2=a%20b%26c") || !hasLine(bodyLines(call), "Content-Type: application/x-www-form-urlencoded") {
		t.Error(string(call.Response.Body))
	}
	if string(call.Request.Body) != "param1=value1&param2=a%20b%26c" {
		t.Error(string(call.Request.Body))
	}

	spec = newSpec(t, "POST", s.URL+"/multipart")
	spec.Multipart = []MultipartParam{
		NewParam("key1", "value1"),
		FileParam{Name: "upload1", Filename: "hello.txt", Data: []byte("Hello World!"), ContentType: "text/plain"},
	}
	spec.ImplicitContentType = "multipart/form-data"
	mux := http.NewServeMux()
	var form map[string]string
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		f, h, _ := r.FormFile("upload1")
		data, _ := io.ReadAll(f)
		form = map[string]string{
			"key1": r.FormValue("key1"), "file": string(data), "filename": h.Filename, "type": h.Header.Get("Content-Type"),
		}
	})
	ms := httptest.NewServer(mux)
	defer ms.Close()
	spec.Url = mustUrl(t, ms.URL+"/multipart")
	call = execute(t, c, spec, DefaultClientOptions())
	if !reflect.DeepEqual(form, map[string]string{"key1": "value1", "file": "Hello World!", "filename": "hello.txt", "type": "text/plain"}) {
		t.Error(form)
	}
	if ct, _ := call.Request.Headers.ContentType(); !strings.HasPrefix(ct, "multipart/form-data; boundary=------------------------") {
		t.Error(ct)
	}

	spec = newSpec(t, "PUT", s.URL+"/body")
	spec.Body = BodyBinary{Value: []byte{1, 2, 3}}
	spec.Headers = NewHeaderVec(NewHeader("X-Foo", "bar"), NewHeader("Host", "example.org"))
	call = execute(t, c, spec, DefaultClientOptions())
	if !strings.HasSuffix(string(call.Response.Body), "\n\x01\x02\x03") || !hasLine(bodyLines(call), "X-Foo: bar") || !hasLine(bodyLines(call), "Host: example.org") {
		t.Errorf("%q", call.Response.Body)
	}
	if hasLine(bodyLines(call), "Content-Type: application/x-www-form-urlencoded") {
		t.Error("unexpected content type")
	}

	call = execute(t, c, newSpec(t, "HEAD", s.URL+"/head"), DefaultClientOptions())
	if len(call.Response.Body) != 0 || call.Response.Status != 200 {
		t.Error(call.Response.Body)
	}
}

func TestHeadersOptions(t *testing.T) {
	s := newServer(t)
	user := "bob:s3cret"
	o := DefaultClientOptions()
	o.User = &user
	o.Compressed = true
	o.NoHeaders = []string{"Accept", "User-Agent"}
	o.Headers = NewHeaderVec(NewHeader("X-Foo", "from-options"))
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL+"/auth"), o)
	lines := bodyLines(call)
	for _, want := range []string{
		"Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte(user)),
		"Accept-Encoding: gzip, deflate",
		"X-Foo: from-options",
	} {
		if !hasLine(lines, want) {
			t.Errorf("missing %q in %q", want, lines)
		}
	}
	for _, l := range lines {
		if strings.HasPrefix(l, "Accept:") || strings.HasPrefix(l, "User-Agent:") {
			t.Errorf("unexpected %q", l)
		}
	}
}

func TestPathAsIs(t *testing.T) {
	// Not newServer: ServeMux redirects to clean paths.
	s := httptest.NewServer(http.HandlerFunc(echo))
	defer s.Close()
	c := NewClient()
	if l := bodyLines(execute(t, c, newSpec(t, "GET", s.URL+"/a/b/../c/./d"), DefaultClientOptions())); l[0] != "GET /a/c/d" {
		t.Error(l[0])
	}
	o := DefaultClientOptions()
	o.PathAsIs = true
	if l := bodyLines(execute(t, c, newSpec(t, "GET", s.URL+"/a/b/../c"), o)); l[0] != "GET /a/b/../c" {
		t.Error(l[0])
	}
}

func TestRedirects(t *testing.T) {
	s := newServer(t)
	c := NewClient()
	l := newBufferedLogger(0)

	o := DefaultClientOptions()
	calls, err := c.ExecuteWithRedirect(newSpec(t, "GET", s.URL+"/redirect/302"), o, l)
	if err != nil || len(calls) != 1 || calls[0].Response.Status != 302 {
		t.Fatalf("no follow: %v %v", calls, err)
	}

	o.FollowLocation = FollowLocation{Follow: true}
	spec := newSpec(t, "POST", s.URL+"/redirect/301")
	spec.Body = BodyText{Value: "data"}
	calls, err = c.ExecuteWithRedirect(spec, o, l)
	if err != nil || len(calls) != 2 {
		t.Fatalf("301: %v %v", calls, err)
	}
	if calls[1].Request.Method != "GET" || calls[1].Request.Url.Raw() != s.URL+"/echo" || len(calls[1].Request.Body) != 0 {
		t.Errorf("301 redirect: %s %s %q", calls[1].Request.Method, calls[1].Request.Url, calls[1].Request.Body)
	}

	calls, err = c.ExecuteWithRedirect(spec.withUrl(t, s.URL+"/redirect/307"), o, l)
	if err != nil || len(calls) != 2 || calls[1].Request.Method != "POST" || string(calls[1].Request.Body) != "data" {
		t.Fatalf("307: %v %v", calls, err)
	}

	o.MaxRedirect = ast.CountFinite{Value: 3}
	_, err = c.ExecuteWithRedirect(newSpec(t, "GET", s.URL+"/redirect-loop"), o, l)
	if err != (HttpErrorTooManyRedirect{}) {
		t.Errorf("loop: %v", err)
	}
}

func (r RequestSpec) withUrl(t *testing.T, u string) RequestSpec {
	r.Url = mustUrl(t, u)
	return r
}

func TestRedirectCredentials(t *testing.T) {
	target := newServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/echo", http.StatusFound)
	})
	origin := httptest.NewServer(mux)
	defer origin.Close()

	spec := newSpec(t, "GET", origin.URL+"/start")
	spec.Headers = NewHeaderVec(NewHeader("Authorization", "Bearer token"))
	spec.Cookies = []RequestCookie{{"session", "abc"}}
	o := DefaultClientOptions()
	o.FollowLocation = FollowLocation{Follow: true}

	calls, err := NewClient().ExecuteWithRedirect(spec, o, newBufferedLogger(0))
	if err != nil || len(calls) != 2 {
		t.Fatal(calls, err)
	}
	lines := strings.Join(bodyLines(&calls[1]), "\n")
	if strings.Contains(lines, "Authorization") || strings.Contains(lines, "Cookie") {
		t.Errorf("credentials leaked: %s", lines)
	}

	o.FollowLocation.Forwarding = AllHosts
	calls, err = NewClient().ExecuteWithRedirect(spec, o, newBufferedLogger(0))
	if err != nil {
		t.Fatal(err)
	}
	l := bodyLines(&calls[1])
	if !hasLine(l, "Authorization: Bearer token") || !hasLine(l, "Cookie: session=abc") {
		t.Errorf("credentials not forwarded: %q", l)
	}
}

func TestCookies(t *testing.T) {
	s := newServer(t)
	c := NewClient()
	l := newBufferedLogger(0)
	execute(t, c, newSpec(t, "GET", s.URL+"/set-cookies"), DefaultClientOptions())

	store := c.CookieStore(l)
	want := "127.0.0.1\tFALSE\t/\tFALSE\t0\ta\t1\n" +
		"#HttpOnly_127.0.0.1\tFALSE\t/cookies\tFALSE\t0\tb\t2\n" +
		"127.0.0.1\tFALSE\t/\tFALSE\t1\tc\t3\n"
	if got := store.ToNetscape(); got != want {
		t.Errorf("store:\n%s", got)
	}

	spec := newSpec(t, "GET", s.URL+"/cookies/x")
	spec.Cookies = []RequestCookie{{"d", "4"}}
	if l := bodyLines(execute(t, c, spec, DefaultClientOptions())); !hasLine(l, "Cookie: b=2; a=1; d=4") {
		t.Errorf("%q", l)
	}
	if got := c.CookieStore(l).Len(); got != 2 {
		t.Errorf("expired cookie not purged: %d", got)
	}

	o := DefaultClientOptions()
	o.UseCookieStore = false
	if l := bodyLines(execute(t, c, newSpec(t, "GET", s.URL+"/cookies/x"), o)); strings.Contains(strings.Join(l, "\n"), "Cookie") {
		t.Errorf("%q", l)
	}

	cookie, _ := CookieFromNetscape("127.0.0.1\tFALSE\t/\tFALSE\t0\te\t5")
	c.AddCookie(cookie, l)
	if l := bodyLines(execute(t, c, newSpec(t, "GET", s.URL+"/"), DefaultClientOptions())); !hasLine(l, "Cookie: e=5; a=1") {
		t.Errorf("%q", l)
	}
	c.ClearCookieStorage(l)
	if c.CookieStore(l).Len() != 0 {
		t.Error("clear")
	}

	file := filepath.Join(t.TempDir(), "cookies.txt")
	os.WriteFile(file, []byte("127.0.0.1\tFALSE\t/\tFALSE\t0\tfromfile\tyes\n"), 0o644)
	o = DefaultClientOptions()
	o.CookieInputFile = file
	if l := bodyLines(execute(t, NewClient(), newSpec(t, "GET", s.URL+"/"), o)); !hasLine(l, "Cookie: fromfile=yes") {
		t.Errorf("%q", l)
	}
}

func TestCompression(t *testing.T) {
	s := newServer(t)
	o := DefaultClientOptions()
	o.Compressed = true
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL+"/gzip"), o)
	if bytes.Equal(call.Response.Body, []byte("Hello World!")) {
		t.Error("body should be kept compressed")
	}
	if text, err := call.Response.Text(); err != nil || text != "Hello World!" {
		t.Error(text, err)
	}

	call = execute(t, NewClient(), newSpec(t, "GET", s.URL+"/brotli"), o)
	if _, err := call.Response.Text(); err != (HttpErrorCouldNotUncompressResponse{Detail: "brotli"}) {
		t.Error(err)
	}
}

func TestTimeout(t *testing.T) {
	s := newServer(t)
	o := DefaultClientOptions()
	o.Timeout = 100 * time.Millisecond
	_, err := NewClient().Execute(newSpec(t, "GET", s.URL+"/slow"), o, newBufferedLogger(0))
	e, ok := err.(HttpErrorLibcurl)
	if !ok || e.Code != 28 || !strings.HasPrefix(e.Detail, "Operation timed out after ") || !strings.HasSuffix(e.Detail, " milliseconds with 0 bytes received") {
		t.Errorf("got %v", err)
	}
	if err.Description() != "HTTP connection" {
		t.Error(err.Description())
	}
}

func TestConnectionErrors(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	_, err := NewClient().Execute(newSpec(t, "GET", "http://"+addr+"/"), DefaultClientOptions(), newBufferedLogger(0))
	e, ok := err.(HttpErrorLibcurl)
	_, port, _ := net.SplitHostPort(addr)
	if !ok || e.Code != 7 || !strings.HasPrefix(e.Detail, "Failed to connect to 127.0.0.1 port "+port+" after ") || !strings.HasSuffix(e.Detail, "Couldn't connect to server") {
		t.Errorf("got %v", err)
	}

	o := DefaultClientOptions()
	o.Resolves = []string{"unknown.invalid:80:127.0.0.1"}
	o.IpResolve = IpResolveIpV6
	_, err = NewClient().Execute(newSpec(t, "GET", "http://unknown.invalid/"), o, newBufferedLogger(0))
	if e, ok := err.(HttpErrorLibcurl); !ok || e.Code != 6 || e.Detail != "Could not resolve host: unknown.invalid" {
		t.Errorf("got %v", err)
	}

	_, err = NewClient().Execute(newSpec(t, "GET", "http://"+addr+"/x'; touch foo; : '"), DefaultClientOptions(), newBufferedLogger(0))
	if e, ok := err.(HttpErrorLibcurl); !ok || e.Code != 3 {
		t.Errorf("got %v", err)
	}
}

func TestLimitRate(t *testing.T) {
	s := newServer(t)
	o := DefaultClientOptions()
	o.MaxRecvSpeed = 4000
	start := time.Now()
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL+"/bytes/2000"), o)
	if len(call.Response.Body) != 2000 {
		t.Fatal(len(call.Response.Body))
	}
	if d := time.Since(start); d < 400*time.Millisecond {
		t.Errorf("transfer too fast: %v", d)
	}
}

func TestMaxFilesize(t *testing.T) {
	s := newServer(t)
	max := uint64(100)
	o := DefaultClientOptions()
	o.MaxFilesize = &max
	execute(t, NewClient(), newSpec(t, "GET", s.URL+"/bytes/100"), o)
	for _, path := range []string{"/bytes/101", "/bytes/5000?chunked=1"} {
		_, err := NewClient().Execute(newSpec(t, "GET", s.URL+path), o, newBufferedLogger(0))
		if err != (HttpErrorLibcurl{Code: 63, Detail: "Maximum file size exceeded"}) {
			t.Errorf("%s: got %v", path, err)
		}
	}
}

func TestResolveAndConnectTo(t *testing.T) {
	s := newServer(t)
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(s.URL, "http://"))

	o := DefaultClientOptions()
	o.Resolves = []string{"foo.test:" + port + ":127.0.0.1"}
	call := execute(t, NewClient(), newSpec(t, "GET", "http://foo.test:"+port+"/resolve"), o)
	if !hasLine(bodyLines(call), "Host: foo.test:"+port) || call.Response.IpAddr.String() != "127.0.0.1" {
		t.Errorf("%q", bodyLines(call))
	}

	o = DefaultClientOptions()
	o.ConnectsTo = []string{"bar.test:80:127.0.0.1:" + port}
	call = execute(t, NewClient(), newSpec(t, "GET", "http://bar.test/connect-to"), o)
	if !hasLine(bodyLines(call), "Host: bar.test") {
		t.Errorf("%q", bodyLines(call))
	}
}

func TestUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "hurl")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "s.sock")
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Skip(err)
	}
	srv := &http.Server{Handler: http.HandlerFunc(echo)}
	go srv.Serve(ln)
	defer srv.Close()

	o := DefaultClientOptions()
	o.UnixSocket = path
	call := execute(t, NewClient(), newSpec(t, "GET", "http://localhost/unix"), o)
	if l := bodyLines(call); l[0] != "GET /unix" || !hasLine(l, "Host: localhost") {
		t.Errorf("%q", l)
	}
	if call.Response.IpAddr.String() != "" {
		t.Error(call.Response.IpAddr)
	}
}

func TestNetrc(t *testing.T) {
	s := newServer(t)
	file := filepath.Join(t.TempDir(), "netrc")
	os.WriteFile(file, []byte("machine 127.0.0.1 login bob password secret\n"), 0o600)
	o := DefaultClientOptions()
	o.NetrcFile = file
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL+"/netrc"), o)
	if !hasLine(bodyLines(call), "Authorization: Basic "+base64.StdEncoding.EncodeToString([]byte("bob:secret"))) {
		t.Errorf("%q", bodyLines(call))
	}
	o.NetrcFile = filepath.Join(t.TempDir(), "missing")
	if _, err := NewClient().Execute(newSpec(t, "GET", s.URL+"/netrc"), o, newBufferedLogger(0)); err == nil {
		t.Error("missing required netrc file should fail")
	}
}

func TestProxy(t *testing.T) {
	var gotUrl, gotHeader string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUrl, gotHeader = r.URL.String(), r.Header.Get("X-Proxy")
		fmt.Fprint(w, "proxied")
	}))
	defer proxy.Close()

	o := DefaultClientOptions()
	o.Proxy = strings.TrimPrefix(proxy.URL, "http://")
	o.ProxyHeaders = NewHeaderVec(NewHeader("X-Proxy", "yes"))
	l := newBufferedLogger(logger.VerbosityVerbose)
	o.Verbosity = logger.VerbosityVerbose
	call, err := NewClient().Execute(newSpec(t, "GET", "http://example.test/proxy"), o, l)
	if err != nil {
		t.Fatal(err)
	}
	if string(call.Response.Body) != "proxied" || gotUrl != "http://example.test/proxy" || gotHeader != "yes" {
		t.Errorf("%q %q %q", call.Response.Body, gotUrl, gotHeader)
	}
	if !strings.Contains(l.Stderr.Buffer(), "> GET http://example.test/proxy HTTP/1.1\n") {
		t.Error(l.Stderr.Buffer())
	}

	noProxy := "example.test"
	o.NoProxy = &noProxy
	o.Resolves = []string{"example.test:80:127.0.0.1"}
	o.ConnectsTo = []string{"example.test:80:" + strings.TrimPrefix(newServer(t).URL, "http://")}
	call = execute(t, NewClient(), newSpec(t, "GET", "http://example.test/direct"), o)
	if bodyLines(call)[0] != "GET /direct" {
		t.Error(string(call.Response.Body))
	}
}

func TestProxyTunnel(t *testing.T) {
	target := newTLSServer(t, false, nil)
	var connectLine, connectHeader, connectServerHeader string
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectLine = r.Method + " " + r.Host
		connectHeader, connectServerHeader = r.Header.Get("X-To-Proxy"), r.Header.Get("X-To-Server")
		upstream, err := net.Dial("tcp", r.Host)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		conn, buf, _ := w.(http.Hijacker).Hijack()
		buf.WriteString("HTTP/1.1 200 Connection established\r\n\r\n")
		buf.Flush()
		go io.Copy(upstream, conn)
		io.Copy(conn, upstream)
		conn.Close()
	}))
	defer proxy.Close()

	o := DefaultClientOptions()
	o.Proxy = proxy.URL
	o.CacertFile = writeServerCert(t, target)
	o.ProxyHeaders = NewHeaderVec(NewHeader("X-To-Proxy", "to-proxy"))
	o.Verbosity = logger.VerbosityVerbose
	spec := newSpec(t, "GET", target.URL+"/hello")
	spec.Headers = NewHeaderVec(NewHeader("X-To-Server", "to-server"))
	l := newBufferedLogger(logger.VerbosityVerbose)
	call, err := NewClient().Execute(spec, o, l)
	if err != nil {
		t.Fatal(err)
	}
	if string(call.Response.Body) != "HTTP/1.1 0" {
		t.Errorf("body %q", call.Response.Body)
	}
	host := strings.TrimPrefix(target.URL, "https://")
	if connectLine != "CONNECT "+host || connectHeader != "to-proxy" || connectServerHeader != "" {
		t.Errorf("proxy got %q, X-To-Proxy %q, X-To-Server %q", connectLine, connectHeader, connectServerHeader)
	}
	log := l.Stderr.Buffer()
	want := "> CONNECT " + host + " HTTP/1.1\n> Host: " + host + "\n> Proxy-Connection: Keep-Alive\n> X-To-Proxy: to-proxy\n>\n< HTTP/1.1 200 Connection established\n<\n> GET /hello HTTP/1.1\n"
	if !strings.Contains(log, want) {
		t.Errorf("log:\n%s\nwant:\n%s", log, want)
	}

	t.Setenv("no_proxy", "127.0.0.1")
	connectLine = ""
	if _, err := NewClient().Execute(spec, o, newBufferedLogger(0)); err != nil {
		t.Fatal(err)
	}
	if connectLine != "" {
		t.Errorf("no_proxy ignored with explicit --proxy: proxy got %q", connectLine)
	}
}

func TestUnsupportedFeatures(t *testing.T) {
	tests := []struct {
		set  func(*ClientOptions)
		want string
	}{
		{func(o *ClientOptions) { o.HttpVersion = RequestedHttpVersionHttp10 }, "HTTP/1.0 is not supported by go-hurl"},
		{func(o *ClientOptions) { o.HttpVersion = RequestedHttpVersionHttp3 }, "HTTP/3 is not supported by go-hurl"},
		{func(o *ClientOptions) { o.Ntlm = true }, "NTLM authentication (--ntlm) is not supported by go-hurl"},
		{func(o *ClientOptions) { o.Negotiate = true }, "Negotiate authentication (--negotiate) is not supported by go-hurl"},
		{func(o *ClientOptions) { o.Digest = true }, "Digest authentication (--digest) is not supported by go-hurl"},
		{func(o *ClientOptions) { o.AwsSigv4 = "aws:amz" }, "AWS SigV4 (--aws-sigv4) is not supported by go-hurl"},
		{func(o *ClientOptions) { o.ClientCertFile = "cert.pem:password" }, "Client certificate password (--cert <file>:<password>) is not supported by go-hurl"},
	}
	for _, tt := range tests {
		o := DefaultClientOptions()
		tt.set(&o)
		_, err := NewClient().Execute(newSpec(t, "GET", "http://localhost/"), o, newBufferedLogger(0))
		if err == nil || err.Message() != tt.want {
			t.Errorf("got %v, want %q", err, tt.want)
		}
	}

	// --ssl-no-revoke is accepted and ignored.
	s := newServer(t)
	o := DefaultClientOptions()
	o.SslNoRevoke = true
	execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
}

func newTLSServer(t *testing.T, http2 bool, config func(*tls.Config)) *httptest.Server {
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s %d", r.Proto, len(r.TLS.PeerCertificates))
	}))
	s.EnableHTTP2 = http2
	s.Config.ErrorLog = log.New(io.Discard, "", 0)
	if config != nil {
		s.TLS = &tls.Config{}
		config(s.TLS)
	}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

func writeServerCert(t *testing.T, s *httptest.Server) string {
	file := filepath.Join(t.TempDir(), "cacert.pem")
	os.WriteFile(file, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: s.Certificate().Raw}), 0o644)
	return file
}

func TestTLS(t *testing.T) {
	s := newTLSServer(t, false, nil)

	_, err := NewClient().Execute(newSpec(t, "GET", s.URL), DefaultClientOptions(), newBufferedLogger(0))
	if e, ok := err.(HttpErrorLibcurl); !ok || e.Code != 60 {
		t.Errorf("got %v", err)
	}

	o := DefaultClientOptions()
	o.Insecure = true
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
	if string(call.Response.Body) != "HTTP/1.1 0" {
		t.Error(string(call.Response.Body))
	}
	cert := call.Response.Certificate
	if cert == nil || *cert.Subject() != "O = Acme Co" || cert.ExpireDate() == nil || cert.SerialNumber() == nil || cert.Value() == nil {
		t.Fatalf("certificate: %+v", cert)
	}
	if !strings.HasPrefix(*cert.Value(), "-----BEGIN CERTIFICATE-----\n") || !strings.HasSuffix(*cert.Value(), "-----END CERTIFICATE-----") || !strings.Contains(*cert.SubjectAltName(), "IP Address:127.0.0.1") {
		t.Error(*cert.Value(), *cert.SubjectAltName())
	}
	if call.Timings.AppConnect < call.Timings.Connect {
		t.Errorf("timings: %+v", call.Timings)
	}

	o = DefaultClientOptions()
	o.CacertFile = writeServerCert(t, s)
	execute(t, NewClient(), newSpec(t, "GET", s.URL), o)

	sum := sha256.Sum256(s.Certificate().RawSubjectPublicKeyInfo)
	o.PinnedPubKey = "sha256//" + base64.StdEncoding.EncodeToString(sum[:])
	execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
	o.PinnedPubKey = "sha256//AAAA"
	_, err = NewClient().Execute(newSpec(t, "GET", s.URL), o, newBufferedLogger(0))
	if err != (HttpErrorLibcurl{Code: 90, Detail: "SSL: public key does not match pinned public key"}) {
		t.Errorf("got %v", err)
	}

	o = DefaultClientOptions()
	o.Insecure = true
	o.TlsMax = TlsVersionTls12
	execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
}

func TestHTTP2(t *testing.T) {
	s := newTLSServer(t, true, nil)
	o := DefaultClientOptions()
	o.Insecure = true
	l := newBufferedLogger(logger.VerbosityVeryVerbose)
	o.Verbosity = logger.VerbosityVeryVerbose
	call, err := NewClient().Execute(newSpec(t, "GET", s.URL+"/h2"), o, l)
	if err != nil {
		t.Fatal(err)
	}
	if call.Response.Version != HttpVersionHttp2 || string(call.Response.Body) != "HTTP/2.0 0" {
		t.Errorf("%v %s", call.Response.Version, call.Response.Body)
	}
	out := l.Stderr.Buffer()
	for _, want := range []string{"** ALPN: server accepted h2\n", "> GET /h2 HTTP/2\n> Host: ", "> User-Agent: " + DefaultUserAgent + "\n", "< HTTP/2 200\n< content-length: 10\n< content-type: "} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if h, _ := call.Response.Headers.Get("Content-Type"); h.Name != "content-type" {
		t.Error(h)
	}

	o.HttpVersion = RequestedHttpVersionHttp11
	call = execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
	if call.Response.Version != HttpVersionHttp11 {
		t.Error(call.Response.Version)
	}
}

func TestH2CPriorKnowledge(t *testing.T) {
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.Proto)
	}))
	s.Config.Protocols = &http.Protocols{}
	s.Config.Protocols.SetHTTP1(true)
	s.Config.Protocols.SetUnencryptedHTTP2(true)
	s.Start()
	defer s.Close()

	o := DefaultClientOptions()
	o.HttpVersion = RequestedHttpVersionHttp2PriorKnowledge
	call := execute(t, NewClient(), newSpec(t, "GET", s.URL), o)
	if call.Response.Version != HttpVersionHttp2 || string(call.Response.Body) != "HTTP/2.0" {
		t.Errorf("%v %s", call.Response.Version, call.Response.Body)
	}
	call = execute(t, NewClient(), newSpec(t, "GET", s.URL), DefaultClientOptions())
	if call.Response.Version != HttpVersionHttp11 {
		t.Error(call.Response.Version)
	}
}

func TestClientCertificate(t *testing.T) {
	s := newTLSServer(t, false, func(c *tls.Config) { c.ClientAuth = tls.RequireAnyClientCert })
	dir := t.TempDir()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "client"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	keyDer, _ := x509.MarshalECPrivateKey(key)
	certFile, keyFile := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDer}), 0o644)

	o := DefaultClientOptions()
	o.Insecure = true
	o.ClientCertFile, o.ClientKeyFile = certFile, keyFile
	if call := execute(t, NewClient(), newSpec(t, "GET", s.URL), o); string(call.Response.Body) != "HTTP/1.1 1" {
		t.Error(string(call.Response.Body))
	}

	encrypted := filepath.Join(dir, "encrypted.pem")
	os.WriteFile(encrypted, pem.EncodeToMemory(&pem.Block{Type: "ENCRYPTED PRIVATE KEY", Bytes: []byte{0}}), 0o644)
	o.ClientKeyFile = encrypted
	_, err := NewClient().Execute(newSpec(t, "GET", s.URL), o, newBufferedLogger(0))
	if err != (HttpErrorUnsupportedFeature{Feature: "Encrypted client key"}) {
		t.Errorf("got %v", err)
	}
}

func TestAllowReuse(t *testing.T) {
	s := newServer(t)
	c := NewClient()
	o := DefaultClientOptions()
	execute(t, c, newSpec(t, "GET", s.URL), o)
	o.AllowReuse = false
	l := newBufferedLogger(logger.VerbosityVerbose)
	call, err := c.Execute(newSpec(t, "GET", s.URL), o, l)
	if err != nil {
		t.Fatal(err)
	}
	if call.Timings.Connect == 0 {
		t.Error("expected a new connection")
	}
	if !strings.Contains(l.Stderr.Buffer(), "* Force refreshing connections because requested HTTP version change\n") {
		t.Error(l.Stderr.Buffer())
	}
}

func TestStatusLineLatin1(t *testing.T) {
	resp := &http.Response{Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Status: "200 Succ\xe8s"}
	if got := statusLine(resp); got != "HTTP/1.1 200 Succès" {
		t.Errorf("got %q", got)
	}
}
