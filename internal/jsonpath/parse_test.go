package jsonpath

import (
	"reflect"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func pos(line, column int) reader.Pos { return reader.Pos{Line: line, Column: column} }

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("\n got: %#v\nwant: %#v", got, want)
	}
}

func assertIndex(t *testing.T, r *reader.Reader, want int) {
	t.Helper()
	if got := r.Cursor().Index; got != want {
		t.Errorf("cursor index: got %d, want %d", got, want)
	}
}

func childName(name string) segment {
	return childSegment{selectors: []selector{nameSelector{value: name}}}
}

func childWildcard() segment {
	return childSegment{selectors: []selector{wildcardSelector{}}}
}

func TestExpectStr(t *testing.T) {
	r := reader.New("hello")
	assertEqual(t, expectStr("hello", r), (*ParseError)(nil))
	assertIndex(t, r, 5)

	r = reader.New("hello ")
	assertEqual(t, expectStr("hello", r), (*ParseError)(nil))
	assertIndex(t, r, 5)

	r = reader.New("")
	assertEqual(t, expectStr("hello", r), &ParseError{pos(1, 1), Expecting("hello")})
	assertIndex(t, r, 0)

	r = reader.New("hi")
	assertEqual(t, expectStr("hello", r), &ParseError{pos(1, 1), Expecting("hello")})
	assertIndex(t, r, 2)

	r = reader.New("he")
	assertEqual(t, expectStr("hello", r), &ParseError{pos(1, 1), Expecting("hello")})
	assertIndex(t, r, 2)
}

// Query

func TestQueryEmpty(t *testing.T) {
	r := reader.New("")
	_, err := parseQuery(r)
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("$")})
	assertIndex(t, r, 0)
}

func TestQueryTrailingSpace(t *testing.T) {
	r := reader.New("$ ")
	_, err := parseQuery(r)
	assertEqual(t, err, &ParseError{pos(1, 2), Expecting("end of query")})
	assertIndex(t, r, 1)
}

func TestQueryRootIdentifier(t *testing.T) {
	r := reader.New("$")
	q, _ := parseQuery(r)
	assertEqual(t, q, &Query{})
	assertIndex(t, r, 1)
}

func TestQueryChildSegment(t *testing.T) {
	r := reader.New("$['store']")
	q, _ := parseQuery(r)
	assertEqual(t, q, &Query{segments: []segment{childName("store")}})
	assertIndex(t, r, 10)

	r = reader.New("$[?@['isbn']]")
	q, _ = parseQuery(r)
	assertEqual(t, q, &Query{segments: []segment{
		childSegment{selectors: []selector{filterSelector{expr: testExpr{
			kind: testFilterQuery{query: relativeQuery{segments: []segment{childName("isbn")}}},
		}}}},
	}})
	assertIndex(t, r, 13)

	r = reader.New("$.book")
	q, _ = parseQuery(r)
	assertEqual(t, q, &Query{segments: []segment{childName("book")}})
	assertIndex(t, r, 6)
}

func TestRelativeQuery(t *testing.T) {
	r := reader.New("@['isbn']")
	q, _ := tryRelativeQuery(r, 0)
	assertEqual(t, q, &relativeQuery{segments: []segment{childName("isbn")}})
	assertIndex(t, r, 9)
}

func TestNestedFunctionDepth(t *testing.T) {
	valid := "$[?" + strings.Repeat("length(", maxNestingDepth) + "0" + strings.Repeat(")", maxNestingDepth) + " == 0]"
	if _, err := Parse(valid); err != nil {
		t.Errorf("unexpected error %v", err)
	}

	tooDeep := "$[?" + strings.Repeat("length(", maxNestingDepth+1) + "0" + strings.Repeat(")", maxNestingDepth+1) + " == 0]"
	r := reader.New(tooDeep)
	_, err := parseQuery(r)
	assertEqual(t, err, &ParseError{r.Cursor().Pos, MaxDepthExceeded{MaxDepth: maxNestingDepth}})
}

func TestSiblingFiltersDoNotAccumulateDepth(t *testing.T) {
	query := "$" + strings.Repeat("[?@]", maxNestingDepth+1)
	if _, err := Parse(query); err != nil {
		t.Errorf("unexpected error %v", err)
	}
}

// Segments

func TestSegments(t *testing.T) {
	r := reader.New("['isbn']]")
	s, _ := parseSegments(r, 0)
	assertEqual(t, s, []segment{childName("isbn")})
	assertIndex(t, r, 8)
}

func TestSegment(t *testing.T) {
	r := reader.New("['store']")
	s, _ := trySegment(r, 0)
	assertEqual(t, s, childName("store"))
	assertIndex(t, r, 9)
}

func TestChildSegment(t *testing.T) {
	r := reader.New("['store']")
	s, _ := tryChildSegment(r, 0)
	assertEqual(t, s, &childSegment{selectors: []selector{nameSelector{value: "store"}}})
	assertIndex(t, r, 9)
}

func TestChildSegmentError(t *testing.T) {
	r := reader.New(".1")
	_, err := tryChildSegment(r, 0)
	assertEqual(t, err, &ParseError{pos(1, 2), Expecting("a wildcard-selector or member-name shorthand")})
}

func TestDescendantSegment(t *testing.T) {
	r := reader.New("..[1]")
	s, _ := tryDescendantSegment(r, 0)
	assertEqual(t, s, &descendantSegment{selectors: []selector{indexSelector{value: 1}}})
	assertIndex(t, r, 5)

	r = reader.New("..[1]")
	seg, _ := trySegment(r, 0)
	assertEqual(t, seg, descendantSegment{selectors: []selector{indexSelector{value: 1}}})
	assertIndex(t, r, 5)
}

func TestDescendantSegmentError(t *testing.T) {
	r := reader.New("..1")
	_, err := tryDescendantSegment(r, 0)
	assertEqual(t, err, &ParseError{pos(1, 3), Expecting("a bracketed-selection, wildcard-selector or member-name shorthand")})
}

func TestBracketedSelection(t *testing.T) {
	r := reader.New("[1,'store']")
	s, _ := tryBracketedSelection(r, 0)
	assertEqual(t, s, []selector{indexSelector{value: 1}, nameSelector{value: "store"}})
	assertIndex(t, r, 11)
}

func TestShorthandNotation(t *testing.T) {
	tests := []struct {
		input string
		want  segment
		index int
	}{
		{".book", childName("book"), 5},
		{".☺", childName("☺"), 2},
		{"..book", descendantSegment{selectors: []selector{nameSelector{value: "book"}}}, 6},
		{".*", childWildcard(), 2},
		{"..*", descendantSegment{selectors: []selector{wildcardSelector{}}}, 3},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		s, _ := trySegment(r, 0)
		assertEqual(t, s, tt.want)
		assertIndex(t, r, tt.index)
	}
}

func TestNameFirst(t *testing.T) {
	for _, c := range []rune{'a', '_', '☺'} {
		got, ok := nameFirst(reader.New(string(c)))
		if !ok || got != c {
			t.Errorf("nameFirst(%q) = %q, %v", c, got, ok)
		}
	}
	if _, ok := nameFirst(reader.New("1")); ok {
		t.Error("nameFirst(1) should fail")
	}
}

// Selectors

func TestSelectors(t *testing.T) {
	r := reader.New("'store',0")
	s, _ := parseSelectors(r, 0)
	assertEqual(t, s, []selector{nameSelector{value: "store"}, indexSelector{value: 0}})
	assertIndex(t, r, 9)

	r = reader.New("1,5:7")
	s, _ = parseSelectors(r, 0)
	assertEqual(t, s, []selector{indexSelector{value: 1}, arraySliceSelector{start: ptr[int64](5), end: ptr[int64](7), step: 1}})
	assertIndex(t, r, 5)
}

func ptr[T any](v T) *T { return &v }

func TestSelector(t *testing.T) {
	r := reader.New("'store'")
	s, _ := parseSelector(r, 0)
	assertEqual(t, s, nameSelector{value: "store"})
	assertIndex(t, r, 7)
}

func TestNameSelector(t *testing.T) {
	r := reader.New("'store'")
	s, _ := tryNameSelector(r)
	assertEqual(t, s, &nameSelector{value: "store"})
	assertIndex(t, r, 7)
}

func TestWildcardSelector(t *testing.T) {
	r := reader.New("*")
	s, _ := parseSelector(r, 0)
	assertEqual(t, s, wildcardSelector{})
	assertIndex(t, r, 1)
}

func TestIndexSelector(t *testing.T) {
	r := reader.New("1")
	s, _ := tryIndexSelector(r)
	assertEqual(t, s, &indexSelector{value: 1})
	assertIndex(t, r, 1)
}

func TestArraySliceSelector(t *testing.T) {
	tests := []struct {
		input string
		want  *arraySliceSelector
		index int
	}{
		{"1:3", &arraySliceSelector{start: ptr[int64](1), end: ptr[int64](3), step: 1}, 3},
		{"5:", &arraySliceSelector{start: ptr[int64](5), step: 1}, 2},
		{"1:5:2", &arraySliceSelector{start: ptr[int64](1), end: ptr[int64](5), step: 2}, 5},
		{"1:5:-2", &arraySliceSelector{start: ptr[int64](1), end: ptr[int64](5), step: -2}, 6},
		{"::-1", &arraySliceSelector{step: -1}, 4},
		{":2", &arraySliceSelector{end: ptr[int64](2), step: 1}, 2}, // First 2 items
		{"2", nil, 0}, // This is an index selector
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		s, err := tryArraySliceSelector(r)
		assertEqual(t, err, (*ParseError)(nil))
		assertEqual(t, s, tt.want)
		assertIndex(t, r, tt.index)
	}

	s, err := tryArraySliceSelector(reader.New("?@['isbn']]"))
	if s != nil || err != nil {
		t.Errorf("got %v, %v", s, err)
	}
}

func TestFilterSelector(t *testing.T) {
	r := reader.New("?@['isbn']")
	s, _ := tryFilterSelector(r, 0)
	assertEqual(t, s, &filterSelector{expr: testExpr{
		kind: testFilterQuery{query: relativeQuery{segments: []segment{childName("isbn")}}},
	}})
	assertIndex(t, r, 10)

	r = reader.New("?@>3]")
	s, _ = tryFilterSelector(r, 0)
	assertEqual(t, s, &filterSelector{expr: comparisonExpr{
		left:     relativeSingularQuery{},
		right:    literal{value: integerNumber(3)},
		operator: opGreater,
	}})
	assertIndex(t, r, 4)
}
