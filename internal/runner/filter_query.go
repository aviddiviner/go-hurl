package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/jsonpath"
	"github.com/aviddiviner/go-hurl/internal/xpath"
)

// evalJSONPath evaluates a JSONPath expression against a string value
// parsed as JSON.
func evalJSONPath(value Value, expr ast.Template, variables *VariableSet, si ast.SourceInfo, assert, useCoercion bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		// For the moment, jsonpath only accepts strings (see Hurl issue #4249).
		return nil, invalidInputType(value, "string", si, assert)
	}
	doc, err := parseJSON([]byte(s.Value))
	if err != nil {
		return nil, invalidInputValue("value is not a valid JSON", si, false)
	}
	return evalJSONPathJSON(doc, expr, variables, useCoercion)
}

// evalJSONPathJSON evaluates a JSONPath expression against a JSON document.
// With coercion, no result is no value and a single result is that value;
// otherwise the result is always the list of matches.
func evalJSONPathJSON(doc any, expr ast.Template, variables *VariableSet, useCoercion bool) (Value, *RunnerError) {
	exprStr, err := evalTemplate(expr, variables)
	if err != nil {
		return nil, err
	}
	query, perr := jsonpath.Parse(exprStr)
	if perr != nil {
		kind := RunnerErrorQueryInvalidJSONPathExpression{Value: exprStr}
		return nil, NewRunnerError(expr.SourceInfo, kind, false)
	}
	results := query.Eval(doc)
	switch {
	case useCoercion && len(results) == 0:
		return nil, nil
	case useCoercion && len(results) == 1:
		return valueFromJSON(results[0]), nil
	}
	return valueFromJSON([]any(results)), nil
}

// evalXPath evaluates an XPath expression against a string value parsed as
// HTML (which also works with XML input).
func evalXPath(value Value, expr ast.Template, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	doc, err := xpath.Parse(s.Value, xpath.HTML)
	if err != nil {
		return nil, invalidInputValue("value is not a valid XML", si, false)
	}
	return evalXPathDoc(doc, expr, variables)
}

// evalXPathDoc evaluates an XPath expression against a document.
func evalXPathDoc(doc *xpath.Document, expr ast.Template, variables *VariableSet) (Value, *RunnerError) {
	exprStr, err := evalTemplate(expr, variables)
	if err != nil {
		return nil, err
	}
	v, xerr := xpathEval(doc, exprStr)
	switch xerr {
	case nil:
		return v, nil
	case xpath.ErrEval:
		return nil, NewRunnerError(expr.SourceInfo, RunnerErrorInvalidXPathEval{}, false)
	}
	panic("unsupported xpath " + expr.String())
}
