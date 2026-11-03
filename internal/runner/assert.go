package runner

import (
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/reader"
)

// AssertResult is the result of an implicit or explicit assert:
// AssertResultImplicitVersion, AssertResultImplicitStatus,
// AssertResultImplicitHeader, AssertResultImplicitBody or
// AssertResultExplicit.
//
// For instance, in:
//
//	GET https://foo.com
//	HTTP 200
//	x-baz: bar
//	[Asserts]
//	header "toto" == "tutu"
//
// HTTP 200 and x-baz: bar are implicit asserts, the header query is an
// explicit one.
type AssertResult interface {
	assertResult()
	// ToRunnerError returns nil if the assert succeeded, or its error.
	ToRunnerError() *RunnerError
	// Line returns the source line of the assert.
	Line() int
}

// AssertResultImplicitVersion is an implicit HTTP version assert (HTTP/2...).
type AssertResultImplicitVersion struct {
	Actual, Expected string
	SourceInfo       ast.SourceInfo
}

// AssertResultImplicitStatus is an implicit HTTP status code assert.
type AssertResultImplicitStatus struct {
	Actual, Expected uint64
	SourceInfo       ast.SourceInfo
}

// AssertResultImplicitHeader is an implicit HTTP response header assert.
// ActualErr is set if the actual value can't be computed.
type AssertResultImplicitHeader struct {
	Actual     string
	ActualErr  *RunnerError
	Expected   string
	SourceInfo ast.SourceInfo
}

// AssertResultImplicitBody is an implicit HTTP response body assert.
type AssertResultImplicitBody struct {
	Actual      Value
	ActualErr   *RunnerError
	Expected    Value
	ExpectedErr *RunnerError
	SourceInfo  ast.SourceInfo
}

// AssertResultExplicit is an explicit assert. Actual is nil when the query
// returns no value. The predicate is only evaluated when ActualErr is nil;
// PredicateErr is its error.
type AssertResultExplicit struct {
	Actual       Value
	ActualErr    *RunnerError
	SourceInfo   ast.SourceInfo
	PredicateErr *RunnerError
}

func (AssertResultImplicitVersion) assertResult() {}
func (AssertResultImplicitStatus) assertResult()  {}
func (AssertResultImplicitHeader) assertResult()  {}
func (AssertResultImplicitBody) assertResult()    {}
func (AssertResultExplicit) assertResult()        {}

func (a AssertResultImplicitVersion) Line() int { return a.SourceInfo.Start.Line }
func (a AssertResultImplicitStatus) Line() int  { return a.SourceInfo.Start.Line }
func (a AssertResultImplicitHeader) Line() int  { return a.SourceInfo.Start.Line }
func (a AssertResultImplicitBody) Line() int    { return a.SourceInfo.Start.Line }
func (a AssertResultExplicit) Line() int        { return a.SourceInfo.Start.Line }

func (a AssertResultImplicitVersion) ToRunnerError() *RunnerError {
	if a.Expected == "HTTP" || a.Actual == a.Expected {
		return nil
	}
	return NewRunnerError(a.SourceInfo, RunnerErrorAssertVersion{Actual: a.Actual}, false)
}

func (a AssertResultImplicitStatus) ToRunnerError() *RunnerError {
	if a.Actual == a.Expected {
		return nil
	}
	return NewRunnerError(a.SourceInfo, RunnerErrorAssertStatus{Actual: strconv.FormatUint(a.Actual, 10)}, false)
}

func (a AssertResultImplicitHeader) ToRunnerError() *RunnerError {
	if a.ActualErr != nil {
		return a.ActualErr
	}
	if a.Actual == a.Expected {
		return nil
	}
	return NewRunnerError(a.SourceInfo, RunnerErrorAssertHeaderValueError{Actual: a.Actual}, false)
}

func (a AssertResultImplicitBody) ToRunnerError() *RunnerError {
	if a.ExpectedErr != nil {
		return a.ExpectedErr
	}
	if a.ActualErr != nil {
		return a.ActualErr
	}
	if Equal(a.Actual, a.Expected) {
		return nil
	}
	actual, expected := a.Actual.String(), a.Expected.String()
	if useDiff(a.Expected, a.Actual) {
		hunks := diff(expected, actual)
		line := a.SourceInfo.Start.Line + hunks[0].SourceLine
		kind := RunnerErrorAssertBodyDiffError{Hunks: hunks, BodySourceInfo: a.SourceInfo}
		pos := reader.Pos{Line: line, Column: 1}
		return NewRunnerError(ast.SourceInfo{Start: pos, End: pos}, kind, false)
	}
	return NewRunnerError(a.SourceInfo, RunnerErrorAssertBodyValueError{Actual: actual, Expected: expected}, false)
}

func (a AssertResultExplicit) ToRunnerError() *RunnerError {
	if a.ActualErr != nil {
		return a.ActualErr
	}
	return a.PredicateErr
}

func useDiff(expected, actual Value) bool {
	e, ok1 := expected.(ValueString)
	a, ok2 := actual.(ValueString)
	return ok1 && ok2 && (strings.Contains(e.Value, "\n") || strings.Contains(a.Value, "\n"))
}

// evalExplicitAssert evaluates an explicit assert against a list of HTTP
// responses. cache holds the parsed XML/JSON response bodies.
func evalExplicitAssert(assert ast.Assert, variables *VariableSet, responses []*httpclient.Response, cache *bodyCache, contextDir util.ContextDir, options QueryOptions) AssertResult {
	actual, err := evalQuery(assert.Query, variables, responses, cache, options)
	if err == nil && len(assert.Filters) > 0 {
		if actual == nil {
			err = NewRunnerError(assert.Filters[0].Filter.SourceInfo, RunnerErrorFilterMissingInput{}, true)
		} else {
			filterOptions := FilterOptions{UseJSONPathCoercion: options.UseJSONPathCoercion}
			actual, err = evalFilters(filters(assert.Filters), actual, variables, true, filterOptions)
		}
	}
	result := AssertResultExplicit{SourceInfo: assert.Predicate.PredicateFunc.SourceInfo}
	if err != nil {
		result.ActualErr = err
		return result
	}
	result.Actual = actual
	result.PredicateErr = evalPredicate(assert.Predicate, variables, actual, contextDir)
	return result
}

func filters(spaced []ast.SpacedFilter) []ast.Filter {
	fs := make([]ast.Filter, len(spaced))
	for i, f := range spaced {
		fs[i] = f.Filter
	}
	return fs
}
