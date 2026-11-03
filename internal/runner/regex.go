package runner

import (
	"regexp"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/regex"
)

// evalRegexValue evaluates a regex given as a template or as a literal.
func evalRegexValue(regexValue ast.RegexValue, variables *VariableSet) (*regexp.Regexp, *RunnerError) {
	switch r := regexValue.(type) {
	case ast.RegexTemplate:
		value, err := evalTemplate(r.Value, variables)
		if err != nil {
			return nil, err
		}
		re, cerr := regex.Compile(value)
		if cerr != nil {
			return nil, NewRunnerError(r.Value.SourceInfo, RunnerErrorInvalidRegex{}, false)
		}
		return re, nil
	case ast.RegexRegex:
		return r.Value.Inner, nil
	}
	panic("unknown regex value")
}
