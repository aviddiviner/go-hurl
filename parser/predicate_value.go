package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// asPredicateValue adapts a parser of T into a parser of predicate values.
func asPredicateValue[T any](f func(*reader.Reader) (T, *ParseError), wrap func(T) ast.PredicateValue) func(*reader.Reader) (ast.PredicateValue, *ParseError) {
	return func(r *reader.Reader) (ast.PredicateValue, *ParseError) {
		v, err := f(r)
		if err != nil {
			return nil, err
		}
		return wrap(v), nil
	}
}

var predicateValueParsers []func(*reader.Reader) (ast.PredicateValue, *ParseError)

func init() {
	predicateValueParsers = []func(*reader.Reader) (ast.PredicateValue, *ParseError){
		func(r *reader.Reader) (ast.PredicateValue, *ParseError) {
			if err := null(r); err != nil {
				return nil, err
			}
			return ast.PredicateNull{}, nil
		},
		asPredicateValue(boolean, func(v bool) ast.PredicateValue { return ast.PredicateBool{Value: v} }),
		asPredicateValue(number, func(v ast.Number) ast.PredicateValue { return ast.PredicateNumber{Value: v} }),
		asPredicateValue(file, func(v ast.File) ast.PredicateValue { return ast.PredicateFile{Value: v} }),
		asPredicateValue(hex, func(v ast.Hex) ast.PredicateValue { return ast.PredicateHex{Value: v} }),
		asPredicateValue(base64, func(v ast.Base64) ast.PredicateValue { return ast.PredicateBase64{Value: v} }),
		asPredicateValue(placeholder, func(v ast.Placeholder) ast.PredicateValue { return ast.PredicatePlaceholder{Value: v} }),
		asPredicateValue(quotedTemplate, func(v ast.Template) ast.PredicateValue { return ast.PredicateString{Value: v} }),
		asPredicateValue(multilineString, func(v ast.MultilineString) ast.PredicateValue {
			return ast.PredicateMultilineString{Value: v}
		}),
		asPredicateValue(backtickTemplate, func(v ast.Template) ast.PredicateValue { return ast.PredicateString{Value: v} }),
		asPredicateValue(regex, func(v ast.Regex) ast.PredicateValue { return ast.PredicateRegex{Value: v} }),
	}
}

func predicateValue(r *reader.Reader) (ast.PredicateValue, *ParseError) {
	v, err := choice(predicateValueParsers, r)
	if err != nil {
		kind := err.Kind
		if err.Recoverable {
			kind = ParseErrorPredicateValue{}
		}
		return nil, newError(err.Pos, false, kind)
	}
	return v, nil
}
