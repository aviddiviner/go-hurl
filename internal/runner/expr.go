package runner

import "github.com/aviddiviner/go-hurl/ast"

// evalExpr evaluates an expression (expr::eval).
func evalExpr(expr ast.Expr, variables *VariableSet) (Value, *RunnerError) {
	switch k := expr.Kind.(type) {
	case ast.ExprVariable:
		if v, ok := variables.Get(k.Value.Name); ok {
			return v.Value, nil
		}
		kind := RunnerErrorTemplateVariableNotDefined{Name: k.Value.Name}
		return nil, NewRunnerError(k.Value.SourceInfo, kind, false)
	case ast.ExprFunction:
		return evalFunction(k.Value)
	}
	panic("unknown expression kind")
}

// renderExpr renders an expression to a string (expr::render).
func renderExpr(expr ast.Expr, variables *VariableSet) (string, *RunnerError) {
	value, err := evalExpr(expr, variables)
	if err != nil {
		return "", err
	}
	if s, ok := Render(value); ok {
		return s, nil
	}
	kind := RunnerErrorUnrenderableExpression{Value: value.String()}
	return "", NewRunnerError(expr.SourceInfo, kind, false)
}
