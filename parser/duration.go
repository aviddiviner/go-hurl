package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func duration(r *reader.Reader) (ast.Duration, *ParseError) {
	value, err := natural(r)
	if err != nil {
		return ast.Duration{}, err
	}
	unit, err := durationUnit(r)
	if err != nil {
		return ast.Duration{}, err
	}
	return ast.Duration{Value: value, Unit: unit}, nil
}

func durationUnit(r *reader.Reader) (*ast.DurationUnit, *ParseError) {
	pos := r.Cursor().Pos
	s := r.ReadWhile(isASCIIAlphabetic)
	if s == "" {
		return nil, nil
	}
	unit, err := ast.ParseDurationUnit(s)
	if err != nil {
		return nil, newError(pos, false, ParseErrorInvalidDurationUnit{Value: s})
	}
	return &unit, nil
}

func isASCIIAlphabetic(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}
