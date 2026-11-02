package jsonpath

import (
	"encoding/json"
	"strings"
	"testing"
)

// j decodes a JSON document the way go-hurl does (UseNumber).
func j(s string) any {
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		panic(err)
	}
	return v
}

func js(docs ...string) NodeList {
	nodes := NodeList{}
	for _, d := range docs {
		nodes = append(nodes, j(d))
	}
	return nodes
}

func assertNodes(t *testing.T, got, want NodeList) {
	t.Helper()
	if !jsonEqual(got, want) {
		t.Errorf("\n got: %s\nwant: %s", toJSON(got, false), toJSON(want, false))
	}
}

func assertValue(t *testing.T, got any, ok bool, want string) {
	t.Helper()
	if !ok || !jsonEqual(got, j(want)) {
		t.Errorf("got %s (%v), want %s", toJSON(got, false), ok, want)
	}
}

func TestEvalRootIdentifier(t *testing.T) {
	root := j(`{"greeting": "Hello"}`)
	assertNodes(t, (&Query{}).Eval(root), NodeList{root})
}

// Segments

func TestEvalSegment(t *testing.T) {
	current := j(`{"greeting": "Hello"}`)
	root := j(`"unused"`)
	assertNodes(t, childName("greeting").eval(current, root), js(`"Hello"`))
}

func TestEvalDescendantSegment(t *testing.T) {
	current := j(`{
	  "o": {"j": 1, "k": 2},
	  "a": [5, 3, [{"j": 4}, {"k": 6}]]
	}`)
	root := j(`"unused"`)
	assertNodes(t,
		descendantSegment{selectors: []selector{nameSelector{value: "j"}}}.eval(current, root),
		js(`4`, `1`))
	assertNodes(t,
		descendantSegment{selectors: []selector{indexSelector{value: 0}}}.eval(current, root),
		js(`5`, `{"j": 4}`))
	assertNodes(t,
		descendantSegment{selectors: []selector{wildcardSelector{}}}.eval(current, root),
		js(`[5, 3, [{"j": 4}, {"k": 6}]]`, `{"j": 1, "k": 2}`, `5`, `3`, `[{"j": 4}, {"k": 6}]`,
			`{"j": 4}`, `{"k": 6}`, `4`, `6`, `1`, `2`))
}

func TestDescendants(t *testing.T) {
	assertNodes(t, descendants(j(`"Hello"`)), js(`"Hello"`))
	assertNodes(t, descendants(j(`[1, 2, 3]`)), js(`[1, 2, 3]`, `1`, `2`, `3`))
	assertNodes(t, descendants(j(`{"name": "Bob"}`)), js(`{"name": "Bob"}`, `"Bob"`))
}

// Selectors

func TestEvalNameSelector(t *testing.T) {
	current := j(`{"greeting": "Hello"}`)
	assertNodes(t, nameSelector{value: "greeting"}.eval(current, j(`"unused"`)), js(`"Hello"`))
	v, ok := nameSelector{value: "greeting"}.evalOne(current)
	assertValue(t, v, ok, `"Hello"`)
}

func TestEvalWildcardSelector(t *testing.T) {
	assertNodes(t, wildcardSelector{}.eval(j(`{"o": {"j": 1, "k": 2}, "a": [5, 3]}`), nil), js(`[5, 3]`, `{"j": 1, "k": 2}`))
	assertNodes(t, wildcardSelector{}.eval(j(`{"j": 1, "k": 2}`), nil), js(`1`, `2`))
	assertNodes(t, wildcardSelector{}.eval(j(`[5, 3]`), nil), js(`5`, `3`))
}

func TestEvalIndexSelector(t *testing.T) {
	current := j(`["a", "b"]`)
	for index, want := range map[int64]string{0: `"a"`, 1: `"b"`, -1: `"b"`, -2: `"a"`} {
		v, ok := indexSelector{value: index}.evalOne(current)
		assertValue(t, v, ok, want)
	}
	// out of bounds is not an error
	for _, index := range []int64{2, -3} {
		if _, ok := (indexSelector{value: index}).evalOne(current); ok {
			t.Errorf("index %d should be out of bounds", index)
		}
	}
}

func TestEvalArraySliceSelector(t *testing.T) {
	current := j(`["a", "b", "c", "d", "e", "f", "g"]`)

	if got := (arraySliceSelector{start: ptr[int64](1), end: ptr[int64](3), step: 0}).eval(current, nil); len(got) != 0 {
		t.Errorf("got %v", got)
	}

	tests := []struct {
		selector     arraySliceSelector
		start, end   int64
		lower, upper int64
		want         NodeList
	}{
		{arraySliceSelector{start: ptr[int64](1), end: ptr[int64](3), step: 1}, 1, 3, 1, 3, js(`"b"`, `"c"`)},
		{arraySliceSelector{start: ptr[int64](5), step: 1}, 5, 7, 5, 7, js(`"f"`, `"g"`)},
		{arraySliceSelector{start: ptr[int64](1), end: ptr[int64](5), step: 2}, 1, 5, 1, 5, js(`"b"`, `"d"`)},
		{arraySliceSelector{start: ptr[int64](5), end: ptr[int64](1), step: -2}, 5, 1, 1, 5, js(`"f"`, `"d"`)},
		{arraySliceSelector{step: -1}, 6, -8, -1, 6, js(`"g"`, `"f"`, `"e"`, `"d"`, `"c"`, `"b"`, `"a"`)},
	}
	for _, tt := range tests {
		s := tt.selector
		if got := s.startIndex(7); got != tt.start {
			t.Errorf("start: got %d, want %d", got, tt.start)
		}
		if got := s.endIndex(7); got != tt.end {
			t.Errorf("end: got %d, want %d", got, tt.end)
		}
		if lower, upper := s.bounds(7); lower != tt.lower || upper != tt.upper {
			t.Errorf("bounds: got (%d, %d), want (%d, %d)", lower, upper, tt.lower, tt.upper)
		}
		assertNodes(t, s.eval(current, nil), tt.want)
	}
}

func TestEvalFilterSelector(t *testing.T) {
	current := j(`[3, 5, 1, 2, 4, 6, {"b": "j"}, {"b": "k"}, {"b": {}}, {"b": "kilo"}]`)
	// @.b
	s := filterSelector{expr: testRelative(childName("b"))}
	assertNodes(t, s.eval(current, j(`{}`)), js(`{"b": "j"}`, `{"b": "k"}`, `{"b": {}}`, `{"b": "kilo"}`))
}

// Logical expressions

func TestEvalTestExpr(t *testing.T) {
	// @.b
	e := testRelative(childName("b"))
	if !e.eval(j(`{"b": "j"}`), j(`{}`)) {
		t.Error("@.b should match {b: j}")
	}
	if e.eval(j(`3`), j(`{}`)) {
		t.Error("@.b should not match 3")
	}

	// $.*.name
	e = testExpr{kind: testFilterQuery{query: &Query{segments: []segment{childWildcard(), childName("name")}}}}
	if !e.eval(j(`{"name": "bob"}`), j(`[1, {"name": "bob"}]`)) {
		t.Error("$.*.name should match")
	}
	if e.eval(j(`{"name": "bob"}`), j(`[1, 2]`)) {
		t.Error("$.*.name should not match")
	}
}

func TestEvalOrExpr(t *testing.T) {
	// @<2 || @>4
	e := orExpr{operands: []logicalExpr{
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(2)}, operator: opLess},
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(4)}, operator: opGreater},
	}}
	root := j(`[1, 2, 3, 4, 5, 6]`)
	if !e.eval(j(`1`), root) || e.eval(j(`3`), root) {
		t.Error("unexpected or result")
	}
}

func TestEvalAndExpr(t *testing.T) {
	// @>1 && @<4
	e := andExpr{operands: []logicalExpr{
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(1)}, operator: opGreater},
		comparisonExpr{left: relativeSingularQuery{}, right: literal{value: integerNumber(4)}, operator: opLess},
	}}
	root := j(`[1, 2, 3, 4, 5, 6]`)
	if e.eval(j(`1`), root) || !e.eval(j(`2`), root) {
		t.Error("unexpected and result")
	}
}

// Comparisons

func TestIsEqual(t *testing.T) {
	if !isEqual(j(`110`), true, j(`110.0`), true) {
		t.Error("110 == 110.0")
	}
	if !isEqual(j(`110.0`), true, j(`110.00000000000001`), true) {
		t.Error("110.0 == 110.00000000000001")
	}
}

func TestIsLess(t *testing.T) {
	if isLess(j(`110`), true, j(`110.0`), true) {
		t.Error("!(110 < 110.0)")
	}
	if isLess(j(`110.0`), true, j(`110.00000000000001`), true) {
		t.Error("!(110.0 < 110.00000000000001)")
	}
}

func nameQuery(name string) absoluteSingularQuery {
	return absoluteSingularQuery{segments: []singularQuerySegment{nameSelector{value: name}}}
}

func TestEvalComparison(t *testing.T) {
	current := j(`{}`)
	root := j(`{"obj": {"x": "y"}, "arr": [2, 3]}`)
	one := literal{value: integerNumber(1)}
	tests := []struct {
		comment     string
		left, right valueType
		op          comparisonOp
		want        bool
	}{
		{"Empty nodelists", nameQuery("absent1"), nameQuery("absent2"), opEqual, true},
		{"== implies <=", nameQuery("absent1"), nameQuery("absent2"), opLessOrEqual, true},
		{"Empty nodelist", nameQuery("absent"), literal{value: "g"}, opEqual, false},
		{"Empty nodelists", nameQuery("absent1"), nameQuery("absent2"), opNotEqual, false},
		{"Empty nodelist", nameQuery("absent"), literal{value: "g"}, opNotEqual, true},
		{"Numeric comparison", one, literal{value: integerNumber(2)}, opLessOrEqual, true},
		{"Numeric comparison", one, literal{value: integerNumber(2)}, opGreater, false},
		{"Type mismatch", literal{value: integerNumber(13)}, literal{value: "13"}, opEqual, false},
		{"String comparison", literal{value: "a"}, literal{value: "b"}, opLessOrEqual, true},
		{"String comparison", literal{value: "a"}, literal{value: "b"}, opGreater, false},
		{"Type mismatch", nameQuery("obj"), nameQuery("arr"), opEqual, false},
		{"Type mismatch", nameQuery("obj"), nameQuery("arr"), opNotEqual, true},
		{"Object comparison", nameQuery("obj"), nameQuery("obj"), opEqual, true},
		{"Object comparison", nameQuery("obj"), nameQuery("obj"), opNotEqual, false},
		{"Array comparison", nameQuery("arr"), nameQuery("arr"), opEqual, true},
		{"Array comparison", nameQuery("arr"), nameQuery("arr"), opNotEqual, false},
		{"Type mismatch", nameQuery("obj"), literal{value: integerNumber(17)}, opEqual, false},
		{"Type mismatch", nameQuery("obj"), literal{value: integerNumber(17)}, opNotEqual, true},
		{"Objects and arrays do not offer < comparison", nameQuery("obj"), nameQuery("arr"), opLessOrEqual, false},
		{"Objects and arrays do not offer < comparison", nameQuery("obj"), nameQuery("arr"), opLess, false},
		{"== implies <=", nameQuery("obj"), nameQuery("obj"), opLessOrEqual, true},
		{"== implies <=", nameQuery("arr"), nameQuery("arr"), opLessOrEqual, true},
		{"Arrays do not offer < comparison", one, nameQuery("arr"), opLessOrEqual, false},
		{"Arrays do not offer < comparison", one, nameQuery("arr"), opGreaterOrEqual, false},
		{"Arrays do not offer < comparison", one, nameQuery("arr"), opGreater, false},
		{"Arrays do not offer < comparison", one, nameQuery("arr"), opLess, false},
		{"== implies <=", literal{value: true}, literal{value: true}, opLessOrEqual, true},
		{"Booleans do not offer < comparison", literal{value: true}, literal{value: true}, opGreater, false},
	}
	for i, tt := range tests {
		e := comparisonExpr{left: tt.left, right: tt.right, operator: tt.op}
		if got := e.eval(current, root); got != tt.want {
			t.Errorf("%d (%s): got %v, want %v", i, tt.comment, got, tt.want)
		}
	}
}

func TestEvalComparisonNumber(t *testing.T) {
	for _, right := range []number{integerNumber(110), floatNumber(110.0)} {
		// @.a==110 and @.a==110.0
		e := comparisonExpr{left: relName("a"), right: literal{value: right}, operator: opEqual}
		for _, doc := range []string{`{"a":110}`, `{"a":110.0}`, `{"a":1.1e2}`} {
			if !e.eval(j(doc), j(`{}`)) {
				t.Errorf("%v == %s", right, doc)
			}
		}
	}
}

// Literals

func TestEvalLiteralNumber(t *testing.T) {
	// Using different float values
	value1 := literal{value: floatNumber(110.0)}.json()
	assertEqual(t, value1, json.Number("110.0"))
	value2 := literal{value: floatNumber(110.00000000000001)}.json()
	assertEqual(t, value2, json.Number("110.00000000000001"))
	if jsonEqual(value1, value2) {
		t.Error("value1 != value2")
	}

	// integer
	assertEqual(t, literal{value: integerNumber(42)}.json(), json.Number("42"))

	// big integer
	assertEqual(t, literal{value: bigIntegerNumber("99999999999")}.json(), json.Number("99999999999"))
}

func TestFloatJSONNumber(t *testing.T) {
	// Expected values are ryu's (serde_json arbitrary_precision) formatting.
	tests := map[float64]string{
		0:        "0.0",
		1:        "1.0",
		-1.5:     "-1.5",
		12.34:    "12.34",
		1e15:     "1000000000000000.0",
		1e16:     "1e16",
		1.234e17: "1.234e17",
		0.001234: "0.001234",
		1e-5:     "0.00001",
		1e-6:     "1e-6",
		1.5e-7:   "1.5e-7",
		0.0001:   "0.0001",
	}
	for f, want := range tests {
		if got := floatNumber(f).jsonNumber(); string(got) != want {
			t.Errorf("%v: got %s, want %s", f, got, want)
		}
	}
}

// Function arguments

func TestEvalValueTypeArgument(t *testing.T) {
	v, ok := literal{value: "hello"}.eval(j(`{}`), j(`{}`))
	assertValue(t, v, ok, `"hello"`)

	v, ok = literal{value: integerNumber(1)}.eval(j(`{}`), j(`{}`))
	assertValue(t, v, ok, `1`)

	// @.authors
	v, ok = relName("authors").eval(j(`{"authors": ["a", "b", "c"]}`), j(`{}`))
	assertValue(t, v, ok, `["a", "b", "c"]`)
}

func TestEvalNodesTypeArgument(t *testing.T) {
	// @.books
	q := relativeQuery{segments: []segment{childName("books")}}
	assertNodes(t, q.evalFilter(j(`{"books": ["book1", "book2"]}`), j(`{}`)), js(`["book1", "book2"]`))
	if got := q.evalFilter(j(`{}`), j(`{}`)); len(got) != 0 {
		t.Errorf("got %v", got)
	}
}

// Functions

func TestEvalLength(t *testing.T) {
	// length("abc")
	v, ok := lengthFunction{arg: literal{value: "abc"}}.eval(j(`{}`), j(`{}`))
	assertValue(t, v, ok, `3`)

	// length(1)
	if _, ok := (lengthFunction{arg: literal{value: integerNumber(1)}}).eval(j(`{}`), j(`{}`)); ok {
		t.Error("length(1) should be nothing")
	}

	// length(@.authors)
	f := lengthFunction{arg: relName("authors")}
	v, ok = f.eval(j(`{"authors": ["a", "b", "c"]}`), j(`{}`))
	assertValue(t, v, ok, `3`)
	v, ok = f.eval(j(`{"authors": {"a":"a", "b": "b", "c": "c"}}`), j(`{}`))
	assertValue(t, v, ok, `3`)
	if _, ok := f.eval(j(`{}`), j(`{}`)); ok {
		t.Error("length of absent should be nothing")
	}
}

func TestEvalCount(t *testing.T) {
	// count(@.*.author)
	f := countFunction{arg: relativeQuery{segments: []segment{childWildcard(), childName("author")}}}
	v, ok := f.eval(j(`[{"author": "Bob"},{"author": "Bill"}]`), j(`{}`))
	assertValue(t, v, ok, `2`)
}

func TestEvalMatch(t *testing.T) {
	// match(@.date, "1974-05-..")
	f := matchFunction{arg: relName("date"), pattern: regexLit("1974-05-..")}
	for doc, want := range map[string]bool{
		`{"date": "1974-05-01"}`:  true,
		`{"date": "74-05-01"}`:    false,
		`{"date": "1974-04-01"}`:  false,
		`{"_date": "1974-05-01"}`: false,
	} {
		if got := f.eval(j(doc), j(`{}`)); got != want {
			t.Errorf("%s: got %v, want %v", doc, got, want)
		}
	}

	// match with regex . (1 character)
	f = matchFunction{arg: relativeSingularQuery{}, pattern: regexLit(".")}
	for doc, want := range map[string]bool{
		`"a"`:      true,
		`"\r"`:     true, // JavaScript is the exception, not the rule
		`"\u2028"`: true, // line separator
		`"\u2029"`: true, // paragraph separator
		`""`:       false,
		`"\n"`:     false,
		`"ab"`:     false,
	} {
		if got := f.eval(j(doc), j(`{}`)); got != want {
			t.Errorf("%s: got %v, want %v", doc, got, want)
		}
	}

	// match with regex [.b] dot or b (not any character)
	f = matchFunction{arg: relativeSingularQuery{}, pattern: regexLit("a[.b]c")}
	for doc, want := range map[string]bool{`"a.c"`: true, `"abc"`: true, `"axc"`: false} {
		if got := f.eval(j(doc), j(`{}`)); got != want {
			t.Errorf("%s: got %v, want %v", doc, got, want)
		}
	}
}

func TestEvalSearch(t *testing.T) {
	// search(@.author, "[BR]ob")
	f := searchFunction{arg: relName("author"), pattern: regexLit("[BR]ob")}
	for doc, want := range map[string]bool{
		`{"author": "Bob Dylan"}`:          true,
		`{"author": "Robert De Niro"}`:     true,
		`{"author": "Christiano Ronaldo"}`: false,
	} {
		if got := f.eval(j(doc), j(`{}`)); got != want {
			t.Errorf("%s: got %v, want %v", doc, got, want)
		}
	}
}

func TestEvalValue(t *testing.T) {
	// value(@..color)
	f := valueFunction{arg: relativeQuery{segments: []segment{
		descendantSegment{selectors: []selector{nameSelector{value: "color"}}},
	}}}
	v, ok := f.eval(j(`{"color": "red"}`), j(`{}`))
	assertValue(t, v, ok, `"red"`)
	if _, ok := f.eval(j(`{}`), j(`{}`)); ok {
		t.Error("value of empty nodelist should be nothing")
	}
}
