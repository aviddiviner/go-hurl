package parser

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
)

const integrationDir = "../integration"

// readHurlFile reads a file as hurl does: BOM removed and UTF-8 validated.
func readHurlFile(t *testing.T, path string) string {
	t.Helper()
	content, err := input.New(path).ReadToString()
	if err != nil {
		t.Fatal(err)
	}
	return content
}

// corpusOKSkips are files that hurl rejects before parsing.
var corpusOKSkips = map[string]bool{
	"parallel_io_b.hurl": true, // invalid UTF-8, tested by tests_failed_not_linted/parallel_io
}

func TestCorpusOK(t *testing.T) {
	dirs := []string{
		"hurl/tests_ok",
		"hurl/tests_ok_not_linted",
		"hurl/tests_failed",
		"hurl/tests_failed_not_linted",
		"hurlfmt/tests_ok",
	}
	count := 0
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(integrationDir, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || filepath.Ext(path) != ".hurl" || corpusOKSkips[d.Name()] {
				return err
			}
			count++
			if _, perr := ParseHurlFile(readHurlFile(t, path)); perr != nil {
				t.Errorf("%s: %v", path, perr)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if count == 0 {
		t.Fatal("no corpus files found")
	}
	t.Logf("parsed %d files", count)
}

// errorParserSkips are tests_error_parser cases that can't be checked
// against an .err file.
var errorParserSkips = map[string]string{
	"invalid_regex": "Go's regexp accepts `{a}` as a literal, while Rust's regex crate rejects it",
	"invalid_utf8":  "not a parser error: hurl rejects the invalid UTF-8 input when reading the file",
}

// errorParserValid are the tests_error_parser files that are valid: in the
// parallel_parsing_error test, only file c has an error.
var errorParserValid = map[string]bool{
	"parallel_parsing_error_a": true,
	"parallel_parsing_error_b": true,
	"parallel_parsing_error_d": true,
}

func TestCorpusErrorParser(t *testing.T) {
	dir := filepath.Join(integrationDir, "hurl", "tests_error_parser")
	files, err := filepath.Glob(filepath.Join(dir, "*.hurl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no corpus files found")
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".hurl")
		t.Run(name, func(t *testing.T) {
			if reason, ok := errorParserSkips[name]; ok {
				t.Skip(reason)
			}
			content := readHurlFile(t, f)
			_, perr := ParseHurlFile(content)
			if errorParserValid[name] {
				if perr != nil {
					t.Fatalf("unexpected error: %v", perr)
				}
				return
			}
			if perr == nil {
				t.Fatal("parsed without error")
			}

			errFile, color := name, false
			if name == "parallel_parsing_error_c" {
				// Run with --color, and the .err file is named after the test.
				errFile, color = "parallel_parsing_error", true
			}
			want := readHurlFile(t, filepath.Join(dir, errFile+".err"))

			// As printed by hurl: logger.error_rich(&error.render(...)).
			message := sourceerr.Render(perr, "tests_error_parser/"+name+".hurl", content, nil, sourceerr.OutputFormatTerminal{Color: color})
			got := "error: " + message + "\n\n"
			if color {
				got = "\x1b[1;31merror\x1b[0m: " + message + "\n\n"
			}
			if got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
