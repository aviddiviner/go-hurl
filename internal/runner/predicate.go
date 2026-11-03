package runner

import (
	"fmt"
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/reader"
)

// evalPredicate evaluates a predicate against an actual value (nil is no
// value). The expected value of the predicate can use variables.
func evalPredicate(predicate ast.Predicate, variables *VariableSet, value Value, contextDir util.ContextDir) *RunnerError {
	result, err := evalPredicateFunc(predicate.PredicateFunc, variables, value, contextDir)
	if err != nil {
		return err
	}

	// Column error is set to 0 to disable the error display of "^^^".
	line := predicate.Space0.SourceInfo.Start.Line
	si := ast.SourceInfo{Start: reader.Pos{Line: line, Column: 0}, End: reader.Pos{Line: line, Column: 0}}

	// `!=` already negates equality, so combine it with the `not` qualifier
	// (XOR) for the prefix.
	_, notEqual := predicate.PredicateFunc.Value.(ast.PredicateFuncNotEqual)
	not := ""
	if predicate.Not != notEqual {
		not = "not "
	}

	if result.typeMismatch || predicate.Not == result.success {
		kind := RunnerErrorAssertFailure{
			Actual:       result.actual,
			Expected:     not + result.expected,
			TypeMismatch: result.typeMismatch,
		}
		return NewRunnerError(si, kind, true)
	}
	return nil
}

type predicateResult struct {
	success      bool
	typeMismatch bool
	actual       string
	expected     string
}

// formatExpected is Value::format, the expected value of a predicate when
// there is no actual value.
func formatExpected(v Value) string {
	switch v := v.(type) {
	case ValueBool:
		return fmt.Sprintf("boolean <%t>", v.Value)
	case ValueBytes:
		s := ""
		if len(v.Value) > 1 {
			s = "s"
		}
		return fmt.Sprintf("%d byte%s", len(v.Value), s)
	case ValueDate:
		return "date <" + v.String() + ">"
	case ValueHTTPResponse:
		return "HTTP response <" + v.String() + ">"
	case ValueList:
		return "list of size " + strconv.Itoa(len(v.Value))
	case ValueNodeset:
		return "list of size " + strconv.Itoa(v.Value)
	case ValueNull:
		return "null"
	case ValueNumber:
		switch n := v.Value.(type) {
		case NumberFloat:
			return "float <" + formatFloat(n.Value) + ">"
		case NumberInteger:
			return "integer <" + n.String() + ">"
		case NumberBigInteger:
			return "number <" + n.Value + ">"
		}
	case ValueObject:
		return "list of size " + strconv.Itoa(len(v.Value))
	case ValueRegex:
		return "regex <" + v.String() + ">"
	case ValueString:
		return "string <" + v.Value + ">"
	case ValueUnit:
		return "something"
	}
	return ""
}

// expectedNoValue formats the expected value of a predicate when there is no
// actual value.
func expectedNoValue(pf ast.PredicateFuncValue, variables *VariableSet, contextDir util.ContextDir) (string, *RunnerError) {
	format := func(pv ast.PredicateValue, prefix, suffix string) (string, *RunnerError) {
		v, err := evalPredicateValue(pv, variables, contextDir)
		if err != nil {
			return "", err
		}
		return prefix + formatExpected(v) + suffix, nil
	}
	switch f := pf.(type) {
	case ast.PredicateFuncEqual:
		return format(f.Value, "", "")
	case ast.PredicateFuncNotEqual:
		return format(f.Value, "", "")
	case ast.PredicateFuncGreaterThan:
		return format(f.Value, "greater than <", ">")
	case ast.PredicateFuncGreaterThanOrEqual:
		return format(f.Value, "greater than or equals to <", ">")
	case ast.PredicateFuncLessThan:
		return format(f.Value, "less than <", ">")
	case ast.PredicateFuncLessThanOrEqual:
		return format(f.Value, "less than or equals to <", ">")
	case ast.PredicateFuncStartWith:
		return format(f.Value, "starts with ", "")
	case ast.PredicateFuncEndWith:
		return format(f.Value, "ends with ", "")
	case ast.PredicateFuncContain:
		return format(f.Value, "contains ", "")
	case ast.PredicateFuncInclude:
		return format(f.Value, "include ", "")
	case ast.PredicateFuncMatch:
		expected, err := evalPredicateValueTemplate(f.Value, variables)
		if err != nil {
			return "", err
		}
		return "matches regex <" + expected + ">", nil
	case ast.PredicateFuncExist:
		return "something", nil
	case ast.PredicateFuncIsBoolean:
		return "boolean", nil
	case ast.PredicateFuncIsCollection:
		return "collection", nil
	case ast.PredicateFuncIsDate:
		return "date", nil
	case ast.PredicateFuncIsEmpty:
		return "empty", nil
	case ast.PredicateFuncIsFloat:
		return "float", nil
	case ast.PredicateFuncIsInteger:
		return "integer", nil
	case ast.PredicateFuncIsIPv4:
		return "ipv4", nil
	case ast.PredicateFuncIsIPv6:
		return "ipv6", nil
	case ast.PredicateFuncIsISODate:
		return "date", nil
	case ast.PredicateFuncIsList:
		return "list", nil
	case ast.PredicateFuncIsNumber:
		return "number", nil
	case ast.PredicateFuncIsObject:
		return "object", nil
	case ast.PredicateFuncIsString:
		return "string", nil
	case ast.PredicateFuncIsUUID:
		return "uuid", nil
	}
	panic("unknown predicate")
}

// evalPredicateFunc evaluates a predicate function against an actual value
// (nil is no value).
func evalPredicateFunc(pf ast.PredicateFunc, variables *VariableSet, value Value, contextDir util.ContextDir) (predicateResult, *RunnerError) {
	if value == nil {
		expected, err := expectedNoValue(pf.Value, variables, contextDir)
		if err != nil {
			return predicateResult{}, err
		}
		return predicateResult{actual: "none", expected: expected}, nil
	}

	// withExpected evaluates the expected predicate value and applies f.
	withExpected := func(pv ast.PredicateValue, f func(actual, expected Value) predicateResult) (predicateResult, *RunnerError) {
		expected, err := evalPredicateValue(pv, variables, contextDir)
		if err != nil {
			return predicateResult{}, err
		}
		return f(value, expected), nil
	}

	switch f := pf.Value.(type) {
	case ast.PredicateFuncEqual:
		return withExpected(f.Value, assertValuesEqual)
	case ast.PredicateFuncNotEqual:
		return withExpected(f.Value, assertValuesNotEqual)
	case ast.PredicateFuncGreaterThan:
		return withExpected(f.Value, assertValuesGreater)
	case ast.PredicateFuncGreaterThanOrEqual:
		return withExpected(f.Value, assertValuesGreaterOrEqual)
	case ast.PredicateFuncLessThan:
		return withExpected(f.Value, assertValuesLess)
	case ast.PredicateFuncLessThanOrEqual:
		return withExpected(f.Value, assertValuesLessOrEqual)
	case ast.PredicateFuncStartWith:
		return withExpected(f.Value, func(actual, expected Value) predicateResult {
			return boolResult(actual, "starts with "+Repr(expected), valueStartsWith)(expected)
		})
	case ast.PredicateFuncEndWith:
		return withExpected(f.Value, func(actual, expected Value) predicateResult {
			return boolResult(actual, "ends with "+Repr(expected), valueEndsWith)(expected)
		})
	case ast.PredicateFuncContain:
		return withExpected(f.Value, func(actual, expected Value) predicateResult {
			return boolResult(actual, "contains "+Repr(expected), valueContains)(expected)
		})
	case ast.PredicateFuncInclude:
		return withExpected(f.Value, assertInclude)
	case ast.PredicateFuncMatch:
		return evalMatch(f.Value, pf.SourceInfo, variables, value, contextDir)
	case ast.PredicateFuncExist:
		return evalExist(value), nil
	case ast.PredicateFuncIsBoolean:
		return typeResult(value, valueIsBoolean(value), "boolean"), nil
	case ast.PredicateFuncIsCollection:
		return typeResult(value, valueIsCollection(value), "collection"), nil
	case ast.PredicateFuncIsDate:
		return evalIsDate(value), nil
	case ast.PredicateFuncIsEmpty:
		return evalIsEmpty(value), nil
	case ast.PredicateFuncIsFloat:
		return typeResult(value, valueIsFloat(value), "float"), nil
	case ast.PredicateFuncIsInteger:
		return evalIsInteger(value), nil
	case ast.PredicateFuncIsIPv4:
		return formatResult(value, valueIsIPv4, "string in IPv4 format"), nil
	case ast.PredicateFuncIsIPv6:
		return formatResult(value, valueIsIPv6, "string in IPv6 format"), nil
	case ast.PredicateFuncIsISODate:
		return evalIsISODate(value), nil
	case ast.PredicateFuncIsList:
		return typeResult(value, valueIsList(value), "list"), nil
	case ast.PredicateFuncIsNumber:
		return evalIsNumber(value), nil
	case ast.PredicateFuncIsObject:
		return typeResult(value, valueIsObject(value), "object"), nil
	case ast.PredicateFuncIsString:
		return typeResult(value, valueIsString(value), "string"), nil
	case ast.PredicateFuncIsUUID:
		return formatResult(value, valueIsUUID, "string in UUID format"), nil
	}
	panic("unknown predicate")
}

// boolResult returns the result of a predicate f(actual, expected) where an
// error is a type mismatch.
func boolResult(actual Value, expectedDisplay string, f func(v, other Value) (bool, error)) func(Value) predicateResult {
	return func(expected Value) predicateResult {
		success, err := f(actual, expected)
		return predicateResult{
			success:      err == nil && success,
			typeMismatch: err != nil,
			actual:       Repr(actual),
			expected:     expectedDisplay,
		}
	}
}

func typeResult(actual Value, success bool, expected string) predicateResult {
	return predicateResult{success: success, actual: Repr(actual), expected: expected}
}

// formatResult checks that a string value conforms to a format: actual is
// displayed raw, and a non-string value is a type mismatch.
func formatResult(actual Value, f func(Value) (bool, error), expected string) predicateResult {
	success, err := f(actual)
	if err != nil {
		return predicateResult{typeMismatch: true, actual: Repr(actual), expected: "string"}
	}
	return predicateResult{success: success, actual: actual.String(), expected: expected}
}

func evalMatch(pv ast.PredicateValue, si ast.SourceInfo, variables *VariableSet, actual Value, contextDir util.ContextDir) (predicateResult, *RunnerError) {
	expected, err := evalPredicateValue(pv, variables, contextDir)
	if err != nil {
		return predicateResult{}, err
	}
	result := predicateResult{actual: Repr(actual), expected: "matches regex <" + expected.String() + ">"}
	success, merr := valueIsMatch(actual, expected)
	switch merr {
	case nil:
		result.success = success
	case EvalErrorInvalidRegex:
		return predicateResult{}, NewRunnerError(si, RunnerErrorInvalidRegex{}, false)
	default:
		result.typeMismatch = true
	}
	return result, nil
}

func evalIsInteger(actual Value) predicateResult {
	return typeResult(actual, valueIsInteger(actual), "integer")
}

func evalIsNumber(actual Value) predicateResult {
	return typeResult(actual, valueIsNumber(actual), "number")
}

func evalIsDate(actual Value) predicateResult {
	return typeResult(actual, valueIsDate(actual), "date")
}

// evalIsISODate checks if actual is a string representing a RFC 3339 date
// (format YYYY-MM-DDTHH:mm:ss.sssZ).
func evalIsISODate(actual Value) predicateResult {
	return formatResult(actual, valueIsISODate, "string with format YYYY-MM-DDTHH:mm:ss.sssZ")
}

func evalExist(actual Value) predicateResult {
	n, ok := actual.(ValueNodeset)
	return predicateResult{success: !ok || n.Value != 0, actual: Repr(actual), expected: "something"}
}

func evalIsEmpty(actual Value) predicateResult {
	expected := "count equals to 0"
	count, err := valueCount(actual)
	if err != nil {
		return predicateResult{typeMismatch: true, actual: Repr(actual), expected: expected}
	}
	return predicateResult{success: count == 0, actual: "count equals to " + strconv.Itoa(count), expected: expected}
}

func assertValuesEqual(actual, expected Value) predicateResult {
	return predicateResult{success: Equal(actual, expected), actual: Repr(actual), expected: Repr(expected)}
}

func assertValuesNotEqual(actual, expected Value) predicateResult {
	return predicateResult{success: !Equal(actual, expected), actual: Repr(actual), expected: Repr(expected)}
}

func compareResult(actual, expected Value, prefix string, ok func(int) bool) predicateResult {
	result := predicateResult{actual: Repr(actual), expected: prefix + Repr(expected)}
	c, err := valueCompare(actual, expected)
	if err != nil {
		result.typeMismatch = true
	} else {
		result.success = ok(c)
	}
	return result
}

func assertValuesGreater(actual, expected Value) predicateResult {
	return compareResult(actual, expected, "greater than ", func(c int) bool { return c > 0 })
}

func assertValuesGreaterOrEqual(actual, expected Value) predicateResult {
	return compareResult(actual, expected, "greater or equal than ", func(c int) bool { return c >= 0 })
}

func assertValuesLess(actual, expected Value) predicateResult {
	return compareResult(actual, expected, "less than ", func(c int) bool { return c < 0 })
}

func assertValuesLessOrEqual(actual, expected Value) predicateResult {
	return compareResult(actual, expected, "less or equal than ", func(c int) bool { return c <= 0 })
}

func assertInclude(value, element Value) predicateResult {
	return boolResult(value, "includes "+Repr(element), valueIncludes)(element)
}
