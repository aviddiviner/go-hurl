package runner

import (
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
)

// evalMultiline renders a multiline string body given a set of variables.
// GraphQL bodies are turned into a JSON {"query": ..., "variables": ...}
// document.
func evalMultiline(multiline ast.MultilineString, variables *VariableSet) (string, *RunnerError) {
	switch k := multiline.Kind.(type) {
	case ast.MultilineStringText:
		return evalTemplate(k.Value, variables)
	case ast.MultilineStringJSON:
		return evalTemplate(k.Value, variables)
	case ast.MultilineStringXML:
		return evalTemplate(k.Value, variables)
	case ast.MultilineStringRaw:
		return k.Value.ToSource(), nil
	case ast.MultilineStringGraphQL:
		query, err := evalTemplate(k.Value.Value, variables)
		if err != nil {
			return "", err
		}
		query = jsonQuote(strings.TrimFunc(query, unicode.IsSpace))
		if k.Value.Variables == nil {
			return `{"query":` + query + `}`, nil
		}
		s, err := evalJSONValue(k.Value.Variables.Value, variables, false)
		if err != nil {
			return "", err
		}
		return `{"query":` + query + `,"variables":` + s + `}`, nil
	}
	panic("unknown multiline string kind")
}
