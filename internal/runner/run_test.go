package runner

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/text"
	"github.com/aviddiviner/go-hurl/parser"
)

// runHurl parses and runs content with buffered standard output and error,
// and returns the result and the standard error.
func runHurl(t *testing.T, content, filename string, options RunnerOptions, variables *VariableSet) (HurlResult, string) {
	t.Helper()
	hurlFile, err := parser.ParseHurlFile(content)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	stdout := term.NewStdout(term.WriteModeBuffered)
	stderr := term.NewStderr(term.WriteModeBuffered)
	l := logger.New(logger.LoggerOptions{}, stderr, variables.Secrets())
	in := input.New(filename)
	result := RunEntries(hurlFile.Entries, content, &in, options, variables, stdout, nil, l)
	return result, stderr.Buffer()
}

// connectTo returns a --connect-to option routing host:port to srv.
func connectTo(srv *httptest.Server, hostPort string) string {
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	return hostPort + ":127.0.0.1:" + port
}

func smokeServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("user") != "bob" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"token":"t0k3n","user":{"id":42,"roles":["admin","dev"]}}`)
	})
	mux.HandleFunc("/users/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer t0k3n" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/xml")
		w.Header().Set("X-Request-Id", "rid-1")
		fmt.Fprint(w, `<?xml version="1.0"?><user id="42"><name>Bob</name><email>bob@example.org</email></user>`)
	})
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie("session")
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "Hello %s!\n", c.Value)
	})
	return httptest.NewServer(mux)
}

const smokeHurl = `# Log in and capture a token
POST http://smoke.test/login
[Form]
user: bob
HTTP 200
Content-Type: application/json
[Captures]
token: jsonpath "$.token"
user_id: jsonpath "$.user.id"
[Asserts]
jsonpath "$.user.roles" count == 2
jsonpath "$.user.roles[0]" == "admin"
cookie "session" == "abc123"

GET http://smoke.test/users/{{user_id}}
Authorization: Bearer {{token}}
HTTP 200
X-Request-Id: rid-1
[Captures]
name: xpath "string(//user/name)"
[Asserts]
status == 200
header "Content-Type" contains "xml"
xpath "string(/user/@id)" == "42"
xpath "//user/email" count == 1
xpath "string(//email)" endsWith "@example.org"

GET http://smoke.test/hello
HTTP 200
[Asserts]
body startsWith "Hello"
body == "Hello {{name}}!\n"
`

func TestRunSmoke(t *testing.T) {
	srv := smokeServer()
	defer srv.Close()
	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "smoke.test:80")}
	options.ContinueOnError = true

	result, stderr := runHurl(t, smokeHurl, "smoke.hurl", options, NewVariableSet())

	if result.Success {
		t.Errorf("run should fail (last entry)")
	}
	if len(result.Entries) != 3 {
		t.Fatalf("%d entries; stderr:\n%s", len(result.Entries), stderr)
	}
	login, user, hello := result.Entries[0], result.Entries[1], result.Entries[2]
	if len(login.Errors) != 0 || len(user.Errors) != 0 {
		t.Fatalf("unexpected errors: %v %v\n%s", login.Errors, user.Errors, stderr)
	}
	wantCaptures := []CaptureResult{{Name: "token", Value: NewString("t0k3n")}, {Name: "user_id", Value: NewInteger(42)}}
	if !reflect.DeepEqual(login.Captures, wantCaptures) {
		t.Errorf("login captures = %#v", login.Captures)
	}
	if !reflect.DeepEqual(user.Captures, []CaptureResult{{Name: "name", Value: NewString("Bob")}}) {
		t.Errorf("user captures = %#v", user.Captures)
	}
	// Version, status, implicit header asserts and explicit asserts.
	if len(login.Asserts) != 6 || len(user.Asserts) != 8 || len(hello.Asserts) != 4 {
		t.Errorf("asserts: %d, %d, %d", len(login.Asserts), len(user.Asserts), len(hello.Asserts))
	}
	if v, ok := result.Variables.Get("name"); !ok || !Equal(v.Value, NewString("Bob")) {
		t.Errorf("variable name = %#v", v)
	}
	if len(login.Calls) != 1 || login.Calls[0].Request.Url.String() != "http://smoke.test/login" {
		t.Errorf("calls = %#v", login.Calls)
	}
	if result.CookieStore.Len() != 1 {
		t.Errorf("cookie store = %v", result.CookieStore.ToNetscape())
	}

	// The last assert fails: the body is "Hello abc123!\n".
	if len(hello.Errors) != 1 {
		t.Fatalf("hello errors = %v", hello.Errors)
	}
	err := hello.Errors[0]
	if !err.Assert {
		t.Errorf("error should be an assert error")
	}
	want := RunnerErrorAssertFailure{Actual: "string <Hello abc123!\n>", Expected: "string <Hello Bob!\n>"}
	if err.Kind != want {
		t.Errorf("error kind = %#v", err.Kind)
	}
	if errors := result.Errors(); len(errors) != 1 || errors[0].Error != err {
		t.Errorf("result errors = %v", errors)
	}
	wantStderr := `error: Assert failure
  --> smoke.hurl:32:0
   |
   | GET http://smoke.test/hello
   | ...
32 | body == "Hello {{name}}!\n"
   |   actual:   string <Hello abc123!
   |>
   |   expected: string <Hello Bob!
   |>
   |

`
	if stderr != wantStderr {
		t.Errorf("stderr:\n%s\nwant:\n%s", stderr, wantStderr)
	}
}

func TestRunStopsOnError(t *testing.T) {
	srv := smokeServer()
	defer srv.Close()
	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "smoke.test:80")}
	content := "GET http://smoke.test/hello\nHTTP 200\n\nGET http://smoke.test/hello\nHTTP 400\n"
	result, stderr := runHurl(t, content, "stop.hurl", options, NewVariableSet())
	if result.Success || len(result.Entries) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if err := result.Entries[0].Errors[0]; err.Kind != (RunnerErrorAssertStatus{Actual: "400"}) {
		t.Errorf("error = %#v", err.Kind)
	}
	if !strings.HasPrefix(stderr, "error: Assert status code\n  --> stop.hurl:2:6\n") {
		t.Errorf("stderr:\n%s", stderr)
	}
}

func TestRunUnsupportedOption(t *testing.T) {
	srv := smokeServer()
	defer srv.Close()
	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "smoke.test:80")}
	for _, option := range []string{"http3: true", "ntlm: true", "aws-sigv4: aws:amz:eu-central-1:sts"} {
		content := "GET http://smoke.test/hello\n[Options]\n" + option + "\nHTTP 200\n"
		result, _ := runHurl(t, content, "unsupported.hurl", options, NewVariableSet())
		if result.Success || len(result.Entries) != 1 || len(result.Entries[0].Errors) != 1 {
			t.Fatalf("%s: result = %+v", option, result)
		}
		err := result.Entries[0].Errors[0]
		k, ok := err.Kind.(RunnerErrorHTTP)
		if !ok || !strings.Contains(k.Value.Message(), "not supported by go-hurl") || err.SourceInfo() != sourceInfo(1, 5, 1, 28) {
			t.Errorf("%s: error = %#v", option, err)
		}
	}
}

func TestRunRetryAndRepeat(t *testing.T) {
	count := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		fmt.Fprint(w, count)
	}))
	defer srv.Close()
	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "retry.test:80")}
	content := "GET http://retry.test\n[Options]\nretry: 5\nretry-interval: 1\nHTTP 200\n`3`\n\nGET http://retry.test\n[Options]\nrepeat: 2\nHTTP 200\n"
	result, stderr := runHurl(t, content, "retry.hurl", options, NewVariableSet())
	if !result.Success || len(result.Entries) != 5 {
		t.Fatalf("result = %+v\n%s", result, stderr)
	}
	indexes := []int{}
	for _, e := range result.Entries {
		indexes = append(indexes, e.EntryIndex.Get())
	}
	if !reflect.DeepEqual(indexes, []int{1, 1, 1, 2, 2}) || len(result.Entries[0].Errors) != 1 || stderr != "" {
		t.Errorf("indexes = %v, stderr = %q", indexes, stderr)
	}
}

// TestRunIntegrationDiff runs Hurl's integration test tests_failed/diff and
// checks the standard error against the expected one.
func TestRunIntegrationDiff(t *testing.T) {
	dir := "../../integration/hurl/tests_failed/diff/"
	content, err := os.ReadFile(dir + "diff.hurl")
	if err != nil {
		t.Skip(err)
	}
	py, _ := os.ReadFile(dir + "diff.py")
	wantErr, _ := os.ReadFile(dir + "diff.err")

	routes := map[string]string{}
	re := regexp.MustCompile(`(?s)@app\.route\("([^"]+)"\)\ndef \w+\(\):\n    return """(.*?)"""`)
	for _, m := range re.FindAllStringSubmatch(string(py), -1) {
		routes[m[1]] = m[2]
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, routes[r.URL.Path])
	}))
	defer srv.Close()

	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "localhost:8000")}
	options.ContinueOnError = true
	result, stderr := runHurl(t, string(content), "tests_failed/diff/diff.hurl", options, NewVariableSet())
	if result.Success || len(result.Entries) != 9 {
		t.Fatalf("result = %+v", result)
	}
	if stderr != string(wantErr) {
		t.Errorf("stderr:\n%s\nwant:\n%s", stderr, wantErr)
	}
}

func TestRunDiffColor(t *testing.T) {
	hunks := diff("a\nb\n", "a\nc\n")
	got := hunks[0].Content.Render(text.FormatANSI)
	if !strings.Contains(got, "\x1b[31m-b\n") || !strings.Contains(got, "\x1b[32m+c\n") {
		t.Errorf("got %q", got)
	}
}
