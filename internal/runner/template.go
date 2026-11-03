package runner

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
)

// evalTemplate renders a template to a string given a set of variables.
func evalTemplate(template ast.Template, variables *VariableSet) (string, *RunnerError) {
	var b strings.Builder
	for _, e := range template.Elements {
		s, err := evalTemplateElement(e, variables)
		if err != nil {
			return "", err
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

func evalTemplateElement(e ast.TemplateElement, variables *VariableSet) (string, *RunnerError) {
	switch e := e.(type) {
	case ast.TemplateString:
		return e.Value, nil
	case ast.TemplatePlaceholder:
		return renderExpr(e.Value.Expr, variables)
	}
	panic("unknown template element")
}
