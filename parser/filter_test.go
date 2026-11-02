package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestFilterCount(t *testing.T) {
	r := reader.New("count")
	assertEqual(t, must(filter(r)), ast.Filter{SourceInfo: si(1, 1, 1, 6), Value: ast.FilterCount{}})
}

func TestFilterError(t *testing.T) {
	r := reader.New("xcount")
	_, err := filter(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "filter"})

	r = reader.New("regex 1")
	_, err = filter(r)
	assertError(t, err, pos(1, 7), false, ParseErrorExpecting{Value: `" or /`})
}
