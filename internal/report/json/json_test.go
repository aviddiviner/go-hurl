package json

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/reader"
)

const integrationDir = "../../../integration/hurl"

func srcInfo(line, col, endLine, endCol int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: line, Column: col}, End: reader.Pos{Line: endLine, Column: endCol}}
}

// patternRegexp converts an integration .pattern file (literal text with
// <<<regex>>> parts) to a regexp.
func patternRegexp(t *testing.T, path string) *regexp.Regexp {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var re strings.Builder
	re.WriteString(`(?s)^`)
	rest := string(b)
	for {
		start := strings.Index(rest, "<<<")
		if start < 0 {
			re.WriteString(regexp.QuoteMeta(rest))
			break
		}
		end := strings.Index(rest[start:], ">>>")
		re.WriteString(regexp.QuoteMeta(rest[:start]))
		re.WriteString(rest[start+3 : start+end])
		rest = rest[start+end+3:]
	}
	re.WriteString(`$`)
	return regexp.MustCompile(re.String())
}

// helloEntry returns an entry GET http://localhost:8000/hello, run against
// the integration test server, at line, with a body assert `body`.
func helloEntry(t *testing.T, index, line int, body string) runner.EntryResult {
	t.Helper()
	url, uerr := httpclient.ParseUrl("http://localhost:8000/hello")
	if uerr != nil {
		t.Fatal(uerr)
	}
	spec := httpclient.NewRequestSpec()
	spec.Method = "GET"
	spec.Url = url
	begin := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	bodyLine := line + 2
	return runner.EntryResult{
		EntryIndex: ast.NewIndex(index),
		SourceInfo: srcInfo(line, 1, line, 32),
		Calls: []httpclient.Call{{
			Request: httpclient.NewRequest("GET", url, httpclient.NewHeaderVec(
				httpclient.NewHeader("Host", "localhost:8000"),
				httpclient.NewHeader("Accept", "*/*"),
				httpclient.NewHeader("User-Agent", "hurl/8.1.0"),
			), nil),
			Response: httpclient.Response{
				Version: httpclient.HttpVersionHttp11,
				Status:  200,
				Headers: httpclient.NewHeaderVec(
					httpclient.NewHeader("Content-Length", "12"),
					httpclient.NewHeader("Content-Type", "text/html; charset=utf-8"),
					httpclient.NewHeader("Date", "Mon, 02 Nov 2026 10:00:00 GMT"),
					httpclient.NewHeader("Server", "Flask Server"),
					httpclient.NewHeader("Via", "waitress"),
				),
				Body: []byte("Hello World!"),
				Url:  url,
			},
			Timings: httpclient.Timings{BeginCall: begin, EndCall: begin.Add(2 * time.Millisecond), Total: 2 * time.Millisecond},
		}},
		Asserts: []runner.AssertResult{
			runner.AssertResultImplicitVersion{Actual: "HTTP/1.1", Expected: "HTTP", SourceInfo: srcInfo(line+1, 1, line+1, 5)},
			runner.AssertResultImplicitStatus{Actual: 200, Expected: 200, SourceInfo: srcInfo(line+1, 6, line+1, 9)},
			runner.AssertResultImplicitBody{
				Actual:     runner.NewString("Hello World!"),
				Expected:   runner.NewString(body),
				SourceInfo: srcInfo(bodyLine, 1, bodyLine, len(body)+3),
			},
		},
		CurlCmd: httpclient.NewCurlCmd(spec, httpclient.NewCookieStore(), util.ContextDir{}, nil, httpclient.DefaultClientOptions()),
	}
}

func newTestcase(t *testing.T, name string, entries ...runner.EntryResult) Testcase {
	t.Helper()
	filename := "tests_ok/report_json/" + name
	content, err := os.ReadFile(filepath.Join(integrationDir, filename))
	if err != nil {
		t.Fatal(err)
	}
	result := &runner.HurlResult{Entries: entries, Success: true, CookieStore: httpclient.NewCookieStore()}
	for i := range result.Entries {
		e := &result.Entries[i]
		for _, a := range e.Asserts {
			if err := a.ToRunnerError(); err != nil {
				e.Errors = append(e.Errors, err)
				result.Success = false
			}
		}
	}
	return NewTestcase(result, string(content), input.New(filename))
}

// TestWriteReportAppend follows integration/hurl/tests_ok/report_json.
func TestWriteReportAppend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "build", "report", "json")
	store := filepath.Join(dir, "store")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(dir, "report.json")

	run1 := []Testcase{
		newTestcase(t, "test.1.hurl", helloEntry(t, 1, 1, "Hello World!")),
		newTestcase(t, "test.2.hurl", helloEntry(t, 1, 1, "Hello World!"), helloEntry(t, 2, 6, "Goodbye World!")),
	}
	if err := WriteReport(index, run1, store, nil); err != nil {
		t.Fatal(err)
	}
	run2 := []Testcase{newTestcase(t, "test.3.hurl", helloEntry(t, 1, 1, "Hello World!"))}
	if err := WriteReport(index, run2, store, nil); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	re := patternRegexp(t, filepath.Join(integrationDir, "tests_ok/report_json/report_json.out.pattern"))
	if !re.Match(got) {
		t.Errorf("report doesn't match pattern:\n%s", got)
	}
	files, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Errorf("got %d store files, want 4", len(files))
	}
}

func TestWriteReportInvalidExisting(t *testing.T) {
	for _, existing := range []string{"", "null", "{}", `[{"foo":1}]`, "[1, 2", "\xff"} {
		dir := t.TempDir()
		index := filepath.Join(dir, "report.json")
		if err := os.WriteFile(index, []byte(existing), 0o666); err != nil {
			t.Fatal(err)
		}
		err := WriteReport(index, nil, dir, nil)
		if existing == "\xff" {
			want := "Issue reading JSON report " + index + " (invalid data)"
			if err == nil || err.Error() != want {
				t.Errorf("err = %v, want %s", err, want)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		got, _ := os.ReadFile(index)
		if string(got) != "[]\n" {
			t.Errorf("existing %q: got %q", existing, got)
		}
	}
}

// Existing runs are kept as is, with sorted keys and number literals.
func TestWriteReportKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	index := filepath.Join(dir, "report.json")
	existing := `[ {"time": 15, "success": true, "filename": "a.hurl", "entries": [], "cookies": [], "extra": 1.50e3} ]`
	if err := os.WriteFile(index, []byte(existing), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := WriteReport(index, nil, dir, nil); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(index)
	want := `[{"cookies":[],"entries":[],"extra":1.50e+3,"filename":"a.hurl","success":true,"time":15}]` + "\n"
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
