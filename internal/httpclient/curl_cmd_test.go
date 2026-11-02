package httpclient

import (
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func newSpec(t *testing.T, method, u string) RequestSpec {
	r := NewRequestSpec()
	r.Method = Method(method)
	r.Url = mustUrl(t, u)
	return r
}

func checkCmd(t *testing.T, cmd CurlCmd, secrets []string, want string) {
	t.Helper()
	if got := cmd.ToRedactedString(secrets); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestHelloRequestWithDefaultOptions(t *testing.T) {
	request := newSpec(t, "GET", "http://localhost:8000/hello")
	contextDir := util.ContextDir{}
	cookieStore := NewCookieStore()
	options := DefaultClientOptions()

	cmd := NewCurlCmd(request, cookieStore, contextDir, nil, options)
	checkCmd(t, cmd, nil, "curl 'http://localhost:8000/hello'")

	// Same requests with some output:
	output := util.NewOutput("foo.out")
	cmd = NewCurlCmd(request, cookieStore, contextDir, output, options)
	checkCmd(t, cmd, nil, "curl --output foo.out 'http://localhost:8000/hello'")

	// With some headers
	request.Headers = NewHeaderVec(NewHeader("User-Agent", "iPhone"), NewHeader("Foo", "Bar"))
	cmd = NewCurlCmd(request, cookieStore, contextDir, output, options)
	checkCmd(t, cmd, nil, "curl "+
		"--header 'User-Agent: iPhone' "+
		"--header 'Foo: Bar' "+
		"--output foo.out "+
		"'http://localhost:8000/hello'")

	// With some cookies:
	cookieStore = NewCookieStore()
	cookieStore.AddCookie("localhost  TRUE    /   FALSE   0   cookie1 valueA")
	cookieStore.AddCookie("localhost  FALSE    /   FALSE   1   cookie2 valueB")
	cmd = NewCurlCmd(request, cookieStore, contextDir, output, options)
	checkCmd(t, cmd, nil, "curl "+
		"--header 'User-Agent: iPhone' "+
		"--header 'Foo: Bar' "+
		"--cookie 'cookie1=valueA' "+
		"--output foo.out "+
		"'http://localhost:8000/hello'")
}

func TestHelloRequestWithOptions(t *testing.T) {
	request := newSpec(t, "GET", "http://localhost:8000/hello")
	headers := NewHeaderVec(
		NewHeader("Test-Header-1", "content-1"),
		NewHeader("Test-Header-2", "content-2"),
		NewHeader("Test-Header-Empty", ""),
	)
	user, userAgent := "user:password", "my-useragent"
	options := ClientOptions{
		AllowReuse:      true,
		Compressed:      true,
		ConnectTimeout:  20 * time.Second,
		ConnectsTo:      []string{"example.com:443:host-47.example.com:443"},
		CookieInputFile: "cookie_file",
		FollowLocation:  FollowLocation{Follow: true, Forwarding: OnlyInitialHost},
		Headers:         headers,
		HttpVersion:     RequestedHttpVersionHttp10,
		Insecure:        true,
		IpResolve:       IpResolveIpV6,
		MaxRecvSpeed:    8000,
		MaxRedirect:     ast.CountFinite{Value: 10},
		MaxSendSpeed:    8000,
		Negotiate:       true,
		NetrcFile:       "/var/run/netrc",
		NetrcOptional:   true,
		Ntlm:            true,
		PathAsIs:        true,
		Proxy:           "localhost:3128",
		ProxyHeaders:    NewHeaderVec(NewHeader("X-Foo", "foo")),
		Resolves:        []string{"foo.com:80:192.168.0.1", "bar.com:443:127.0.0.1"},
		Timeout:         10 * time.Second,
		TlsMax:          TlsVersionTls12,
		TlsMin:          TlsVersionTls10,
		UnixSocket:      "/var/run/example.sock",
		UseCookieStore:  true,
		User:            &user,
		UserAgent:       &userAgent,
	}

	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, options)
	checkCmd(t, cmd, nil, "curl "+
		"--header 'Test-Header-1: content-1' "+
		"--header 'Test-Header-2: content-2' "+
		"--header 'Test-Header-Empty;' "+
		"--compressed "+
		"--connect-timeout 20 "+
		"--connect-to example.com:443:host-47.example.com:443 "+
		"--cookie cookie_file "+
		"--http1.0 "+
		"--insecure "+
		"--ipv6 "+
		"--location "+
		"--limit-rate 8000 "+
		"--max-redirs 10 "+
		"--max-time 10 "+
		"--tlsv1.0 "+
		"--tls-max 1.2 "+
		"--negotiate "+
		"--netrc-file '/var/run/netrc' "+
		"--netrc-optional "+
		"--ntlm "+
		"--path-as-is "+
		"--proxy 'localhost:3128' "+
		"--proxy-header 'X-Foo: foo' "+
		"--resolve foo.com:80:192.168.0.1 "+
		"--resolve bar.com:443:127.0.0.1 "+
		"--unix-socket '/var/run/example.sock' "+
		"--user 'user:password' "+
		"--user-agent 'my-useragent' "+
		"'http://localhost:8000/hello'")
}

func TestUrlWithDot(t *testing.T) {
	request := newSpec(t, "GET", "https://example.org/hello/../to/../your/../file")
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl 'https://example.org/hello/../to/../your/../file'")
}

func TestUrlWithCurlGlob(t *testing.T) {
	request := newSpec(t, "GET", "http://foo.com?param1=value1&param2={bar}")
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl --globoff 'http://foo.com?param1=value1&param2={bar}'")
}

func TestUrlWithSingleQuote(t *testing.T) {
	request := newSpec(t, "GET", "http://127.0.0.1:8000/x'; touch POC_A_MARKER; : '")
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, `curl $'http://127.0.0.1:8000/x\'; touch POC_A_MARKER; : \''`)
}

func TestQueryRequest(t *testing.T) {
	request := newSpec(t, "GET", "http://localhost:8000/querystring-params")
	request.Querystring = []Param{NewParam("param1", "value1"), NewParam("param2", "a b")}
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl 'http://localhost:8000/querystring-params?param1=value1&param2=a%20b'")

	// Add some query param in the URL
	request.Url = mustUrl(t, "http://localhost:8000/querystring-params?param3=foo&param4=bar")
	cmd = NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl 'http://localhost:8000/querystring-params?param3=foo&param4=bar&param1=value1&param2=a%20b'")
}

func TestFormRequest(t *testing.T) {
	request := newSpec(t, "POST", "http://localhost/form-params")
	request.Headers = NewHeaderVec(NewHeader("Content-Type", "application/x-www-form-urlencoded"))
	request.Form = []Param{NewParam("param1", "value1"), NewParam("param2", "a b")}
	request.ImplicitContentType = "multipart/form-data"
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--header 'Content-Type: application/x-www-form-urlencoded' "+
		"--data 'param1=value1' "+
		"--data 'param2=a%20b' "+
		"'http://localhost/form-params'")
}

func TestJsonRequest(t *testing.T) {
	request := newSpec(t, "POST", "http://localhost/json")
	request.Headers = NewHeaderVec(NewHeader("content-type", "application/vnd.api+json"))
	request.Body = BodyText{Value: ""}
	request.ImplicitContentType = "application/json"
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--request POST "+
		"--header 'content-type: application/vnd.api+json' "+
		"'http://localhost/json'")

	// Add a non-empty body
	request.Body = BodyText{Value: `{"foo":"bar"}`}
	cmd = NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--header 'content-type: application/vnd.api+json' "+
		`--data '{"foo":"bar"}' `+
		"'http://localhost/json'")

	// Change method
	request.Method = "PUT"
	cmd = NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--request PUT "+
		"--header 'content-type: application/vnd.api+json' "+
		`--data '{"foo":"bar"}' `+
		"'http://localhost/json'")
}

func TestPostBinaryFile(t *testing.T) {
	request := newSpec(t, "POST", "http://localhost:8000/hello")
	request.Body = BodyFile{Value: []byte("Hello World!"), Filename: "foo.bin"}
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--header 'Content-Type:' "+
		"--data-binary '@foo.bin' "+
		"'http://localhost:8000/hello'")
}

func TestPostBinaryBodyWithNullBytes(t *testing.T) {
	// https://github.com/Orange-OpenSource/hurl/issues/5141
	// A body containing a NUL byte can not be embedded in a shell string:
	// the curl command must read the body from stdin instead.
	request := newSpec(t, "POST", "http://localhost:8000/post-bytes-null")
	request.Body = BodyBinary{Value: []byte{0, 1, 2, 3}}
	request.Headers = NewHeaderVec(NewHeader("Content-Type", "application/octet-stream"))
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, `printf '\x00\x01\x02\x03' | `+
		"curl "+
		"--header 'Content-Type: application/octet-stream' "+
		"--data-binary @- "+
		"'http://localhost:8000/post-bytes-null'")
}

func TestPostBinaryBodyWithoutNullBytes(t *testing.T) {
	// Control: without a NUL byte, the rendering is unchanged.
	request := newSpec(t, "POST", "http://localhost:8000/post-bytes")
	request.Body = BodyBinary{Value: []byte{1, 2, 3}}
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, "curl "+
		"--header 'Content-Type: application/octet-stream' "+
		`--data $'\x01\x02\x03' `+
		"'http://localhost:8000/post-bytes'")
}

func TestEncodeByte(t *testing.T) {
	if encodeByte(1) != `\x01` || encodeByte(32) != `\x20` {
		t.Error(encodeByte(1), encodeByte(32))
	}
}

func TestHeaderCurlArgs(t *testing.T) {
	if got := headerCurlArgs(NewHeader("Host", "example.com"), "--header"); !reflect.DeepEqual(got, []arg{argPlain("--header"), argQuoted("Host: example.com")}) {
		t.Error(got)
	}
	if got := headerCurlArgs(NewHeader("If-Match", `"e0023aa4e"`), "--proxy-header"); !reflect.DeepEqual(got, []arg{argPlain("--proxy-header"), argQuoted(`If-Match: "e0023aa4e"`)}) {
		t.Error(got)
	}
}

func TestParamCurlArgs(t *testing.T) {
	if s := paramCurlArg(NewParam("param1", "value1")); s != "param1=value1" {
		t.Error(s)
	}
	if s := paramCurlArg(NewParam("param2", "")); s != "param2=" {
		t.Error(s)
	}
	if s := paramCurlArgEscape(NewParam("param3", "a=b")); s != "param3=a%3Db" {
		t.Error(s)
	}
	if s := paramCurlArgEscape(NewParam("param4", "1,2,3")); s != "param4=1%2C2%2C3" {
		t.Error(s)
	}
}

func TestEncodeBody(t *testing.T) {
	contextDir := util.NewContextDir("/tmp", "/tmp")
	if got := bodyCurlArg(BodyText{Value: "hello"}, contextDir); got != argQuoted("hello") {
		t.Error(got)
	}
	if runtime.GOOS != "windows" {
		if got := bodyCurlArg(BodyFile{Filename: "filename"}, contextDir); got != argQuoted("@/tmp/filename") {
			t.Error(got)
		}
	}
	if got := bodyCurlArg(BodyBinary{Value: []byte{1, 2, 3}}, contextDir); !reflect.DeepEqual(got, argBytes{1, 2, 3}) {
		t.Error(got)
	}
}

func TestEncodeArg(t *testing.T) {
	tests := []struct {
		a    arg
		want string
	}{
		{argQuoted("hello"), "'hello'"},
		{argQuoted(""), "''"},
		{argQuoted(`\n`), `'\n'`},
		{argQuoted("'"), `$'\''`},
		{argQuoted(`\'`), `$'\\\''`},
		{argQuoted("\n"), `$'\n'`},
		{argQuoted("\t"), `$'\t'`},
		{argPlain("tests_ssl/certs/client/cert.pem:foobar"), "tests_ssl/certs/client/cert.pem:foobar"},
		{argPlain("foo.com:80:localhost:8000"), "foo.com:80:localhost:8000"},
		{argPlain("-"), "-"},
		{argPlain(""), "''"},
		{argPlain("my file"), "'my file'"},
		{argPlain("~/foo"), "'~/foo'"},
		{argPlain("foo.out; touch POC_A_MARKER"), "'foo.out; touch POC_A_MARKER'"},
		{argPlain("$(touch x)"), "'$(touch x)'"},
		{argPlain("a'b"), `$'a\'b'`},
		{argBytes{1, 2, 3}, `$'\x01\x02\x03'`},
	}
	for _, tt := range tests {
		if got := tt.a.encode(nil); got != tt.want {
			t.Errorf("encode(%#v) = %s, want %s", tt.a, got, tt.want)
		}
	}
}

func TestEncodeArgWithSecrets(t *testing.T) {
	secrets := []string{"s3cr'et"}
	if got := argQuoted("Token: s3cr'et").encode(secrets); got != "'Token: ***'" {
		t.Error(got)
	}
	if got := argPlain("foo-s3cr'et").encode(secrets); got != "'foo-***'" {
		t.Error(got)
	}
	if got := argPlain("--header").encode(secrets); got != "--header" {
		t.Error(got)
	}
}

func TestRedactedCurlCmd(t *testing.T) {
	// A secret containing a quote is encoded in the command line, so it must
	// be redacted before being encoded.
	request := newSpec(t, "GET", "http://localhost:8000/hello?token=s3cr'et")
	request.Headers = NewHeaderVec(NewHeader("Authorization", "Bearer s3cr'et"))
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, `curl `+
		`--header $'Authorization: Bearer s3cr\'et' `+
		`$'http://localhost:8000/hello?token=s3cr\'et'`)
	checkCmd(t, cmd, []string{"s3cr'et"}, "curl "+
		"--header 'Authorization: Bearer ***' "+
		"'http://localhost:8000/hello?token=***'")
}

func TestCookieWithSingleQuote(t *testing.T) {
	request := newSpec(t, "GET", "http://localhost:8000/hello")
	request.Cookies = []RequestCookie{{Name: "cookie1", Value: "value'with'quotes"}}
	cmd := NewCurlCmd(request, NewCookieStore(), util.ContextDir{}, nil, DefaultClientOptions())
	checkCmd(t, cmd, nil, `curl --cookie $'cookie1=value\'with\'quotes' 'http://localhost:8000/hello'`)
}

// Port of client.rs command_line_args.
func TestCommandLineArgs(t *testing.T) {
	client := NewClient()
	request := newSpec(t, "GET", "https://example.org")
	user, userAgent := "user:password", "my-useragent"
	options := DefaultClientOptions()
	options.AwsSigv4 = "aws:amz:sts"
	options.CacertFile = "/etc/cert.pem"
	options.Compressed = true
	options.ConnectsTo = []string{"example.com:443:host-47.example.com:443"}
	options.Insecure = true
	options.MaxRedirect = ast.CountFinite{Value: 10}
	options.PathAsIs = true
	options.Proxy = "localhost:3128"
	options.UnixSocket = "/var/run/example.sock"
	options.User = &user
	options.UserAgent = &userAgent

	cmd := client.CurlCommandLine(request, util.ContextDir{}, util.OutputFile{Path: "/tmp/foo.bin"}, options, newBufferedLogger(0))
	checkCmd(t, cmd, nil, "curl "+
		"--aws-sigv4 aws:amz:sts "+
		"--cacert /etc/cert.pem "+
		"--compressed "+
		"--connect-to example.com:443:host-47.example.com:443 "+
		"--insecure "+
		"--max-redirs 10 "+
		"--path-as-is "+
		"--proxy 'localhost:3128' "+
		"--unix-socket '/var/run/example.sock' "+
		"--user 'user:password' "+
		"--user-agent 'my-useragent' "+
		"--output /tmp/foo.bin "+
		"'https://example.org'")
}
