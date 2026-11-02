package jsonpath

import (
	"regexp"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func relName(name string) relativeSingularQuery {
	return relativeSingularQuery{segments: []singularQuerySegment{nameSelector{value: name}}}
}

func testRelative(segments ...segment) testExpr {
	return testExpr{kind: testFilterQuery{query: relativeQuery{segments: segments}}}
}

func regexLit(pattern string) regexLiteral {
	return regexLiteral{re: regexp.MustCompile(pattern)}
}

// Logical expressions

func TestLogicalOrExpr(t *testing.T) {
	r := reader.New("@.b")
	e, _ := logicalOrExpr(r, 0)
	assertEqual(t, e, testRelative(childName("b")))
	assertIndex(t, r, 3)
}

func TestOrExpression(t *testing.T) {
	r := reader.New("@<2 || @>4")
	e, _ := logicalOrExpr(r, 0)
	assertEqual(t, e, orExpr{operands: []logicalExpr{
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(2)}, operator: opLess},
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(4)}, operator: opGreater},
	}})
	assertIndex(t, r, 10)
}

func TestMaxNestingDepth(t *testing.T) {
	validOr := strings.Repeat("@ || ", maxNestingDepth-1) + "@"
	if _, err := logicalOrExpr(reader.New(validOr), 0); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	tooDeepOr := strings.Repeat("@ || ", maxNestingDepth) + "@"
	r := reader.New(tooDeepOr)
	_, err := logicalOrExpr(r, 0)
	assertEqual(t, err, &ParseError{r.Cursor().Pos, MaxDepthExceeded{MaxDepth: maxNestingDepth}})

	validParens := strings.Repeat("(", maxNestingDepth-1) + "@" + strings.Repeat(")", maxNestingDepth-1)
	if _, err := logicalOrExpr(reader.New(validParens), 0); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	tooDeepParens := strings.Repeat("(", maxNestingDepth) + "@" + strings.Repeat(")", maxNestingDepth)
	r = reader.New(tooDeepParens)
	_, err = logicalOrExpr(r, 0)
	assertEqual(t, err, &ParseError{r.Cursor().Pos, MaxDepthExceeded{MaxDepth: maxNestingDepth}})
}

func TestAndExpression(t *testing.T) {
	r := reader.New("@>1&&@<4")
	e, _ := logicalAndExpr(r, 0)
	assertEqual(t, e, andExpr{operands: []logicalExpr{
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(1)}, operator: opGreater},
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(4)}, operator: opLess},
	}})
}

func TestParenExpression(t *testing.T) {
	r := reader.New("!(@.b)")
	e, _ := tryParenExpr(r, 0)
	assertEqual(t, e, notExpr{expr: testRelative(childName("b"))})
	assertIndex(t, r, 6)
}

func TestParenExpressionNone(t *testing.T) {
	r := reader.New("!@.b")
	e, err := tryParenExpr(r, 0)
	if e != nil || err != nil {
		t.Errorf("got %v, %v", e, err)
	}
	assertIndex(t, r, 0)
}

func TestTestExpr(t *testing.T) {
	r := reader.New("@.b")
	e, _ := tryTestExpr(r, 0)
	assertEqual(t, e, ptr(testRelative(childName("b"))))
	assertIndex(t, r, 3)

	r = reader.New("$")
	e, _ = tryTestExpr(r, 0)
	assertEqual(t, e, &testExpr{kind: testFilterQuery{query: &Query{}}})
	assertIndex(t, r, 1)

	r = reader.New("$.*.a]")
	e, _ = tryTestExpr(r, 0)
	assertEqual(t, e, &testExpr{kind: testFilterQuery{query: &Query{segments: []segment{childWildcard(), childName("a")}}}})
	assertIndex(t, r, 5)

	r = reader.New("!@.a")
	e, _ = tryTestExpr(r, 0)
	want := testRelative(childName("a"))
	want.not = true
	assertEqual(t, e, &want)
	assertIndex(t, r, 4)

	r = reader.New("match(@.a, 'a.*')")
	e, _ = tryTestExpr(r, 0)
	assertEqual(t, e, &testExpr{kind: testFunction{function: matchFunction{arg: relName("a"), pattern: regexLit("a.*")}}})
	assertIndex(t, r, 17)
}

// Comparisons

func TestComparisonExpr(t *testing.T) {
	r := reader.New("1<=2")
	e, _ := tryComparison(r, 0)
	assertEqual(t, e, &comparisonExpr{
		left:     literal{value: integerNumber(1)},
		right:    literal{value: integerNumber(2)},
		operator: opLessOrEqual,
	})
	assertIndex(t, r, 4)

	r = reader.New("@.a == 1")
	e, _ = tryComparison(r, 0)
	assertEqual(t, e, &comparisonExpr{
		left:     relName("a"),
		right:    literal{value: integerNumber(1)},
		operator: opEqual,
	})
	assertIndex(t, r, 8)
}

func TestComparisonExprError(t *testing.T) {
	r := reader.New("@ == 1e999")
	_, err := tryComparison(r, 0)
	assertEqual(t, err, &ParseError{pos(1, 6), Expecting("Number is too big")})
	assertIndex(t, r, 10)
}

func TestComparisonExprNone(t *testing.T) {
	// These are test expressions, not comparison expressions.
	for _, input := range []string{"@.b]", "$.*.a]"} {
		r := reader.New(input)
		e, err := tryComparison(r, 0)
		if e != nil || err != nil {
			t.Errorf("%s: got %v, %v", input, e, err)
		}
		assertIndex(t, r, 0)
	}
}

func TestComparable(t *testing.T) {
	r := reader.New("1")
	v, _ := parseComparable(r, 0)
	assertEqual(t, v, literal{value: integerNumber(1)})
	assertIndex(t, r, 1)
}

func TestComparableError(t *testing.T) {
	r := reader.New("1e999")
	_, err := tryComparable(r, 0)
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("Number is too big")})
	assertIndex(t, r, 5)
}

func TestComparisonOp(t *testing.T) {
	r := reader.New("==")
	op, ok := tryComparisonOp(r)
	if !ok || op != opEqual {
		t.Errorf("got %v, %v", op, ok)
	}
	assertIndex(t, r, 2)

	r = reader.New("]")
	if _, ok := tryComparisonOp(r); ok {
		t.Error("expected no operator")
	}
	assertIndex(t, r, 0)
}

// Singular queries

func TestSingularQuery(t *testing.T) {
	r := reader.New("$.store")
	q, _ := trySingularQuery(r)
	assertEqual(t, q, absoluteSingularQuery{segments: []singularQuerySegment{nameSelector{value: "store"}}})
	assertIndex(t, r, 7)
}

func TestSingularQueryNone(t *testing.T) {
	r := reader.New("1")
	q, err := trySingularQuery(r)
	if q != nil || err != nil {
		t.Errorf("got %v, %v", q, err)
	}
	assertIndex(t, r, 0)
}

func TestSingularQuerySegment(t *testing.T) {
	r := reader.New(".store")
	s, _ := trySingularQuerySegment(r)
	assertEqual(t, s, nameSelector{value: "store"})
	assertIndex(t, r, 6)

	for _, input := range []string{"]", ".*"} {
		r = reader.New(input)
		s, err := trySingularQuerySegment(r)
		if s != nil || err != nil {
			t.Errorf("%s: got %v, %v", input, s, err)
		}
		assertIndex(t, r, 0)
	}
}

// Functions

func TestLengthFunction(t *testing.T) {
	r := reader.New("length(\"abc\")")
	f, _ := tryValueTypeFunction(r, 0)
	assertEqual(t, f, lengthFunction{arg: literal{value: "abc"}})
	assertIndex(t, r, 13)

	r = reader.New("length(value($..c))")
	f, _ = tryValueTypeFunction(r, 0)
	assertEqual(t, f, lengthFunction{arg: valueFunction{arg: &Query{segments: []segment{
		descendantSegment{selectors: []selector{nameSelector{value: "c"}}},
	}}}})
	assertIndex(t, r, 19)
}

func TestLengthFunctionError(t *testing.T) {
	// not well-typed since @.* is a non-singular query
	_, err := tryValueTypeFunction(reader.New("length(@.*)"), 0)
	assertEqual(t, err, &ParseError{pos(1, 9), Expecting(")")})
}

func TestCountFunction(t *testing.T) {
	r := reader.New("count(@.*.author)")
	f, _ := tryValueTypeFunction(r, 0)
	assertEqual(t, f, countFunction{arg: relativeQuery{segments: []segment{childWildcard(), childName("author")}}})
	assertIndex(t, r, 17)
}

func TestCountFunctionError(t *testing.T) {
	// not well-typed since 1 is not a query or function expression
	_, err := tryValueTypeFunction(reader.New("count(1)"), 0)
	assertEqual(t, err, &ParseError{pos(1, 7), Expecting("a NodesType argument")})
}

func TestMatchFunction(t *testing.T) {
	r := reader.New("match(@.date, \"1974-05-01\")")
	f, _ := tryLogicalTypeFunction(r, 0)
	assertEqual(t, f, matchFunction{arg: relName("date"), pattern: regexLit("1974-05-01")})
	assertIndex(t, r, 27)
}

func TestSearchFunction(t *testing.T) {
	r := reader.New("search(@.author, \"[BR]ob\")")
	f, _ := tryLogicalTypeFunction(r, 0)
	assertEqual(t, f, searchFunction{arg: relName("author"), pattern: regexLit("[BR]ob")})
	assertIndex(t, r, 26)
}

func TestValueFunction(t *testing.T) {
	r := reader.New("value(@..color)")
	f, _ := tryValueTypeFunction(r, 0)
	assertEqual(t, f, valueFunction{arg: relativeQuery{segments: []segment{
		descendantSegment{selectors: []selector{nameSelector{value: "color"}}},
	}}})
	assertIndex(t, r, 15)
}

// Function arguments

func TestValueTypeArgument(t *testing.T) {
	r := reader.New("42")
	v, _ := parseValueTypeArgument(r, 0)
	assertEqual(t, v, literal{value: integerNumber(42)})
	assertIndex(t, r, 2)
}

func TestRegexValueTypeArgument(t *testing.T) {
	for _, input := range []string{"42", "[]"} {
		_, err := parseRegexValueTypeArgument(reader.New(input), 0)
		assertEqual(t, err, &ParseError{pos(1, 1), Expecting("a RegexValueType argument")})
	}

	v, _ := parseRegexValueTypeArgument(reader.New("\"[Bb]ob\""), 0)
	assertEqual(t, v, regexLit("[Bb]ob"))
}

func TestNodesTypeArgument(t *testing.T) {
	r := reader.New("@.*.author")
	q, _ := parseNodesTypeArgument(r, 0)
	assertEqual(t, q, relativeQuery{segments: []segment{childWildcard(), childName("author")}})
	assertIndex(t, r, 10)
}

func TestTryRegex(t *testing.T) {
	re, err := tryRegex(reader.New("1"))
	if re != nil || err != nil {
		t.Errorf("got %v, %v", re, err)
	}

	_, err = tryRegex(reader.New("'[]'"))
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("a valid regex")})

	re, _ = tryRegex(reader.New("'[Bb]ob'"))
	if re == nil || re.String() != "[Bb]ob" {
		t.Errorf("got %v", re)
	}
}
