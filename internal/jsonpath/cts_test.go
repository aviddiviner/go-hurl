package jsonpath

// Compliance Test Suite
// https://github.com/jsonpath-standard/jsonpath-compliance-test-suite

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

// ignoredTests are CTS cases that Hurl skips. They may apply to some existing
// implementations such as jsonpath.com (JSONPath Plus engine) but are not
// standard compliant.
var ignoredTests = []string{
	// $[?(@[0, 0, 'a'])]
	// The union selector cannot be used directly as a boolean predicate.
	// The standard-compliant expression is $[?(@[0] || @['a'])]
	"filter, non-singular existence, multiple",
	// $[?@[0:2]]
	// The slice selector cannot be used directly as a boolean predicate
	// To check that the array has at least 2 elements, you can use $[?(@[1] exists)]
	"filter, non-singular existence, slice",
	// The regex '.' should match \r (except in Javascript)
	"functions, match, dot matcher on \\u2028",
	"functions, match, dot matcher on \\u2029",
	"functions, search, dot matcher on \\u2028",
	"functions, search, dot matcher on \\u2029",
	// The second argument of the match/search function should be a string (regex)
	"functions, match, non-string second arg",
	"functions, search, non-string second arg",
}

type ctsTestCase struct {
	name            string
	document        any
	selector        string
	invalidSelector bool
	results         []NodeList // several results when the order is non-deterministic
}

type ctsTestCaseError struct {
	testcase ctsTestCase
	kind     ctsErrorKind
}

type ctsErrorKind interface{ isCTSErrorKind() }

type (
	ctsUnexpectedParseError struct{ err ParseError }
	ctsInvalidSelector      struct{}
	ctsEval                 struct{ actual NodeList }
)

func (ctsUnexpectedParseError) isCTSErrorKind() {}
func (ctsInvalidSelector) isCTSErrorKind()      {}
func (ctsEval) isCTSErrorKind()                 {}

func (tc ctsTestCase) run() *ctsTestCaseError {
	query, err := Parse(tc.selector)
	if err != nil {
		if tc.invalidSelector {
			return nil
		}
		return &ctsTestCaseError{testcase: tc, kind: ctsUnexpectedParseError{err: *err}}
	}
	if tc.invalidSelector {
		return &ctsTestCaseError{testcase: tc, kind: ctsInvalidSelector{}}
	}
	actual := query.Eval(tc.document)
	for _, result := range tc.results {
		if jsonEqual(result, actual) {
			return nil
		}
	}
	return &ctsTestCaseError{testcase: tc, kind: ctsEval{actual: actual}}
}

func (e *ctsTestCaseError) String() string {
	s := ">>> " + e.testcase.name
	switch k := e.kind.(type) {
	case ctsUnexpectedParseError:
		s += fmt.Sprintf("\ncan not parse valid selector <%s>", e.testcase.selector)
	case ctsInvalidSelector:
		s += fmt.Sprintf("\nshould not parse the invalid selector <%s>", e.testcase.selector)
	case ctsEval:
		s += "\ndocument: " + toJSON(e.testcase.document, false)
		s += "\nselector: " + e.testcase.selector
		if len(e.testcase.results) == 1 {
			s += "\nexpected: " + toJSON(e.testcase.results[0], true)
		} else {
			s += "\nexpected &one of these):"
			for _, result := range e.testcase.results {
				s += "\n  " + toJSON(result, true)
			}
		}
		s += "\nactual: " + toJSON(k.actual, true)
	}
	return s
}

func toJSON(v any, pretty bool) string {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	if err := enc.Encode(v); err != nil {
		panic(err)
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

func TestCTSRunTestcase(t *testing.T) {
	// UnexpectedParseError
	tc := ctsTestCase{name: "valid selector that fails", selector: "xxx"}
	got := tc.run()
	want := &ctsTestCaseError{testcase: tc, kind: ctsUnexpectedParseError{
		err: ParseError{Pos: reader.Pos{Line: 1, Column: 1}, Kind: Expecting("$")},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if s := got.String(); s != ">>> valid selector that fails\ncan not parse valid selector <xxx>" {
		t.Errorf("got %q", s)
	}

	// InvalidSelector
	tc = ctsTestCase{name: "invalid selector that succeeds", selector: "$", invalidSelector: true}
	got = tc.run()
	want = &ctsTestCaseError{testcase: tc, kind: ctsInvalidSelector{}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if s := got.String(); s != ">>> invalid selector that succeeds\nshould not parse the invalid selector <$>" {
		t.Errorf("got %q", s)
	}

	// EvalError
	tc = ctsTestCase{
		name:     "valid eval that fails",
		document: map[string]any{"name": "Bob"},
		selector: "$.name",
		results:  []NodeList{{"Bill"}},
	}
	got = tc.run()
	want = &ctsTestCaseError{testcase: tc, kind: ctsEval{actual: NodeList{"Bob"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if s := got.String(); s != ">>> valid eval that fails\ndocument: {\"name\":\"Bob\"}\nselector: $.name\nexpected: [\n  \"Bill\"\n]\nactual: [\n  \"Bob\"\n]" {
		t.Errorf("got %q", s)
	}

	// Eval OK
	tc = ctsTestCase{
		name:     "valid eval that succeeed",
		document: map[string]any{"name": "Bob"},
		selector: "$.name",
		results:  []NodeList{{"Bob"}},
	}
	if got := tc.run(); got != nil {
		t.Errorf("unexpected error %s", got)
	}
}

func ctsResults(test map[string]any) []NodeList {
	if results, ok := test["results"]; ok {
		var nodeLists []NodeList
		for _, r := range results.([]any) {
			nodeLists = append(nodeLists, r.([]any))
		}
		return nodeLists
	}
	if result, ok := test["result"]; ok {
		return []NodeList{result.([]any)}
	}
	return nil
}

func loadCTSTestcases(t *testing.T) []ctsTestCase {
	content, err := os.ReadFile("testdata/cts.json")
	if err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(content))
	dec.UseNumber()
	var data map[string]any
	if err := dec.Decode(&data); err != nil {
		t.Fatal(err)
	}
	var testcases []ctsTestCase
	for _, test := range data["tests"].([]any) {
		test := test.(map[string]any)
		_, invalid := test["invalid_selector"]
		testcases = append(testcases, ctsTestCase{
			name:            test["name"].(string),
			document:        test["document"],
			selector:        test["selector"].(string),
			invalidSelector: invalid,
			results:         ctsResults(test),
		})
	}
	return testcases
}

func TestCTS(t *testing.T) {
	testcases := loadCTSTestcases(t)
	countTotal := len(testcases)

	// Make sure that ignored tests are in the test suite.
	for _, name := range ignoredTests {
		if !slices.ContainsFunc(testcases, func(tc ctsTestCase) bool { return tc.name == name }) {
			t.Fatalf("ignored test %q is not in the suite", name)
		}
	}

	var errs []*ctsTestCaseError
	countIgnored := 0
	for _, tc := range testcases {
		if slices.Contains(ignoredTests, tc.name) {
			countIgnored++
			continue
		}
		if err := tc.run(); err != nil {
			errs = append(errs, err)
		}
	}

	countFailed := len(errs)
	countPassed := countTotal - countIgnored - countFailed
	summary := fmt.Sprintf("RFC9535 Compliance tests:\nTotal:  %d\nPassed: %d\nFailed: %d\nIgnore: %d\n",
		countTotal, countPassed, countFailed, countIgnored)
	if countFailed > 0 {
		var s strings.Builder
		for _, err := range errs {
			s.WriteString(err.String())
			s.WriteString("\n\n")
		}
		s.WriteString(summary)
		t.Fatal(s.String())
	}
	t.Log(summary)
}
