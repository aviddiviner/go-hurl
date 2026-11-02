package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestFunction(t *testing.T) {
	r := reader.New("newUuid")
	assertEqual(t, must(function(r)), ast.FunctionNewUUID)

	r = reader.New("name")
	_, err := function(r)
	assertError(t, err, pos(1, 1), true, nil)
}
