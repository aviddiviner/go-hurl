package lint

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/format"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/parser"
)

func TestLintHurlFile(t *testing.T) {
	src := `
    # comment 1
  #comment 2 with trailing spaces
  GET   https://foo.com
[Form]
  bar : baz
  [Options]
 location : true
HTTP   200`
	file, err := parser.ParseHurlFile(src)
	if err != nil {
		t.Fatal(err)
	}
	want := `
# comment 1
#comment 2 with trailing spaces
GET https://foo.com
[Options]
location: true
[Form]
bar: baz
HTTP 200
`
	if got := HurlFile(file); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestExport checks the hurlfmt Hurl output (lint, reparse, format) against
// the expected .lint.hurl files.
func TestExport(t *testing.T) {
	paths, err := filepath.Glob("../../integration/hurlfmt/tests_export/*.lint.hurl")
	if err != nil || len(paths) == 0 {
		t.Fatalf("no files: %v", err)
	}
	for _, want := range paths {
		src := strings.TrimSuffix(want, ".lint.hurl") + ".hurl"
		content, err := input.New(src).ReadToString()
		if err != nil {
			t.Fatal(err)
		}
		file, perr := parser.ParseHurlFile(content)
		if perr != nil {
			t.Fatalf("%s: %v", src, perr)
		}
		linted := HurlFile(file)
		file, perr = parser.ParseHurlFile(linted)
		if perr != nil {
			t.Fatalf("%s: linted output doesn't parse: %v", src, perr)
		}
		got := format.Text(file, false)
		if !strings.HasSuffix(got, "\n") {
			got += "\n"
		}
		expected, err := os.ReadFile(want)
		if err != nil {
			t.Fatal(err)
		}
		if got != string(expected) {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", src, got, expected)
		}
	}
}

// TestLintedCorpus checks that linting is idempotent on files that are
// already linted upstream.
func TestLintedCorpus(t *testing.T) {
	var paths []string
	err := filepath.WalkDir("../../integration/hurl/tests_ok", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && filepath.Ext(path) == ".hurl" {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil || len(paths) == 0 {
		t.Fatalf("no files: %v", err)
	}
	for _, path := range paths {
		content, err := input.New(path).ReadToString()
		if err != nil {
			t.Fatal(err)
		}
		file, perr := parser.ParseHurlFile(content)
		if perr != nil {
			t.Fatalf("%s: %v", path, perr)
		}
		if got := HurlFile(file); got != content {
			t.Errorf("%s: would reformat", path)
		}
	}
}
