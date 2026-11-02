package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func function(r *reader.Reader) (ast.Function, *ParseError) {
	start := r.Cursor()
	switch r.ReadWhile(isVariableChar) {
	case "newDate":
		return ast.FunctionNewDate, nil
	case "newUuid":
		return ast.FunctionNewUUID, nil
	}
	return 0, newError(start.Pos, true, ParseErrorExpecting{Value: "function"})
}
