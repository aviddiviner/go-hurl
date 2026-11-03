package runner

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func testContextDir() util.ContextDir { return util.NewContextDir("/home", "file_root") }

func intPredicateValue(n int64) ast.PredicateValue {
	return ast.PredicateNumber{Value: ast.NumberInteger{Value: ast.I64{Value: n}}}
}

func floatPredicateValue(f float64) ast.PredicateValue {
	return ast.PredicateNumber{Value: ast.NumberFloat{Value: ast.Float{Value: f}}}
}

func space() ast.Whitespace { return ast.Whitespace{Value: " ", SourceInfo: sourceInfo(0, 0, 0, 0)} }

func predicate(not bool, f ast.PredicateFuncValue, si ast.SourceInfo) ast.Predicate {
	return ast.Predicate{Not: not, Space0: space(), PredicateFunc: ast.PredicateFunc{SourceInfo: si, Value: f}}
}

func wantAssertFailure(t *testing.T, err *RunnerError, actual, expected string, typeMismatch bool) {
	t.Helper()
	want := RunnerErrorAssertFailure{Actual: actual, Expected: expected, TypeMismatch: typeMismatch}
	if err == nil || err.Kind != want || !err.Assert {
		t.Errorf("err = %#v; want %#v", err, want)
	}
}

func wantResult(t *testing.T, got predicateResult, success, typeMismatch bool, actual, expected string) {
	t.Helper()
	want := predicateResult{success: success, typeMismatch: typeMismatch, actual: actual, expected: expected}
	if got != want {
		t.Errorf("got %#v; want %#v", got, want)
	}
}

func TestPredicate(t *testing.T) {
	vars := NewVariableSet()
	p := ast.Predicate{
		Not:           true,
		Space0:        ast.Whitespace{Value: " ", SourceInfo: sourceInfo(1, 1, 0, 0)},
		PredicateFunc: ast.PredicateFunc{SourceInfo: sourceInfo(1, 11, 1, 12), Value: ast.PredicateFuncEqual{Space0: space(), Value: intPredicateValue(10)}},
	}
	if err := evalPredicate(p, vars, NewBool(true), testContextDir()); err != nil {
		t.Errorf("bool: %v", err)
	}
	err := evalPredicate(p, vars, NewInteger(10), testContextDir())
	wantAssertFailure(t, err, "integer <10>", "not integer <10>", false)
	if err.SourceInfo() != sourceInfo(1, 0, 1, 0) {
		t.Errorf("source info = %v", err.SourceInfo())
	}
	if err := evalPredicate(p, vars, NewInteger(1), testContextDir()); err != nil {
		t.Errorf("1: %v", err)
	}
}

func TestPredicateNotEqualErrorMessage(t *testing.T) {
	vars := NewVariableSet()
	f := ast.PredicateFuncNotEqual{Space0: space(), Value: intPredicateValue(1)}
	p := predicate(false, f, sourceInfo(1, 11, 1, 13))
	wantAssertFailure(t, evalPredicate(p, vars, NewInteger(1), testContextDir()), "integer <1>", "not integer <1>", false)
	wantAssertFailure(t, evalPredicate(p, vars, nil, testContextDir()), "none", "not integer <1>", false)

	p = predicate(true, f, sourceInfo(1, 11, 1, 13))
	wantAssertFailure(t, evalPredicate(p, vars, NewInteger(2), testContextDir()), "integer <2>", "integer <1>", false)
}

func evalEqual(t *testing.T, expected ast.PredicateValue, vars *VariableSet, value Value) predicateResult {
	t.Helper()
	got, err := evalPredicateFunc(ast.PredicateFunc{Value: ast.PredicateFuncEqual{Value: expected}}, vars, value, testContextDir())
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestPredicateEqual(t *testing.T) {
	vars := NewVariableSet()
	tests := []struct {
		expected         ast.PredicateValue
		value            Value
		success          bool
		actual, wantExpd string
	}{
		{intPredicateValue(10), NewBool(true), false, "boolean <true>", "integer <10>"},
		{intPredicateValue(10), ValueUnit{}, false, "unit", "integer <10>"},
		{intPredicateValue(10), NewInteger(1), false, "integer <1>", "integer <10>"},
		{ast.PredicateBool{Value: true}, NewBool(false), false, "boolean <false>", "boolean <true>"},
		{floatPredicateValue(1.2), NewFloat(1.1), false, "float <1.1>", "float <1.2>"},
		{intPredicateValue(1), NewInteger(1), true, "integer <1>", "integer <1>"},
		{ast.PredicateBool{Value: false}, NewBool(false), true, "boolean <false>", "boolean <false>"},
		{ast.PredicateBool{Value: true}, NewBool(true), true, "boolean <true>", "boolean <true>"},
		{floatPredicateValue(1.1), NewFloat(1.1), true, "float <1.1>", "float <1.1>"},
		{intPredicateValue(1), NewFloat(1.0), true, "float <1.0>", "integer <1>"},
		{intPredicateValue(1), NewInteger(2), false, "integer <2>", "integer <1>"},
	}
	for _, tt := range tests {
		wantResult(t, evalEqual(t, tt.expected, vars, tt.value), tt.success, false, tt.actual, tt.wantExpd)
	}
}

func TestPredicateExist(t *testing.T) {
	pf := ast.PredicateFunc{Value: ast.PredicateFuncExist{}}
	got, _ := evalPredicateFunc(pf, NewVariableSet(), ValueUnit{}, testContextDir())
	wantResult(t, got, true, false, "unit", "something")
	got, _ = evalPredicateFunc(pf, NewVariableSet(), nil, testContextDir())
	wantResult(t, got, false, false, "none", "something")
}

func TestPredicateValueEqualsString(t *testing.T) {
	tmpl := ast.Template{
		Delimiter:  '"',
		Elements:   []ast.TemplateElement{ast.TemplatePlaceholder{Value: placeholder("base_url", sourceInfo(1, 11, 1, 19))}},
		SourceInfo: sourceInfo(1, 1, 1, 1),
	}
	expected := ast.PredicateString{Value: tmpl}
	value := NewString("http://localhost:8000")
	_, err := evalPredicateFunc(ast.PredicateFunc{Value: ast.PredicateFuncEqual{Value: expected}}, NewVariableSet(), value, testContextDir())
	if err == nil || err.Kind != (RunnerErrorTemplateVariableNotDefined{Name: "base_url"}) || err.SourceInfo() != sourceInfo(1, 11, 1, 19) {
		t.Errorf("err = %#v", err)
	}
	vars := NewVariableSet()
	vars.Insert("base_url", NewString("http://localhost:8000"))
	wantResult(t, evalEqual(t, expected, vars, value), true, false, "string <http://localhost:8000>", "string <http://localhost:8000>")
}

func TestAssertValueGreater(t *testing.T) {
	wantResult(t, assertValuesGreater(NewInteger(2), NewInteger(1)), true, false, "integer <2>", "greater than integer <1>")
	wantResult(t, assertValuesGreater(NewInteger(1), NewInteger(1)), false, false, "integer <1>", "greater than integer <1>")
	wantResult(t, assertValuesGreater(NewFloat(1.1), NewInteger(1)), true, false, "float <1.1>", "greater than integer <1>")
	wantResult(t, assertValuesGreater(NewFloat(1.1), NewInteger(2)), false, false, "float <1.1>", "greater than integer <2>")
}

func TestPredicateIsEmpty(t *testing.T) {
	wantResult(t, evalIsEmpty(ValueList{[]Value{NewInteger(1)}}), false, false, "count equals to 1", "count equals to 0")
	wantResult(t, evalIsEmpty(ValueNodeset{12}), false, false, "count equals to 12", "count equals to 0")
	wantResult(t, evalIsEmpty(ValueList{}), true, false, "count equals to 0", "count equals to 0")
	wantResult(t, evalIsEmpty(ValueNodeset{0}), true, false, "count equals to 0", "count equals to 0")
}

func TestPredicateType(t *testing.T) {
	wantResult(t, evalIsInteger(NewInteger(1)), true, false, "integer <1>", "integer")
	wantResult(t, evalIsInteger(NewFloat(1.0)), false, false, "float <1.0>", "integer")
}

func TestPredicateNotWithDifferentTypes(t *testing.T) {
	vars := NewVariableSet()
	p := predicate(true, ast.PredicateFuncEqual{Space0: space(), Value: ast.PredicateNull{}}, sourceInfo(0, 0, 0, 0))
	if err := evalPredicate(p, vars, NewInteger(1), testContextDir()); err != nil {
		t.Errorf("null: %v", err)
	}
	p = predicate(true, ast.PredicateFuncStartWith{Space0: space(), Value: ast.PredicateString{Value: templateString("toto", sourceInfo(0, 0, 0, 0))}}, sourceInfo(0, 0, 0, 0))
	wantAssertFailure(t, evalPredicate(p, vars, NewInteger(1), testContextDir()), "integer <1>", "not starts with string <toto>", true)
}

func TestDatePredicate(t *testing.T) {
	wantResult(t, evalIsDate(ValueDate{time.Date(2002, 6, 16, 10, 10, 10, 0, time.UTC)}), true, false, "date <2002-06-16 10:10:10 UTC>", "date")
	wantResult(t, evalIsDate(NewString("toto")), false, false, "string <toto>", "date")
}

func TestNoTypeMismatchWithNoneValue(t *testing.T) {
	vars := NewVariableSet()
	p := predicate(false, ast.PredicateFuncEqual{Space0: space(), Value: ast.PredicateNull{}}, sourceInfo(0, 0, 0, 0))
	wantAssertFailure(t, evalPredicate(p, vars, nil, testContextDir()), "none", "null", false)
	p.Not = true
	if err := evalPredicate(p, vars, nil, testContextDir()); err != nil {
		t.Errorf("not: %v", err)
	}
}

func TestPredicateMatch(t *testing.T) {
	expected := ast.PredicateRegex{Value: ast.Regex{Inner: regexp.MustCompile(`a{3}`), Source: "/a{3}/"}}
	got, err := evalMatch(expected, sourceInfo(0, 0, 0, 0), NewVariableSet(), NewString("aa"), testContextDir())
	if err != nil {
		t.Fatal(err)
	}
	wantResult(t, got, false, false, "string <aa>", "matches regex </a{3}/>")
}

func TestPredicateIsISODate(t *testing.T) {
	wantResult(t, evalIsISODate(NewString("2020-03-09T22:18:26.625Z")), true, false, "2020-03-09T22:18:26.625Z", "string with format YYYY-MM-DDTHH:mm:ss.sssZ")
}

func TestPredicateIsNumber(t *testing.T) {
	wantResult(t, evalIsNumber(NewInteger(1)), true, false, "integer <1>", "number")
	wantResult(t, evalIsNumber(NewFloat(1.0)), true, false, "float <1.0>", "number")
}

func TestExpectedNoValue(t *testing.T) {
	tests := []struct {
		f    ast.PredicateFuncValue
		want string
	}{
		{ast.PredicateFuncGreaterThan{Value: floatPredicateValue(1)}, "greater than <float <1.0>>"},
		{ast.PredicateFuncContain{Value: ast.PredicateHex{Value: ast.Hex{Value: []byte{1, 2}}}}, "contains 2 bytes"},
		{ast.PredicateFuncMatch{Value: ast.PredicateString{Value: templateString("a+", sourceInfo(0, 0, 0, 0))}}, "matches regex <a+>"},
		{ast.PredicateFuncIsUUID{}, "uuid"},
	}
	for _, tt := range tests {
		got, err := expectedNoValue(tt.f, NewVariableSet(), testContextDir())
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("got %q, %v; want %q", got, err, tt.want)
		}
	}
}
