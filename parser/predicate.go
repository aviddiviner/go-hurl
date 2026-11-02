package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func predicate(r *reader.Reader) (ast.Predicate, *ParseError) {
	not, space0 := predicateNot(r)
	f, err := predicateFunc(r)
	if err != nil {
		return ast.Predicate{}, err
	}
	return ast.Predicate{Not: not, Space0: space0, PredicateFunc: f}, nil
}

// predicateNot can not fail.
func predicateNot(r *reader.Reader) (bool, ast.Whitespace) {
	save := r.Cursor()
	noWhitespace := ast.Whitespace{SourceInfo: sourceInfo(save.Pos, save.Pos)}
	if tryLiteral("not", r) != nil {
		return false, noWhitespace
	}
	space, err := oneOrMoreSpaces(r)
	if err != nil {
		r.Seek(save)
		return false, noWhitespace
	}
	return true, space
}

func predicateFunc(r *reader.Reader) (ast.PredicateFunc, *ParseError) {
	start := r.Cursor()
	value, err := predicateFuncValue(r)
	if err != nil {
		return ast.PredicateFunc{}, err
	}
	return ast.PredicateFunc{SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos), Value: value}, nil
}

type predicateFuncParser = func(*reader.Reader) (ast.PredicateFuncValue, *ParseError)

var predicateFuncParsers []predicateFuncParser

func init() {
	predicateFuncParsers = []predicateFuncParser{
		equalPredicate,
		notEqualPredicate,
		greaterOrEqualPredicate,
		greaterPredicate,
		lessOrEqualPredicate,
		lessPredicate,
		startWithPredicate,
		endWithPredicate,
		containPredicate,
		includePredicate,
		matchPredicate,
		keywordPredicate("isInteger", ast.PredicateFuncIsInteger{}),
		keywordPredicate("isFloat", ast.PredicateFuncIsFloat{}),
		keywordPredicate("isBoolean", ast.PredicateFuncIsBoolean{}),
		keywordPredicate("isString", ast.PredicateFuncIsString{}),
		keywordPredicate("isCollection", ast.PredicateFuncIsCollection{}),
		keywordPredicate("isList", ast.PredicateFuncIsList{}),
		keywordPredicate("isObject", ast.PredicateFuncIsObject{}),
		datePredicate,
		keywordPredicate("isIsoDate", ast.PredicateFuncIsISODate{}),
		keywordPredicate("exists", ast.PredicateFuncExist{}),
		keywordPredicate("isEmpty", ast.PredicateFuncIsEmpty{}),
		keywordPredicate("isNumber", ast.PredicateFuncIsNumber{}),
		keywordPredicate("isIpv4", ast.PredicateFuncIsIPv4{}),
		keywordPredicate("isIpv6", ast.PredicateFuncIsIPv6{}),
		keywordPredicate("isUuid", ast.PredicateFuncIsUUID{}),
	}
}

var (
	equalPredicate = valuePredicate("==", zeroOrMoreSpaces, nil, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncEqual{Space0: s, Value: v}
	})
	notEqualPredicate = valuePredicate("!=", zeroOrMoreSpaces, nil, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncNotEqual{Space0: s, Value: v}
	})
	greaterOrEqualPredicate = valuePredicate(">=", zeroOrMoreSpaces, isComparable, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncGreaterThanOrEqual{Space0: s, Value: v}
	})
	greaterPredicate = valuePredicate(">", zeroOrMoreSpaces, isComparable, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncGreaterThan{Space0: s, Value: v}
	})
	lessOrEqualPredicate = valuePredicate("<=", zeroOrMoreSpaces, isComparable, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncLessThanOrEqual{Space0: s, Value: v}
	})
	lessPredicate = valuePredicate("<", zeroOrMoreSpaces, isComparable, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncLessThan{Space0: s, Value: v}
	})
	startWithPredicate = valuePredicate("startsWith", oneOrMoreSpaces, isStringOrBytes, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncStartWith{Space0: s, Value: v}
	})
	endWithPredicate = valuePredicate("endsWith", oneOrMoreSpaces, isStringOrBytes, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncEndWith{Space0: s, Value: v}
	})
	containPredicate = valuePredicate("contains", oneOrMoreSpaces, nil, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncContain{Space0: s, Value: v}
	})
	includePredicate = valuePredicate("includes", oneOrMoreSpaces, nil, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncInclude{Space0: s, Value: v}
	})
	matchPredicate = valuePredicate("matches", oneOrMoreSpaces, isStringOrRegex, func(s ast.Whitespace, v ast.PredicateValue) ast.PredicateFuncValue {
		return ast.PredicateFuncMatch{Space0: s, Value: v}
	})
	datePredicate = keywordPredicate("isDate", ast.PredicateFuncIsDate{})
)

func predicateFuncValue(r *reader.Reader) (ast.PredicateFuncValue, *ParseError) {
	start := r.Cursor()
	v, err := choice(predicateFuncParsers, r)
	if err != nil && err.Recoverable {
		return nil, newError(start.Pos, false, ParseErrorPredicate{})
	}
	return v, err
}

func isComparable(v ast.PredicateValue) bool {
	switch v.(type) {
	case ast.PredicateNumber, ast.PredicateString, ast.PredicatePlaceholder:
		return true
	}
	return false
}

func isStringOrBytes(v ast.PredicateValue) bool {
	switch v.(type) {
	case ast.PredicateString, ast.PredicateHex, ast.PredicateBase64:
		return true
	}
	return false
}

func isStringOrRegex(v ast.PredicateValue) bool {
	switch v.(type) {
	case ast.PredicateString, ast.PredicateRegex:
		return true
	}
	return false
}

// valuePredicate parses an operator followed by spaces and a predicate
// value. If valid is set, a value failing it is a PredicateValue error.
func valuePredicate(
	operator string,
	spaces func(*reader.Reader) (ast.Whitespace, *ParseError),
	valid func(ast.PredicateValue) bool,
	build func(ast.Whitespace, ast.PredicateValue) ast.PredicateFuncValue,
) predicateFuncParser {
	return func(r *reader.Reader) (ast.PredicateFuncValue, *ParseError) {
		if err := tryLiteral(operator, r); err != nil {
			return nil, err
		}
		space0, err := spaces(r)
		if err != nil {
			return nil, err
		}
		start := r.Cursor()
		value, err := predicateValue(r)
		if err != nil {
			return nil, err
		}
		if valid != nil && !valid(value) {
			return nil, newError(start.Pos, false, ParseErrorPredicateValue{})
		}
		return build(space0, value), nil
	}
}

func keywordPredicate(name string, value ast.PredicateFuncValue) predicateFuncParser {
	return func(r *reader.Reader) (ast.PredicateFuncValue, *ParseError) {
		if err := tryLiteral(name, r); err != nil {
			return nil, err
		}
		return value, nil
	}
}
