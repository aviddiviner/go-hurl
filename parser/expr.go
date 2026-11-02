package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// expr parses an expression. Currently, an expression can only be found
// inside a placeholder.
func expr(r *reader.Reader) (ast.Expr, *ParseError) {
	start := r.Cursor().Pos
	save := r.Cursor()
	var kind ast.ExprKind
	f, err := function(r)
	if err == nil {
		kind = ast.ExprFunction{Value: f}
	} else {
		if !err.Recoverable {
			return ast.Expr{}, err
		}
		r.Seek(save)
		v, err := variableName(r)
		if err != nil {
			return ast.Expr{}, err
		}
		kind = ast.ExprVariable{Value: v}
	}
	return ast.Expr{SourceInfo: sourceInfo(start, r.Cursor().Pos), Kind: kind}, nil
}

func isVariableChar(c rune) bool {
	return isAlphanumeric(c) || c == '_' || c == '-'
}

func variableName(r *reader.Reader) (ast.Variable, *ParseError) {
	start := r.Cursor()
	name := r.ReadWhile(isVariableChar)
	if name == "" {
		return ast.Variable{}, newError(start.Pos, false, ParseErrorTemplateVariable{})
	}
	return ast.Variable{Name: name, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}
