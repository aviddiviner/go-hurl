package parser

import (
	"regexp"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestSpace(t *testing.T) {
	r := reader.New("x")
	_, err := space(r)
	assertError(t, err, pos(1, 1), true, nil)
	assertCursor(t, r, 1)

	r = reader.New("  ")
	assertEqual(t, must(space(r)), ws(" ", 1, 1, 1, 2))
	assertCursor(t, r, 1)
}

func TestOneOrMoreSpaces(t *testing.T) {
	r := reader.New("  ")
	assertEqual(t, must(oneOrMoreSpaces(r)), ws("  ", 1, 1, 1, 3))

	r = reader.New("abc")
	_, err := oneOrMoreSpaces(r)
	assertError(t, err, pos(1, 1), false, nil)
}

func TestZeroOrMoreSpaces(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Whitespace
		index int
	}{
		{"  ", ws("  ", 1, 1, 1, 3), 2},
		{"xxx", ews(1, 1), 0},
		{" xxx", ws(" ", 1, 1, 1, 2), 1},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(zeroOrMoreSpaces(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestComment(t *testing.T) {
	r := reader.New("#\n")
	assertEqual(t, must(comment(r)), ast.Comment{Value: "", SourceInfo: si(1, 2, 1, 2)})

	r = reader.New("# comment\n")
	assertEqual(t, must(comment(r)), ast.Comment{Value: " comment", SourceInfo: si(1, 2, 1, 10)})
	assertCursor(t, r, 9)

	r = reader.New("xxx")
	_, err := comment(r)
	assertError(t, err, pos(1, 1), true, nil)
}

func TestLiteral(t *testing.T) {
	r := reader.New("hello")
	if err := literal("hello", r); err != nil {
		t.Fatal(err)
	}
	assertCursor(t, r, 5)

	tests := []struct {
		input string
		index int
	}{
		{"", 0},
		{"hi", 2},
		{"he", 2},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		err := literal("hello", r)
		assertError(t, err, pos(1, 1), false, ParseErrorExpecting{Value: "hello"})
		assertCursor(t, r, tt.index)
	}
}

func TestNewLine(t *testing.T) {
	r := reader.New("\n")
	assertEqual(t, must(newline(r)), ws("\n", 1, 1, 2, 1))
}

func TestKeyValue(t *testing.T) {
	r := reader.New("message: hello {{name}}! # comment")
	assertEqual(t, must(keyValue(r)), ast.KeyValue{
		Space0: ews(1, 1),
		Key:    strTmpl("message", si(1, 1, 1, 8)),
		Space1: ews(1, 8),
		Space2: ws(" ", 1, 9, 1, 10),
		Value:  tmpl(0, si(1, 10, 1, 25), str("hello "), phElem("name", 1, 18), str("!")),
		LineTerminator0: ast.LineTerminator{
			Space0:  ws(" ", 1, 25, 1, 26),
			Comment: &ast.Comment{Value: " comment", SourceInfo: si(1, 27, 1, 35)},
			Newline: ews(1, 35),
		},
	})
}

func TestKeyValueTemplate(t *testing.T) {
	r := reader.New("{{key}}: value")
	assertEqual(t, must(keyValue(r)), ast.KeyValue{
		Space0:          ews(1, 1),
		Key:             tmpl(0, si(1, 1, 1, 8), phElem("key", 1, 3)),
		Space1:          ews(1, 8),
		Space2:          ws(" ", 1, 9, 1, 10),
		Value:           strTmpl("value", si(1, 10, 1, 15)),
		LineTerminator0: lt(ews(1, 15), ews(1, 15)),
	})
	assertCursor(t, r, 14)
}

func TestKeyValueRecover(t *testing.T) {
	r := reader.New("{{key")
	_, err := keyValue(r)
	assertError(t, err, pos(1, 6), true, nil)
	assertCursor(t, r, 5) // does not reset cursor

	r = reader.New("GET ®http://google.fr")
	_, err = keyValue(r)
	assertError(t, err, pos(1, 5), true, nil)
	assertCursor(t, r, 5) // does not reset cursor
}

func TestBoolean(t *testing.T) {
	r := reader.New("true")
	if !must(boolean(r)) {
		t.Fatal("expected true")
	}
	for _, input := range []string{"xxx", "trux"} {
		r := reader.New(input)
		_, err := boolean(r)
		assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "true|false"})
	}
}

func TestHexDigit(t *testing.T) {
	r := reader.New("0")
	assertEqual(t, must(hexDigit(r)), 0)

	r = reader.New("x")
	_, err := hexDigit(r)
	assertError(t, err, pos(1, 1), true, ParseErrorHexDigit{})
}

func TestHex(t *testing.T) {
	r := reader.New("hex, ff;")
	assertEqual(t, must(hex(r)), ast.Hex{
		Space0: ws(" ", 1, 5, 1, 6),
		Value:  []byte{255},
		Source: "ff",
		Space1: ews(1, 8),
	})

	r = reader.New("hex,010203 ;")
	assertEqual(t, must(hex(r)), ast.Hex{
		Space0: ews(1, 5),
		Value:  []byte{1, 2, 3},
		Source: "010203",
		Space1: ws(" ", 1, 11, 1, 12),
	})
}

func TestHexError(t *testing.T) {
	r := reader.New("hex,012;")
	_, err := hex(r)
	assertError(t, err, pos(1, 8), false, ParseErrorOddNumberOfHexDigits{})
}

func TestRegex(t *testing.T) {
	tests := []struct {
		input, pattern string
	}{
		{`/a{3}/`, `a{3}`},
		{`/a\/b/`, `a/b`},
		{`/a\.b/`, `a\.b`},
		{`/\d{4}-\d{2}-\d{2}/`, `\d{4}-\d{2}-\d{2}`},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(regex(r)), ast.Regex{Inner: regexp.MustCompile(tt.pattern), Source: tt.input})
	}
}

func TestRegexError(t *testing.T) {
	r := reader.New("xxx")
	_, err := regex(r)
	assertError(t, err, pos(1, 1), true, nil)

	r = reader.New("/xxx")
	_, err = regex(r)
	assertError(t, err, pos(1, 5), false, ParseErrorRegexExpr{Message: "unexpected end of file"})

	r = reader.New("/x{a}/")
	_, err = regex(r)
	assertError(t, err, pos(1, 2), false, ParseErrorRegexExpr{Message: "repetition quantifier expects a valid decimal"})
}

func TestFile(t *testing.T) {
	tests := []struct {
		input string
		want  ast.File
	}{
		{"file,data.xml;", ast.File{
			Space0:   ews(1, 6),
			Filename: strTmpl("data.xml", si(1, 6, 1, 14)),
			Space1:   ews(1, 14),
		}},
		{"file, filename1;", ast.File{
			Space0:   ws(" ", 1, 6, 1, 7),
			Filename: strTmpl("filename1", si(1, 7, 1, 16)),
			Space1:   ews(1, 16),
		}},
		{"file, tmp/filename1;", ast.File{
			Space0:   ws(" ", 1, 6, 1, 7),
			Filename: strTmpl("tmp/filename1", si(1, 7, 1, 20)),
			Space1:   ews(1, 20),
		}},
		{`file, tmp/filename\ with\ spaces.txt;`, ast.File{
			Space0:   ws(" ", 1, 6, 1, 7),
			Filename: tmpl(0, si(1, 7, 1, 37), strSrc("tmp/filename with spaces.txt", `tmp/filename\ with\ spaces.txt`)),
			Space1:   ews(1, 37),
		}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(file(r)), tt.want)
	}
}

func TestFileError(t *testing.T) {
	r := reader.New("fil; filename1;")
	_, err := file(r)
	assertError(t, err, pos(1, 1), true, nil)

	r = reader.New("file, filename1")
	_, err = file(r)
	assertError(t, err, pos(1, 16), false, ParseErrorExpecting{Value: ";"})

	r = reader.New(`file, tmp/filename\ with\ unescaped .txt;`)
	_, err = file(r)
	assertError(t, err, pos(1, 37), false, ParseErrorExpecting{Value: ";"})
}

func TestBase64(t *testing.T) {
	r := reader.New("base64,  T WE=;xxx")
	assertEqual(t, must(base64(r)), ast.Base64{
		Space0: ws("  ", 1, 8, 1, 10),
		Value:  []byte{77, 97},
		Source: "T WE=",
		Space1: ews(1, 15),
	})
	assertCursor(t, r, 15)
}
