package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestKeyString(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Template
		index int
	}{
		{`aaa\: `, tmpl(0, si(1, 1, 1, 6), strSrc("aaa:", `aaa\:`)), 5},
		{"$top:", strTmpl("$top", si(1, 1, 1, 5)), 4},
		{`key\u{20}\u{3a} :`, tmpl(0, si(1, 1, 1, 16), strSrc("key :", `key\u{20}\u{3a}`)), 15},
		{`values\u{5b}0\u{5d} :`, tmpl(0, si(1, 1, 1, 20), strSrc("values[0]", `values\u{5b}0\u{5d}`)), 19},
		{"values[0] :", strTmpl("values[0]", si(1, 1, 1, 10)), 9},
		{`\u{5b}0\u{5d}`, tmpl(0, si(1, 1, 1, 14), strSrc("[0]", `\u{5b}0\u{5d}`)), 13},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(keyString(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestKeyStringError(t *testing.T) {
	r := reader.New("")
	_, err := keyString(r)
	assertError(t, err, pos(1, 1), false, nil)

	r = reader.New("{{key")
	_, err = keyString(r)
	assertError(t, err, pos(1, 6), false, nil)

	// A key string can not start with a '[' (reserved for sections).
	r = reader.New("[0]:")
	_, err = keyString(r)
	assertError(t, err, pos(1, 1), false, nil)
	assertCursor(t, r, 3)

	r = reader.New(`\l`)
	_, err = keyString(r)
	assertError(t, err, pos(1, 2), false, ParseErrorEscapeChar{})

	r = reader.New(`{"id":1}`)
	_, err = keyString(r)
	assertError(t, err, pos(1, 1), false, ParseErrorExpecting{Value: "key-string"})
}

func TestKeyStringContent(t *testing.T) {
	r := reader.New(`aaa\:`)
	assertEqual(t, must(keyStringContent(r)), "aaa:")
}

func TestKeyStringText(t *testing.T) {
	r := reader.New(`aaa\:`)
	assertEqual(t, keyStringText(r), "aaa")
	assertCursor(t, r, 3)
}

func TestKeyStringEscapedChar(t *testing.T) {
	r := reader.New(`\u{0a}`)
	assertEqual(t, must(keyStringEscapedChar(r)), '\n')
	assertCursor(t, r, 6)

	r = reader.New(`\:`)
	assertEqual(t, must(keyStringEscapedChar(r)), ':')
	assertCursor(t, r, 2)

	r = reader.New("x")
	_, err := keyStringEscapedChar(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: `\`})
	assertCursor(t, r, 0)
}
