package jsonpath

import (
	"regexp"

	"github.com/aviddiviner/go-hurl/reader"
)

const maxNestingDepth = 64

// logicalOrExpr parses a logical expression, sharing depth with nested filters and functions.
func logicalOrExpr(r *reader.Reader, depth int) (logicalExpr, *ParseError) {
	if err := checkDepth(r, depth); err != nil {
		return nil, err
	}
	first, err := logicalAndExpr(r, depth)
	if err != nil {
		return nil, err
	}
	operands := []logicalExpr{first}
	for {
		skipWhitespace(r)
		if !matchStr("||", r) {
			break
		}
		skipWhitespace(r)
		operand, err := logicalOrExpr(r, depth+1)
		if err != nil {
			return nil, err
		}
		operands = append(operands, operand)
	}
	if len(operands) == 1 {
		return first, nil
	}
	return orExpr{operands: operands}, nil
}

func checkDepth(r *reader.Reader, depth int) *ParseError {
	if depth >= maxNestingDepth {
		return parseError(r.Cursor().Pos, MaxDepthExceeded{MaxDepth: maxNestingDepth})
	}
	return nil
}

func logicalAndExpr(r *reader.Reader, depth int) (logicalExpr, *ParseError) {
	first, err := basicExpr(r, depth)
	if err != nil {
		return nil, err
	}
	operands := []logicalExpr{first}
	for {
		skipWhitespace(r)
		if !matchStr("&&", r) {
			break
		}
		skipWhitespace(r)
		operand, err := basicExpr(r, depth)
		if err != nil {
			return nil, err
		}
		operands = append(operands, operand)
	}
	if len(operands) == 1 {
		return first, nil
	}
	return andExpr{operands: operands}, nil
}

func basicExpr(r *reader.Reader, depth int) (logicalExpr, *ParseError) {
	save := r.Cursor()
	if e, err := tryParenExpr(r, depth); err != nil {
		return nil, err
	} else if e != nil {
		return e, nil
	}
	if e, err := tryComparison(r, depth); err != nil {
		return nil, err
	} else if e != nil {
		return *e, nil
	}
	if e, err := tryTestExpr(r, depth); err != nil {
		return nil, err
	} else if e != nil {
		return *e, nil
	}
	return nil, parseError(save.Pos, Expecting("basic expression"))
}

func tryParenExpr(r *reader.Reader, depth int) (logicalExpr, *ParseError) {
	save := r.Cursor()
	not := matchStr("!", r)
	skipWhitespace(r)
	if !matchStr("(", r) {
		r.Seek(save)
		return nil, nil
	}
	expr, err := logicalOrExpr(r, depth+1)
	if err != nil {
		return nil, err
	}
	if !matchStr(")", r) {
		return nil, parseError(r.Cursor().Pos, Expecting("')'"))
	}
	if not {
		return notExpr{expr: expr}, nil
	}
	return expr, nil
}

func tryTestExpr(r *reader.Reader, depth int) (*testExpr, *ParseError) {
	not := matchStr("!", r)
	skipWhitespace(r)
	if q, err := tryFilterQuery(r, depth+1); err != nil {
		return nil, err
	} else if q != nil {
		return &testExpr{not: not, kind: testFilterQuery{query: q}}, nil
	}
	if f, err := tryLogicalTypeFunction(r, depth); err != nil {
		return nil, err
	} else if f != nil {
		return &testExpr{not: not, kind: testFunction{function: f}}, nil
	}
	return nil, nil
}

func tryComparison(r *reader.Reader, depth int) (*comparisonExpr, *ParseError) {
	save := r.Cursor()
	left, err := tryComparable(r, depth)
	if err != nil || left == nil {
		return nil, err
	}
	skipWhitespace(r)
	op, ok := tryComparisonOp(r)
	if !ok {
		r.Seek(save)
		return nil, nil
	}
	skipWhitespace(r)
	right, err := parseComparable(r, depth)
	if err != nil {
		return nil, err
	}
	return &comparisonExpr{left: left, right: right, operator: op}, nil
}

func parseComparable(r *reader.Reader, depth int) (valueType, *ParseError) {
	v, err := tryComparable(r, depth)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, parseError(r.Cursor().Pos, Expecting("comparable"))
	}
	return v, nil
}

func tryComparable(r *reader.Reader, depth int) (valueType, *ParseError) {
	if l, err := tryLiteral(r); err != nil {
		return nil, err
	} else if l != nil {
		return *l, nil
	}
	if q, err := trySingularQuery(r); err != nil {
		return nil, err
	} else if q != nil {
		return q, nil
	}
	if f, err := tryValueTypeFunction(r, depth); err != nil {
		return nil, err
	} else if f != nil {
		return f, nil
	}
	return nil, nil
}

func tryComparisonOp(r *reader.Reader) (comparisonOp, bool) {
	for _, op := range []struct {
		s  string
		op comparisonOp
	}{
		{"==", opEqual},
		{"!=", opNotEqual},
		{"<=", opLessOrEqual},
		{"<", opLess},
		{">=", opGreaterOrEqual},
		{">", opGreater},
	} {
		if matchStr(op.s, r) {
			return op.op, true
		}
	}
	return 0, false
}

func trySingularQuery(r *reader.Reader) (singularQuery, *ParseError) {
	switch {
	case matchStr("$", r):
		segments, err := singularQuerySegments(r)
		if err != nil {
			return nil, err
		}
		return absoluteSingularQuery{segments: segments}, nil
	case matchStr("@", r):
		segments, err := singularQuerySegments(r)
		if err != nil {
			return nil, err
		}
		return relativeSingularQuery{segments: segments}, nil
	}
	return nil, nil
}

func singularQuerySegments(r *reader.Reader) ([]singularQuerySegment, *ParseError) {
	var segments []singularQuerySegment
	skipWhitespace(r)
	for {
		s, err := trySingularQuerySegment(r)
		if err != nil {
			return nil, err
		}
		if s == nil {
			return segments, nil
		}
		segments = append(segments, s)
		skipWhitespace(r)
	}
}

func trySingularQuerySegment(r *reader.Reader) (singularQuerySegment, *ParseError) {
	save := r.Cursor()
	if matchStr("[", r) {
		if name, err := tryNameSelector(r); err != nil {
			return nil, err
		} else if name != nil {
			if err := expectStr("]", r); err != nil {
				return nil, err
			}
			return *name, nil
		}
		v, ok, err := tryInteger(r)
		if err != nil || !ok {
			// As in Hurl, the '[' is not given back when nothing follows.
			return nil, err
		}
		if err := expectStr("]", r); err != nil {
			return nil, err
		}
		return indexSelector{value: v}, nil
	}
	if matchStr(".", r) {
		name, err := memberNameShorthand(r)
		if err != nil {
			r.Seek(save)
			return nil, nil
		}
		return nameSelector{value: name}, nil
	}
	return nil, nil
}

// Functions

func tryValueTypeFunction(r *reader.Reader, depth int) (valueTypeFunction, *ParseError) {
	var name string
	for _, n := range []string{"length", "value", "count"} {
		if matchStr(n, r) {
			name = n
			break
		}
	}
	if name == "" {
		return nil, nil
	}
	if err := checkDepth(r, depth); err != nil {
		return nil, err
	}
	if err := expectStr("(", r); err != nil {
		return nil, err
	}
	skipWhitespace(r)
	var f valueTypeFunction
	if name == "length" {
		arg, err := parseValueTypeArgument(r, depth+1)
		if err != nil {
			return nil, err
		}
		f = lengthFunction{arg: arg}
	} else {
		arg, err := parseNodesTypeArgument(r, depth+1)
		if err != nil {
			return nil, err
		}
		if name == "value" {
			f = valueFunction{arg: arg}
		} else {
			f = countFunction{arg: arg}
		}
	}
	skipWhitespace(r)
	if err := expectStr(")", r); err != nil {
		return nil, err
	}
	return f, nil
}

func tryLogicalTypeFunction(r *reader.Reader, depth int) (logicalTypeFunction, *ParseError) {
	var name string
	for _, n := range []string{"match", "search"} {
		if matchStr(n, r) {
			name = n
			break
		}
	}
	if name == "" {
		return nil, nil
	}
	if err := checkDepth(r, depth); err != nil {
		return nil, err
	}
	if err := expectStr("(", r); err != nil {
		return nil, err
	}
	skipWhitespace(r)
	arg, err := parseValueTypeArgument(r, depth+1)
	if err != nil {
		return nil, err
	}
	skipWhitespace(r)
	if err := expectStr(",", r); err != nil {
		return nil, err
	}
	skipWhitespace(r)
	pattern, err := parseRegexValueTypeArgument(r, depth+1)
	if err != nil {
		return nil, err
	}
	skipWhitespace(r)
	if err := expectStr(")", r); err != nil {
		return nil, err
	}
	if name == "match" {
		return matchFunction{arg: arg, pattern: pattern}, nil
	}
	return searchFunction{arg: arg, pattern: pattern}, nil
}

func parseValueTypeArgument(r *reader.Reader, depth int) (valueType, *ParseError) {
	if l, err := tryLiteral(r); err != nil {
		return nil, err
	} else if l != nil {
		return *l, nil
	}
	if q, err := trySingularQuery(r); err != nil {
		return nil, err
	} else if q != nil {
		return q, nil
	}
	if f, err := tryValueTypeFunction(r, depth); err != nil {
		return nil, err
	} else if f != nil {
		return f, nil
	}
	return nil, parseError(r.Cursor().Pos, Expecting("a ValueType argument"))
}

func parseRegexValueTypeArgument(r *reader.Reader, depth int) (regexArgument, *ParseError) {
	if re, err := tryRegex(r); err != nil {
		return nil, err
	} else if re != nil {
		return regexLiteral{re: re}, nil
	}
	if q, err := trySingularQuery(r); err != nil {
		return nil, err
	} else if q != nil {
		return q, nil
	}
	if f, err := tryValueTypeFunction(r, depth); err != nil {
		return nil, err
	} else if f != nil {
		return f, nil
	}
	return nil, parseError(r.Cursor().Pos, Expecting("a RegexValueType argument"))
}

func tryRegex(r *reader.Reader) (*regexp.Regexp, *ParseError) {
	pos := r.Cursor().Pos
	s, ok, err := tryStringLiteral(r)
	if err != nil || !ok {
		return nil, err
	}
	re, cerr := regexp.Compile(s)
	if cerr != nil {
		return nil, parseError(pos, Expecting("a valid regex"))
	}
	return re, nil
}

func parseNodesTypeArgument(r *reader.Reader, depth int) (filterQuery, *ParseError) {
	q, err := tryFilterQuery(r, depth)
	if err != nil {
		return nil, err
	}
	if q == nil {
		return nil, parseError(r.Cursor().Pos, Expecting("a NodesType argument"))
	}
	return q, nil
}
