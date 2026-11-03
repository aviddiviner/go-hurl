package runner

import (
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
)

// evalCount counts the items of a list, bytes or node-set.
func evalCount(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	switch v := value.(type) {
	case ValueList:
		return NewInteger(int64(len(v.Value))), nil
	case ValueBytes:
		return NewInteger(int64(len(v.Value))), nil
	case ValueNodeset:
		return NewInteger(int64(v.Value)), nil
	}
	return nil, invalidInputType(value, "list, bytes or nodeset", si, assert)
}

// evalFirst returns the first item of a list.
func evalFirst(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	l, ok := value.(ValueList)
	if !ok {
		return nil, invalidInputType(value, "list", si, assert)
	}
	if len(l.Value) == 0 {
		return nil, invalidInputValue("list is empty", si, assert)
	}
	return l.Value[0], nil
}

// evalLast returns the last item of a list.
func evalLast(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	l, ok := value.(ValueList)
	if !ok {
		return nil, invalidInputType(value, "list", si, assert)
	}
	if len(l.Value) == 0 {
		return nil, invalidInputValue("list is empty", si, assert)
	}
	return l.Value[len(l.Value)-1], nil
}

// evalNth returns the item of a list at a zero-based index (negative
// indices count from the end).
func evalNth(value Value, n ast.IntegerValue, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	index, err := evalIntegerValue(n, variables)
	if err != nil {
		return nil, err
	}
	l, ok := value.(ValueList)
	if !ok {
		return nil, invalidInputType(value, "list", si, assert)
	}
	v, reason, ok := tryNth(l.Value, index)
	if !ok {
		return nil, invalidInputValue(reason, si, assert)
	}
	return v, nil
}

func tryNth[T any](items []T, index int64) (T, string, bool) {
	n := int64(len(items))
	switch {
	case index >= 0 && index < n:
		return items[index], "", true
	case index < 0 && n+index >= 0:
		return items[n+index], "", true
	}
	var zero T
	return zero, "out of bound - size is " + strconv.FormatInt(n, 10), false
}

// evalIntegerValue evaluates an integer literal or placeholder.
func evalIntegerValue(n ast.IntegerValue, variables *VariableSet) (int64, *RunnerError) {
	switch n := n.(type) {
	case ast.IntegerLiteral:
		return n.Value.Value, nil
	case ast.IntegerPlaceholder:
		v, err := evalExpr(n.Value.Expr, variables)
		if err != nil {
			return 0, err
		}
		if num, ok := v.(ValueNumber); ok {
			if i, ok := num.Value.(NumberInteger); ok {
				return i.Value, nil
			}
		}
		kind := RunnerErrorExpressionInvalidType{Value: Repr(v), Expecting: "integer"}
		return 0, NewRunnerError(n.Value.Expr.SourceInfo, kind, false)
	}
	panic("unknown integer value")
}
