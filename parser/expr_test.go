package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestVariableName(t *testing.T) {
	r := reader.New("name")
	assertEqual(t, must(variableName(r)), ast.Variable{Name: "name", SourceInfo: si(1, 1, 1, 5)})

	r = reader.New("my-id")
	assertEqual(t, must(variableName(r)), ast.Variable{Name: "my-id", SourceInfo: si(1, 1, 1, 6)})
}
