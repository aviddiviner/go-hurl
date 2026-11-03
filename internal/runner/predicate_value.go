package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func evalPredicateValue(pv ast.PredicateValue, variables *VariableSet, contextDir util.ContextDir) (Value, *RunnerError) {
	switch v := pv.(type) {
	case ast.PredicateString:
		s, err := evalTemplate(v.Value, variables)
		if err != nil {
			return nil, err
		}
		return NewString(s), nil
	case ast.PredicateMultilineString:
		s, err := evalMultiline(v.Value, variables)
		if err != nil {
			return nil, err
		}
		return NewString(s), nil
	case ast.PredicateBool:
		return NewBool(v.Value), nil
	case ast.PredicateNull:
		return ValueNull{}, nil
	case ast.PredicateNumber:
		return ValueNumber{evalASTNumber(v.Value)}, nil
	case ast.PredicateFile:
		b, err := evalFile(v.Value.Filename, variables, contextDir)
		if err != nil {
			return nil, err
		}
		return ValueBytes{b}, nil
	case ast.PredicateHex:
		return ValueBytes{v.Value.Value}, nil
	case ast.PredicateBase64:
		return ValueBytes{v.Value.Value}, nil
	case ast.PredicatePlaceholder:
		return evalExpr(v.Value.Expr, variables)
	case ast.PredicateRegex:
		return ValueRegex{v.Value.Inner}, nil
	}
	panic("unknown predicate value")
}

func evalPredicateValueTemplate(pv ast.PredicateValue, variables *VariableSet) (string, *RunnerError) {
	switch v := pv.(type) {
	case ast.PredicateString:
		return evalTemplate(v.Value, variables)
	case ast.PredicateRegex:
		return v.Value.Inner.String(), nil
	}
	panic("expect a string or a regex predicate value")
}

func evalASTNumber(n ast.Number) Number {
	switch n := n.(type) {
	case ast.NumberFloat:
		return NumberFloat{n.Value.Value}
	case ast.NumberInteger:
		return NumberInteger{n.Value.Value}
	case ast.NumberBigInteger:
		return NumberBigInteger{n.Value}
	}
	panic("unknown number")
}
