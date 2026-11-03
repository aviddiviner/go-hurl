package tap

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestParseTapTestLine(t *testing.T) {
	_, err := parseTestcase("toto")
	if err == nil || err.Error() != "Invalid TAP line <toto> - must start with ok or nok" {
		t.Errorf("err = %v", err)
	}
	tc, err := parseTestcase("ok 1 - tests_ok/test.1.hurl")
	if err != nil {
		t.Fatal(err)
	}
	if want := (Testcase{description: "tests_ok/test.1.hurl", success: true}); tc != want {
		t.Errorf("got %+v, want %+v", tc, want)
	}
}

func TestParseTapReport(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []Testcase
	}{
		{
			name: "no version",
			in:   "1..3\nok 1 - tests_ok/test.1.hurl\n ok 2  -tests_ok/test.2.hurl\nnot ok 3 - tests_ok/test.3.hurl\n\n",
			want: []Testcase{
				{"tests_ok/test.1.hurl", true},
				{"tests_ok/test.2.hurl", true},
				{"tests_ok/test.3.hurl", false},
			},
		},
		{
			name: "version",
			in:   "TAP version 13\n1..3\nok 1 - tests_ok/test.1.hurl\n ok 2  -tests_ok/test.2.hurl\nnot ok 3 - tests_ok/test.3.hurl\n\n",
			want: []Testcase{
				{"tests_ok/test.1.hurl", true},
				{"tests_ok/test.2.hurl", true},
				{"tests_ok/test.3.hurl", false},
			},
		},
		{
			name: "header comment",
			in:   "TAP version 13\n1..5 # TAP header can have comments\nok 1 - test.1.hurl\nok 2 - test.2.hurl\nnot ok 3 - test.3.hurl\nnot ok 4 - test.4.hurl\nok 5 - test.5.hurl\n",
			want: []Testcase{
				{"test.1.hurl", true},
				{"test.2.hurl", true},
				{"test.3.hurl", false},
				{"test.4.hurl", false},
				{"test.5.hurl", true},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTapReport(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	s := "Dummy header\nok 1 - test.1.hurl\nok 2 - test.2.hurl\nnot ok 3 - test.3.hurl\n"
	if _, err := parseTapReport(s); err == nil {
		t.Error("expected error")
	}
	if _, err := parseTapReport("TAP version 13\n"); err == nil || err.Error() != "Invalid TAP Header <>" {
		t.Errorf("err = %v", err)
	}
}

func failedResult() *runner.HurlResult {
	pos := reader.Pos{Line: 1, Column: 1}
	return &runner.HurlResult{Entries: []runner.EntryResult{{
		EntryIndex: ast.NewIndex(1),
		Errors:     []*runner.RunnerError{runner.NewRunnerError(ast.SourceInfo{Start: pos, End: pos}, runner.RunnerErrorAssertStatus{Actual: "404"}, true)},
	}}}
}

// TestWriteReportAppend follows integration/hurl/tests_ok/tap/tap.sh.
func TestWriteReportAppend(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "build", "tap", "result.tap")

	run1 := []Testcase{
		NewTestcase(&runner.HurlResult{Success: true}, input.New("tests_ok/tap/test.1.hurl")),
		NewTestcase(failedResult(), input.New("tests_ok/tap/test.2.hurl")),
	}
	if err := WriteReport(filename, run1); err != nil {
		t.Fatal(err)
	}
	run2 := []Testcase{NewTestcase(&runner.HurlResult{Success: true}, input.New("tests_ok/tap/test.3.hurl"))}
	if err := WriteReport(filename, run2); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../../integration/hurl/tests_ok/tap/tap.out")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteReportInvalidExisting(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "result.tap")
	if err := os.WriteFile(filename, []byte("Dummy header\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	err := WriteReport(filename, nil)
	if err == nil || err.Error() != "Invalid TAP Header <Dummy header>" {
		t.Errorf("err = %v", err)
	}
}
