package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// placeholder parses {{ expr }}.
func placeholder(r *reader.Reader) (ast.Placeholder, *ParseError) {
	if err := tryLiteral("{{", r); err != nil {
		return ast.Placeholder{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Placeholder{}, err
	}
	e, err := expr(r)
	if err != nil {
		return ast.Placeholder{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Placeholder{}, err
	}
	if err := literal("}}", r); err != nil {
		return ast.Placeholder{}, err
	}
	return ast.Placeholder{Space0: space0, Expr: e, Space1: space1}, nil
}
