package jsonpath

import (
	"encoding/json"
	"errors"
	"maps"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Queries and segments

func evalSegments(segments []segment, current, root any) NodeList {
	results := NodeList{current}
	for _, s := range segments {
		var next NodeList
		for _, v := range results {
			next = append(next, s.eval(v, root)...)
		}
		results = next
	}
	return results
}

func (q *Query) evalFilter(_, root any) NodeList { return q.Eval(root) }

func (q relativeQuery) evalFilter(current, root any) NodeList {
	return evalSegments(q.segments, current, root)
}

func (s childSegment) eval(current, root any) NodeList {
	var results NodeList
	for _, sel := range s.selectors {
		results = append(results, sel.eval(current, root)...)
	}
	return results
}

func (s descendantSegment) eval(current, root any) NodeList {
	var results NodeList
	for _, d := range descendants(current) {
		for _, sel := range s.selectors {
			results = append(results, sel.eval(d, root)...)
		}
	}
	return results
}

func descendants(node any) NodeList {
	nodes := NodeList{node}
	switch v := node.(type) {
	case map[string]any:
		for _, child := range sortedValues(v) {
			nodes = append(nodes, descendants(child)...)
		}
	case []any:
		for _, child := range v {
			nodes = append(nodes, descendants(child)...)
		}
	}
	return nodes
}

// sortedValues returns the member values of an object in key order, like
// iterating a serde_json map without preserve_order.
func sortedValues(m map[string]any) []any {
	values := make([]any, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		values = append(values, m[k])
	}
	return values
}

// Selectors

func (s nameSelector) eval(current, _ any) NodeList { return optionalNode(s.evalOne(current)) }

func (s nameSelector) evalOne(v any) (any, bool) {
	if m, ok := v.(map[string]any); ok {
		value, ok := m[s.value]
		return value, ok
	}
	return nil, false
}

func (wildcardSelector) eval(current, _ any) NodeList {
	switch v := current.(type) {
	case map[string]any:
		return sortedValues(v)
	case []any:
		return slices.Clone(v)
	}
	return nil
}

func (s indexSelector) eval(current, _ any) NodeList { return optionalNode(s.evalOne(current)) }

func (s indexSelector) evalOne(v any) (any, bool) {
	values, ok := v.([]any)
	if !ok {
		return nil, false
	}
	n := int64(len(values))
	if s.value < -n || s.value >= n {
		return nil, false
	}
	i := s.value
	if i < 0 {
		i += n
	}
	return values[i], true
}

func (s arraySliceSelector) eval(current, _ any) NodeList {
	values, ok := current.([]any)
	if !ok || s.step == 0 {
		return nil
	}
	lower, upper := s.bounds(int64(len(values)))
	var results NodeList
	if s.step > 0 {
		for i := lower; i < upper; i += s.step {
			results = append(results, values[i])
		}
	} else {
		for i := upper; lower < i; i += s.step {
			results = append(results, values[i])
		}
	}
	return results
}

func (s arraySliceSelector) startIndex(n int64) int64 {
	switch {
	case s.start != nil:
		return *s.start
	case s.step >= 0:
		return 0
	}
	return n - 1
}

func (s arraySliceSelector) endIndex(n int64) int64 {
	switch {
	case s.end != nil:
		return *s.end
	case s.step >= 0:
		return n
	}
	return -n - 1
}

func (s arraySliceSelector) bounds(n int64) (int64, int64) {
	start := normalizeIndex(s.startIndex(n), n)
	end := normalizeIndex(s.endIndex(n), n)
	if s.step > 0 {
		return min(max(start, 0), n), min(max(end, 0), n)
	}
	return min(max(end, -1), n-1), min(max(start, -1), n-1)
}

func normalizeIndex(i, n int64) int64 {
	if i >= 0 {
		return i
	}
	return n + i
}

func (s filterSelector) eval(current, root any) NodeList {
	var values []any
	switch v := current.(type) {
	case map[string]any:
		values = sortedValues(v)
	case []any:
		values = v
	}
	var results NodeList
	for _, v := range values {
		if s.expr.eval(v, root) {
			results = append(results, v)
		}
	}
	return results
}

func optionalNode(v any, ok bool) NodeList {
	if !ok {
		return nil
	}
	return NodeList{v}
}

// Singular queries

func (q absoluteSingularQuery) eval(_, root any) (any, bool) {
	return evalSingular(q.segments, root)
}

func (q relativeSingularQuery) eval(current, _ any) (any, bool) {
	return evalSingular(q.segments, current)
}

func (q absoluteSingularQuery) evalRegex(current, root any) (*regexp.Regexp, bool) {
	return compileValue(q.eval(current, root))
}

func (q relativeSingularQuery) evalRegex(current, root any) (*regexp.Regexp, bool) {
	return compileValue(q.eval(current, root))
}

func evalSingular(segments []singularQuerySegment, v any) (any, bool) {
	for _, s := range segments {
		var ok bool
		if v, ok = s.evalOne(v); !ok {
			return nil, false
		}
	}
	return v, true
}

// Logical expressions

func (e testExpr) eval(current, root any) bool {
	return e.kind.test(current, root) != e.not
}

func (k testFilterQuery) test(current, root any) bool {
	return len(k.query.evalFilter(current, root)) > 0
}

func (k testFunction) test(current, root any) bool {
	return k.function.eval(current, root)
}

func (e andExpr) eval(current, root any) bool {
	for _, operand := range e.operands {
		if !operand.eval(current, root) {
			return false
		}
	}
	return true
}

func (e orExpr) eval(current, root any) bool {
	for _, operand := range e.operands {
		if operand.eval(current, root) {
			return true
		}
	}
	return false
}

func (e notExpr) eval(current, root any) bool { return !e.expr.eval(current, root) }

// Comparisons

func (e comparisonExpr) eval(current, root any) bool {
	l, lok := e.left.eval(current, root)
	r, rok := e.right.eval(current, root)
	// !=, <=, > and >= are defined in terms of == and <.
	switch e.operator {
	case opEqual:
		return isEqual(l, lok, r, rok)
	case opNotEqual:
		return !isEqual(l, lok, r, rok)
	case opLess:
		return isLess(l, lok, r, rok)
	case opLessOrEqual:
		return isLess(l, lok, r, rok) || isEqual(l, lok, r, rok)
	case opGreater:
		return isLess(r, rok, l, lok)
	case opGreaterOrEqual:
		return isLess(r, rok, l, lok) || isEqual(l, lok, r, rok)
	}
	return false
}

// isEqual compares two optional values, with numeric equality for numbers
// (110 equals 110.0).
func isEqual(l any, lok bool, r any, rok bool) bool {
	if !lok || !rok {
		return !lok && !rok
	}
	ln, lnum := l.(json.Number)
	rn, rnum := r.(json.Number)
	if lnum && rnum {
		a, aok := asF64(ln)
		b, bok := asF64(rn)
		// Hurl panics on numbers that are not finite f64s.
		return aok && bok && math.Abs(a-b) < 1e-12
	}
	return jsonEqual(l, r)
}

func isLess(l any, lok bool, r any, rok bool) bool {
	if !lok || !rok {
		return false
	}
	switch l := l.(type) {
	case string:
		r, ok := r.(string)
		return ok && l < r
	case json.Number:
		r, ok := r.(json.Number)
		if !ok {
			return false
		}
		a, aok := asF64(l)
		b, bok := asF64(r)
		return aok && bok && b-a > 1e-12
	}
	return false
}

// asF64 is serde_json's Number::as_f64: the number as a finite f64, if any.
func asF64(n json.Number) (float64, bool) {
	f, err := strconv.ParseFloat(string(n), 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return f, !math.IsInf(f, 0)
}

// jsonEqual is serde_json Value equality. With arbitrary_precision, numbers
// are compared by their text, so 1 and 1.0 differ when nested.
func jsonEqual(a, b any) bool {
	switch a := a.(type) {
	case nil:
		return b == nil
	case bool:
		b, ok := b.(bool)
		return ok && a == b
	case string:
		b, ok := b.(string)
		return ok && a == b
	case json.Number:
		b, ok := b.(json.Number)
		return ok && canonicalNumber(a) == canonicalNumber(b)
	case []any:
		b, ok := b.([]any)
		return ok && slices.EqualFunc(a, b, jsonEqual)
	case map[string]any:
		b, ok := b.(map[string]any)
		if !ok || len(a) != len(b) {
			return false
		}
		for k, av := range a {
			bv, ok := b[k]
			if !ok || !jsonEqual(av, bv) {
				return false
			}
		}
		return true
	}
	return false
}

// canonicalNumber normalises a JSON number lexeme the way serde_json's
// arbitrary_precision parser stores it: exponents as "e", always signed.
func canonicalNumber(n json.Number) string {
	s := string(n)
	i := strings.IndexAny(s, "eE")
	if i < 0 {
		return s
	}
	exp := s[i+1:]
	if !strings.HasPrefix(exp, "+") && !strings.HasPrefix(exp, "-") {
		exp = "+" + exp
	}
	return s[:i] + "e" + exp
}

// Literals

func (l literal) eval(_, _ any) (any, bool) { return l.json(), true }

func (l literal) json() any {
	if n, ok := l.value.(number); ok {
		return n.jsonNumber()
	}
	return l.value
}

// Functions

func (f lengthFunction) eval(current, root any) (any, bool) {
	arg, ok := f.arg.eval(current, root)
	if !ok {
		return nil, false
	}
	n, ok := length(arg)
	if !ok {
		return nil, false
	}
	return json.Number(strconv.Itoa(n)), true
}

func length(v any) (int, bool) {
	switch v := v.(type) {
	case []any:
		return len(v), true
	case map[string]any:
		return len(v), true
	case string:
		return utf8.RuneCountInString(v), true
	}
	return 0, false
}

func (f countFunction) eval(current, root any) (any, bool) {
	return json.Number(strconv.Itoa(len(f.arg.evalFilter(current, root)))), true
}

func (f valueFunction) eval(current, root any) (any, bool) {
	nodes := f.arg.evalFilter(current, root)
	if len(nodes) == 1 {
		return nodes[0], true
	}
	return nil, false
}

func (f lengthFunction) evalRegex(current, root any) (*regexp.Regexp, bool) {
	return compileValue(f.eval(current, root))
}

func (f countFunction) evalRegex(current, root any) (*regexp.Regexp, bool) {
	return compileValue(f.eval(current, root))
}

func (f valueFunction) evalRegex(current, root any) (*regexp.Regexp, bool) {
	return compileValue(f.eval(current, root))
}

func (l regexLiteral) evalRegex(_, _ any) (*regexp.Regexp, bool) { return l.re, true }

func compileValue(v any, ok bool) (*regexp.Regexp, bool) {
	s, isString := v.(string)
	if !ok || !isString {
		return nil, false
	}
	re, err := regexp.Compile(s)
	return re, err == nil
}

func (f matchFunction) eval(current, root any) bool {
	s, p, ok := evalStringAndPattern(f.arg, f.pattern, current, root)
	if !ok {
		return false
	}
	// Total match.
	re, err := regexp.Compile("^" + p.String() + "$")
	return err == nil && re.MatchString(s)
}

func (f searchFunction) eval(current, root any) bool {
	s, p, ok := evalStringAndPattern(f.arg, f.pattern, current, root)
	return ok && p.MatchString(s)
}

func evalStringAndPattern(arg valueType, pattern regexArgument, current, root any) (string, *regexp.Regexp, bool) {
	v, ok := arg.eval(current, root)
	p, pok := pattern.evalRegex(current, root)
	s, isString := v.(string)
	if !ok || !isString || !pok {
		return "", nil, false
	}
	return s, p, true
}
