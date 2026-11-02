package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestCookiepath(t *testing.T) {
	domain := ast.CookieAttributeName{Kind: ast.CookieAttributeDomain, Value: "Domain"}
	tests := []struct {
		input string
		want  ast.CookiePath
		index int
	}{
		{"cookie1", ast.CookiePath{Name: strTmpl("cookie1", si(1, 1, 1, 8))}, 7},
		{"cookie1[Domain]", ast.CookiePath{
			Name:      strTmpl("cookie1", si(1, 1, 1, 8)),
			Attribute: &ast.CookieAttribute{Space0: ews(1, 9), Name: domain, Space1: ews(1, 15)},
		}, 15},
		{"{{name}}[Domain]", ast.CookiePath{
			Name:      tmpl(0, si(1, 1, 1, 9), phElem("name", 1, 3)),
			Attribute: &ast.CookieAttribute{Space0: ews(1, 10), Name: domain, Space1: ews(1, 16)},
		}, 16},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(cookiepath(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestCookiepathError(t *testing.T) {
	for _, input := range []string{"cookie1[", "cookie1[{{field]"} {
		r := reader.New(input)
		_, err := cookiepath(r)
		assertError(t, err, pos(1, 9), false, ParseErrorInvalidCookieAttribute{})
	}

	// Errors are well reported with a reader that has already read data.
	r := reader.New("xxxx{{cookie[Domain]")
	r.ReadWhile(func(c rune) bool { return c == 'x' })
	_, err := cookiepath(r)
	assertError(t, err, pos(1, 13), false, ParseErrorExpecting{Value: "}}"})
}

func TestCookiepathAttributeName(t *testing.T) {
	for _, input := range []string{"Domain", "domain"} {
		r := reader.New(input)
		assertEqual(t, must(cookiepathAttributeName(r)), ast.CookieAttributeName{Kind: ast.CookieAttributeDomain, Value: input})
		assertCursor(t, r, 6)
	}
	r := reader.New("unknown")
	_, err := cookiepathAttributeName(r)
	assertError(t, err, pos(1, 1), false, ParseErrorInvalidCookieAttribute{})
}
