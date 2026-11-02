package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestFilename(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Template
		index int
	}{
		{"data/data.bin", strTmpl("data/data.bin", si(1, 1, 1, 14)), 13},
		{"data.bin", strTmpl("data.bin", si(1, 1, 1, 9)), 8},
		{`file\ with\ spaces`, tmpl(0, si(1, 1, 1, 19), strSrc("file with spaces", `file\ with\ spaces`)), 18},
		{`filename\{`, tmpl(0, si(1, 1, 1, 11), strSrc("filename{", `filename\{`)), 10},
		{"foo_{{bar}}", tmpl(0, si(1, 1, 1, 12), str("foo_"), phElem("bar", 1, 7)), 11},
		{"foo_{{bar}}_baz", tmpl(0, si(1, 1, 1, 16), str("foo_"), phElem("bar", 1, 7), str("_baz")), 15},
		{"foo\r\n", strTmpl("foo", si(1, 1, 1, 4)), 3},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(filename(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestFilenameError(t *testing.T) {
	r := reader.New("{")
	_, err := filename(r)
	assertError(t, err, pos(1, 1), false, ParseErrorFilename{})

	r = reader.New(`\:`)
	_, err = filename(r)
	assertError(t, err, pos(1, 2), false, ParseErrorEscapeChar{})
}
