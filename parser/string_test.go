package parser

import (
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestUnquotedTemplate(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Template
		index int
	}{
		{"", tmpl(0, si(1, 1, 1, 1)), 0},
		{"a#", strTmpl("a", si(1, 1, 1, 2)), 1},
		{`a\u{23}`, tmpl(0, si(1, 1, 1, 8), strSrc("a#", `a\u{23}`)), 7},
		{`"hi"`, strTmpl(`"hi"`, si(1, 1, 1, 5)), 4},
		{`hello\u{20}{{name}}!`, tmpl(0, si(1, 1, 1, 21), strSrc("hello ", `hello\u{20}`), phElem("name", 1, 14), str("!")), 20},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(unquotedTemplate(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestQuotedTemplate(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Template
		index int
	}{
		{`""`, tmpl('"', si(1, 1, 1, 3)), 2},
		{`"a#"`, tmpl('"', si(1, 1, 1, 5), str("a#")), 4},
		{`"{0}"`, tmpl('"', si(1, 1, 1, 6), str("{0}")), 5},
		{`"\"hi\""`, tmpl('"', si(1, 1, 1, 9), strSrc(`"hi"`, `\"hi\"`)), 8},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(quotedTemplate(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestQuotedTemplateErrorMissingClosingQuote(t *testing.T) {
	r := reader.New(`"not found`)
	_, err := quotedTemplate(r)
	assertError(t, err, pos(1, 11), false, nil)
}

func TestQuotedString(t *testing.T) {
	r := reader.New(`""`)
	assertEqual(t, must(quotedOnelineString(r)), "")
	assertCursor(t, r, 2)

	r = reader.New(`"Hello"`)
	assertEqual(t, must(quotedOnelineString(r)), "Hello")
	assertCursor(t, r, 7)
}

func TestBacktickTemplate(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Template
		index int
	}{
		{"``", tmpl('`', si(1, 1, 1, 3)), 2},
		{"`foo#`", tmpl('`', si(1, 1, 1, 7), str("foo#")), 6},
		{"`{0}`", tmpl('`', si(1, 1, 1, 6), str("{0}")), 5},
		{"`\\`hi\\``", tmpl('`', si(1, 1, 1, 9), strSrc("`hi`", "\\`hi\\`")), 8},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(backtickTemplate(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestBacktickTemplateErrorMissingClosingBacktick(t *testing.T) {
	r := reader.New("`not found")
	_, err := backtickTemplate(r)
	assertError(t, err, pos(1, 11), false, nil)
}

func TestAnyChar(t *testing.T) {
	tests := []struct {
		input  string
		c      rune
		source string
		index  int
	}{
		{"a", 'a', "a", 1},
		{" ", ' ', " ", 1},
		{`\t`, '\t', `\t`, 2},
		{"#", '#', "#", 1},
		{`\"`, '"', `\"`, 2},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		c, s, err := anyChar(nil, r)
		if err != nil || c != tt.c || s != tt.source {
			t.Fatalf("anyChar(%q) = %q, %q, %v", tt.input, c, s, err)
		}
		assertCursor(t, r, tt.index)
	}
}

func TestAnyCharError(t *testing.T) {
	tests := []struct {
		input  string
		except []rune
	}{
		{"", nil},
		{"#", []rune{'#'}},
		{"\t", nil},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		_, _, err := anyChar(tt.except, r)
		assertError(t, err, pos(1, 1), true, nil)
	}
}

func TestEscapeChar(t *testing.T) {
	r := reader.New(`\n`)
	assertEqual(t, must(escapeChar(r)), '\n')
	assertCursor(t, r, 2)

	r = reader.New(`\u{0a}`)
	assertEqual(t, must(escapeChar(r)), '\n')
	assertCursor(t, r, 6)

	r = reader.New("x")
	_, err := escapeChar(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: `\`})
	assertCursor(t, r, 0)
}

func TestUnicode(t *testing.T) {
	r := reader.New("{000a}")
	assertEqual(t, must(unicodeChar(r)), '\n')
	assertCursor(t, r, 6)

	r = reader.New("{E9}")
	assertEqual(t, must(unicodeChar(r)), 'é')
	assertCursor(t, r, 4)
}

func TestHexValue(t *testing.T) {
	r := reader.New("20x")
	assertEqual(t, must(hexValue(r)), uint32(32))

	r = reader.New("x")
	_, err := hexValue(r)
	assertError(t, err, pos(1, 1), false, ParseErrorHexDigit{})
}

func TestQuotedTemplateBenchmark(t *testing.T) {
	r := reader.New(`"Hello World!"` + strings.Repeat("X", 10_000_000))
	must(quotedTemplate(r))
	assertCursor(t, r, 14)
}
