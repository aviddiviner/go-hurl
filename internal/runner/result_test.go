package runner

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/parser"
)

func entryResultWithBody(contentType string, body []byte) EntryResult {
	r := DefaultEntryResult()
	response := defaultHTTPResponse()
	response.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("Content-Type", contentType))
	response.Body = body
	r.Calls = []httpclient.Call{{Response: response}}
	return r
}

func TestWriteResponse(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		mode        pretty.PrettyMode
		want        string
	}{
		{"json pretty", "application/json", `{"a":1}`, pretty.Automatic, "{\n  \"a\": 1\n}\n"},
		{"invalid json falls back to raw bytes", "application/json", `{"a":`, pretty.Automatic, `{"a":`},
		{"text", "text/plain", `{"a":1}`, pretty.Automatic, `{"a":1}`},
		{"force", "text/plain", `{"a":1}`, pretty.Force, "{\n  \"a\": 1\n}\n"},
		{"none", "application/json", `{"a":1}`, pretty.None, `{"a":1}`},
	}
	for _, tt := range tests {
		r := entryResultWithBody(tt.contentType, []byte(tt.body))
		stdout := term.NewStdout(term.WriteModeBuffered)
		if err := r.WriteResponse(nil, stdout, false, false, tt.mode, false, sourceInfo(1, 1, 1, 1)); err != nil {
			t.Errorf("%s: %v", tt.name, err)
		}
		if got := string(stdout.Buffer()); got != tt.want {
			t.Errorf("%s: got %q; want %q", tt.name, got, tt.want)
		}
	}
}

func TestWriteResponseIncludeHeaders(t *testing.T) {
	r := entryResultWithBody("text/plain", []byte("hello"))
	stdout := term.NewStdout(term.WriteModeBuffered)
	if err := r.WriteResponse(util.OutputStdout{}, stdout, true, false, pretty.None, false, sourceInfo(1, 1, 1, 1)); err != nil {
		t.Fatal(err)
	}
	if got := string(stdout.Buffer()); got != "HTTP/1.0 200\nContent-Type: text/plain\n\nhello" {
		t.Errorf("got %q", got)
	}
}

func TestWriteResponseFileError(t *testing.T) {
	r := entryResultWithBody("text/plain", []byte("hello"))
	path := filepath.Join(t.TempDir(), "missing", "out.txt")
	err := r.WriteResponse(util.OutputFile{Path: path}, term.NewStdout(term.WriteModeBuffered), false, false, pretty.None, false, sourceInfo(3, 1, 3, 9))
	if err == nil || err.SourceInfo() != sourceInfo(3, 1, 3, 9) {
		t.Fatalf("err = %#v", err)
	}
	if k, ok := err.Kind.(RunnerErrorFileWriteAccess); !ok || k.Path != path {
		t.Errorf("kind = %#v", err.Kind)
	}
}

func TestRunEntryOutput(t *testing.T) {
	srv := smokeServer()
	defer srv.Close()
	dir := t.TempDir()
	options := DefaultRunnerOptions()
	options.ConnectsTo = []string{connectTo(srv, "smoke.test:80")}
	options.ContextDir = util.NewContextDir(dir, dir)
	content := "POST http://smoke.test/login\n[Form]\nuser: bob\n[Options]\noutput: out.json\nHTTP 200\n\nGET http://smoke.test/hello\n[Options]\noutput: ../escape.txt\nHTTP 200\n"
	result, stderr := runHurl(t, content, "output.hurl", options, NewVariableSet())
	got, err := os.ReadFile(filepath.Join(dir, "out.json"))
	if err != nil || string(got) != `{"token":"t0k3n","user":{"id":42,"roles":["admin","dev"]}}` {
		t.Errorf("out.json = %q, %v", got, err)
	}
	// The second output is outside the file root.
	if result.Success || len(result.Entries) != 2 || len(result.Entries[1].Errors) != 1 {
		t.Fatalf("result = %+v\n%s", result, stderr)
	}
	if err := result.Entries[1].Errors[0]; err.Kind != (RunnerErrorUnauthorizedFileAccess{Path: "../escape.txt"}) || err.SourceInfo() != sourceInfo(10, 9, 10, 22) {
		t.Errorf("err = %#v", err)
	}
}

func TestRunParseError(t *testing.T) {
	result, err := Run("GET http://localhost\nHTTP 200\n[Asserts]\nfoo\n", nil, DefaultRunnerOptions(), NewVariableSet(), logger.LoggerOptions{})
	var perr *parser.ParseError
	if result != nil || !errors.As(err, &perr) || perr.Description() != "Parsing method" {
		t.Errorf("result = %v, err = %v", result, err)
	}
}

func TestRunNoEntry(t *testing.T) {
	result := RunEntries(nil, "", nil, DefaultRunnerOptions(), NewVariableSet(), term.NewStdout(term.WriteModeBuffered), nil, testLogger())
	if !result.Success || len(result.Entries) != 0 || result.CookieStore == nil || result.Variables == nil {
		t.Errorf("result = %+v", result)
	}
}
