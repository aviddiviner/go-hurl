package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestQuery(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Query
	}{
		{"status", ast.Query{SourceInfo: si(1, 1, 1, 7), Value: ast.QueryStatus{}}},
		{"bytes", ast.Query{SourceInfo: si(1, 1, 1, 6), Value: ast.QueryBytes{}}},
		{"rawbytes", ast.Query{SourceInfo: si(1, 1, 1, 9), Value: ast.QueryRawBytes{}}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(query(r)), tt.want)
	}
}

func TestHeaderQuery(t *testing.T) {
	r := reader.New(`header "Foo"`)
	assertEqual(t, must(headerQuery(r)), ast.QueryValue(ast.QueryHeader{
		Space0: ws(" ", 1, 7, 1, 8),
		Name:   tmpl('"', si(1, 8, 1, 13), str("Foo")),
	}))
}

func TestCookieQuery(t *testing.T) {
	r := reader.New(`cookie "Foo[Domain]"`)
	assertEqual(t, must(cookieQuery(r)), ast.QueryValue(ast.QueryCookie{
		Space0: ws(" ", 1, 7, 1, 8),
		Expr: ast.CookiePath{
			Name: strTmpl("Foo", si(1, 9, 1, 12)),
			Attribute: &ast.CookieAttribute{
				Space0: ews(1, 13),
				Name:   ast.CookieAttributeName{Kind: ast.CookieAttributeDomain, Value: "Domain"},
				Space1: ews(1, 19),
			},
		},
	}))
	assertCursor(t, r, 20)
}

func TestXPathQuery(t *testing.T) {
	r := reader.New(`xpath "normalize-space(//head/title)"`)
	assertEqual(t, must(xpathQuery(r)), ast.QueryValue(ast.QueryXPath{
		Space0: ws(" ", 1, 6, 1, 7),
		Expr:   tmpl('"', si(1, 7, 1, 38), str("normalize-space(//head/title)")),
	}))

	expr := "normalize-space(//div[contains(concat(' ',normalize-space(@class),' '),' monthly-price ')])"
	r = reader.New(`xpath "` + expr + `"`)
	assertEqual(t, must(xpathQuery(r)), ast.QueryValue(ast.QueryXPath{
		Space0: ws(" ", 1, 6, 1, 7),
		Expr:   tmpl('"', si(1, 7, 1, 100), str(expr)),
	}))
}

func TestJSONPathQuery(t *testing.T) {
	r := reader.New(`jsonpath "$['statusCode']"`)
	assertEqual(t, must(jsonpathQuery(r)), ast.QueryValue(ast.QueryJSONPath{
		Space0: ws(" ", 1, 9, 1, 10),
		Expr:   tmpl('"', si(1, 10, 1, 27), str("$['statusCode']")),
	}))

	r = reader.New(`jsonpath "$.success"`)
	assertEqual(t, must(jsonpathQuery(r)), ast.QueryValue(ast.QueryJSONPath{
		Space0: ws(" ", 1, 9, 1, 10),
		Expr:   tmpl('"', si(1, 10, 1, 21), str("$.success")),
	}))
}

func TestQueryWithFilters(t *testing.T) {
	r := reader.New("body urlDecode ")
	assertEqual(t, must(query(r)), ast.Query{SourceInfo: si(1, 1, 1, 5), Value: ast.QueryBody{}})
	assertEqual(t, must(filters(r)), []ast.SpacedFilter{{
		Space0: ws(" ", 1, 5, 1, 6),
		Filter: ast.Filter{SourceInfo: si(1, 6, 1, 15), Value: ast.FilterURLDecode{}},
	}})
	assertCursor(t, r, 14)
}
