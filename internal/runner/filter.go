package runner

import "github.com/aviddiviner/go-hurl/ast"

// FilterOptions controls the evaluation of filters.
type FilterOptions struct {
	UseJSONPathCoercion bool
}

// DefaultFilterOptions returns the default filter options (JSONPath
// coercion enabled).
func DefaultFilterOptions() FilterOptions { return FilterOptions{UseJSONPathCoercion: true} }

// evalFilters applies successive filters to an input value, executed
// in an assert or not. A nil result is "no value".
func evalFilters(filters []ast.Filter, value Value, variables *VariableSet, inAssert bool, options FilterOptions) (Value, *RunnerError) {
	for _, filter := range filters {
		if value == nil {
			return nil, NewRunnerError(filter.SourceInfo, RunnerErrorFilterMissingInput{}, inAssert)
		}
		var err *RunnerError
		if value, err = evalFilter(filter, value, variables, inAssert, options); err != nil {
			return nil, err
		}
	}
	return value, nil
}

// evalFilter evaluates a filter with an input value.
func evalFilter(filter ast.Filter, value Value, variables *VariableSet, inAssert bool, options FilterOptions) (Value, *RunnerError) {
	si := filter.SourceInfo
	switch f := filter.Value.(type) {
	case ast.FilterBase64Decode:
		return evalBase64Decode(value, si, inAssert)
	case ast.FilterBase64Encode:
		return evalBase64Encode(value, si, inAssert)
	case ast.FilterBase64URLSafeDecode:
		return evalBase64URLSafeDecode(value, si, inAssert)
	case ast.FilterBase64URLSafeEncode:
		return evalBase64URLSafeEncode(value, si, inAssert)
	case ast.FilterCount:
		return evalCount(value, si, inAssert)
	case ast.FilterCharsetDecode:
		return evalCharsetDecode(value, f.Encoding, variables, si, inAssert)
	case ast.FilterDaysAfterNow:
		return evalDaysAfterNow(value, si, inAssert)
	case ast.FilterDaysBeforeNow:
		return evalDaysBeforeNow(value, si, inAssert)
	case ast.FilterDecode:
		return evalCharsetDecode(value, f.Encoding, variables, si, inAssert)
	case ast.FilterFirst:
		return evalFirst(value, si, inAssert)
	case ast.FilterFormat:
		return evalDateFormat(value, f.Fmt, variables, si, inAssert)
	case ast.FilterDateFormat:
		return evalDateFormat(value, f.Fmt, variables, si, inAssert)
	case ast.FilterHTMLEscape:
		return evalHTMLEscape(value, si, inAssert)
	case ast.FilterHTMLUnescape:
		return evalHTMLUnescape(value, si, inAssert)
	case ast.FilterJSONPath:
		return evalJSONPath(value, f.Expr, variables, si, inAssert, options.UseJSONPathCoercion)
	case ast.FilterLast:
		return evalLast(value, si, inAssert)
	case ast.FilterLocation:
		return evalLocation(value, si, inAssert)
	case ast.FilterRegex:
		return evalRegex(value, f.Value, variables, si, inAssert)
	case ast.FilterNth:
		return evalNth(value, f.N, variables, si, inAssert)
	case ast.FilterReplace:
		return evalReplace(value, variables, si, inAssert, f.OldValue, f.NewValue)
	case ast.FilterReplaceRegex:
		return evalReplaceRegex(value, variables, si, inAssert, f.Pattern, f.NewValue)
	case ast.FilterSplit:
		return evalSplit(value, variables, si, inAssert, f.Sep)
	case ast.FilterToDate:
		return evalToDate(value, f.Fmt, variables, si, inAssert)
	case ast.FilterToFloat:
		return evalToFloat(value, si, inAssert)
	case ast.FilterToHex:
		return evalToHex(value, si, inAssert)
	case ast.FilterToInt:
		return evalToInt(value, si, inAssert)
	case ast.FilterToString:
		return evalToString(value, si, inAssert)
	case ast.FilterURLDecode:
		return evalURLDecode(value, si, inAssert)
	case ast.FilterURLEncode:
		return evalURLEncode(value, si, inAssert)
	case ast.FilterURLQueryParam:
		return evalURLQueryParam(value, f.Param, variables, si, inAssert)
	case ast.FilterUTF8Decode:
		return evalUTF8Decode(value, si, inAssert)
	case ast.FilterUTF8Encode:
		return evalUTF8Encode(value, si, inAssert)
	case ast.FilterXPath:
		return evalXPath(value, f.Expr, variables, si, inAssert)
	}
	panic("unknown filter " + filter.Value.Identifier())
}

func invalidInputType(v Value, expected string, si ast.SourceInfo, assert bool) *RunnerError {
	kind := RunnerErrorFilterInvalidInputType{Actual: v.Kind().String(), Expected: expected}
	return NewRunnerError(si, kind, assert)
}

func invalidInputValue(reason string, si ast.SourceInfo, assert bool) *RunnerError {
	return NewRunnerError(si, RunnerErrorFilterInvalidInputValue{Value: reason}, assert)
}
