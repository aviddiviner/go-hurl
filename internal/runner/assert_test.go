package runner

import (
	"reflect"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/httpclient/httpfixture"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// assertCountUser is `xpath "//user" count == 3`.
func assertCountUser() ast.Assert {
	ws := ast.Whitespace{Value: " ", SourceInfo: sourceInfo(1, 1, 1, 1)}
	return ast.Assert{
		Space0:  ws,
		Query:   xpathUsersQuery(),
		Filters: []ast.SpacedFilter{{Space0: ws, Filter: ast.Filter{SourceInfo: sourceInfo(1, 16, 1, 21), Value: ast.FilterCount{}}}},
		Space1:  ws,
		Predicate: ast.Predicate{
			Space0:        ws,
			PredicateFunc: ast.PredicateFunc{SourceInfo: sourceInfo(1, 22, 1, 24), Value: ast.PredicateFuncEqual{Space0: ws, Value: intPredicateValue(3)}},
		},
	}
}

func TestEvalExplicitAssert(t *testing.T) {
	got := evalExplicitAssert(assertCountUser(), NewVariableSet(), responsesOf(httpfixture.XmlThreeUsersHttpResponse()), &bodyCache{}, testContextDir(), DefaultQueryOptions())
	want := AssertResultExplicit{Actual: NewInteger(3), SourceInfo: sourceInfo(1, 22, 1, 24)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestUseDiff(t *testing.T) {
	if useDiff(NewBool(true), NewBool(false)) || useDiff(NewString("a"), NewString("b")) || !useDiff(NewString("a\n"), NewString("b")) {
		t.Error("useDiff")
	}
}

func TestImplicitBodyDiffError(t *testing.T) {
	a := AssertResultImplicitBody{Actual: NewString("a\nc\n"), Expected: NewString("a\nb\n"), SourceInfo: sourceInfo(10, 1, 10, 1)}
	err := a.ToRunnerError()
	if err == nil || err.SourceInfo() != sourceInfo(11, 1, 11, 1) {
		t.Fatalf("err = %#v", err)
	}
	if k, ok := err.Kind.(RunnerErrorAssertBodyDiffError); !ok || k.BodySourceInfo != a.SourceInfo || len(k.Hunks) != 1 {
		t.Errorf("kind = %#v", err.Kind)
	}
}

func userCountCapture() ast.Capture {
	return ast.Capture{Name: templateString("UserCount", sourceInfo(0, 0, 0, 0)), Query: xpathCountUserQuery()}
}

func durationCapture() ast.Capture {
	return ast.Capture{Name: templateString("duration", sourceInfo(0, 0, 0, 0)), Query: jsonpathDurationQuery()}
}

func TestCaptureInvalidXPath(t *testing.T) {
	capture := ast.Capture{Name: templateString("count", sourceInfo(0, 0, 0, 0)), Query: xpathInvalidQuery()}
	_, err := evalCapture(capture, NewVariableSet(), responsesOf(httpfixture.XmlThreeUsersHttpResponse()), &bodyCache{}, DefaultQueryOptions())
	if err == nil || err.SourceInfo().Start != sourceInfo(1, 7, 1, 7).Start || err.Kind != (RunnerErrorInvalidXPathEval{}) {
		t.Errorf("err = %#v", err)
	}
}

func TestCapture(t *testing.T) {
	cache := &bodyCache{}
	got, err := evalCapture(userCountCapture(), NewVariableSet(), responsesOf(httpfixture.XmlThreeUsersHttpResponse()), cache, DefaultQueryOptions())
	if err != nil || !reflect.DeepEqual(got, CaptureResult{Name: "UserCount", Value: NewFloat(3)}) {
		t.Errorf("user count: got %#v, %v", got, err)
	}
	got, err = evalCapture(durationCapture(), NewVariableSet(), responsesOf(httpfixture.JsonHttpResponse()), cache, DefaultQueryOptions())
	if err != nil || !reflect.DeepEqual(got, CaptureResult{Name: "duration", Value: NewFloat(1.5)}) {
		t.Errorf("duration: got %#v, %v", got, err)
	}
}

func TestCaptureNoResult(t *testing.T) {
	q := simpleQuery(ast.QueryHeader{Name: quotedTemplate("Custom")})
	q.SourceInfo = sourceInfo(2, 3, 2, 18)
	capture := ast.Capture{Name: templateString("x", sourceInfo(0, 0, 0, 0)), Query: q}
	_, err := evalCapture(capture, NewVariableSet(), responsesOf(httpfixture.HelloHttpResponse()), &bodyCache{}, DefaultQueryOptions())
	if err == nil || err.Kind != (RunnerErrorNoQueryResult{}) || err.SourceInfo() != q.SourceInfo {
		t.Errorf("err = %#v", err)
	}
}

// userResponse is:
//
//	HTTP/1.0 200
//	[Asserts]
//	xpath "//user" count == 3
//	[Captures]
//	UserCount: xpath "count(//user)"
func userResponse() *ast.Response {
	return &ast.Response{
		Version: ast.Version{Value: ast.Version1, SourceInfo: sourceInfo(2, 1, 2, 9)},
		Status:  ast.Status{Value: ast.StatusSpecific{Value: 200}, SourceInfo: sourceInfo(2, 10, 2, 13)},
		Sections: []ast.Section{
			{Value: ast.SectionAsserts{Value: []ast.Assert{assertCountUser()}}},
			{Value: ast.SectionCaptures{Value: []ast.Capture{userCountCapture()}}},
		},
	}
}

func TestEvalAsserts(t *testing.T) {
	got := evalAsserts(userResponse(), NewVariableSet(), responsesOf(httpfixture.XmlTwoUsersHttpResponse()), &bodyCache{}, util.ContextDir{}, DefaultQueryOptions())
	want := []AssertResult{AssertResultExplicit{
		Actual:     NewInteger(2),
		SourceInfo: sourceInfo(1, 22, 1, 24),
		PredicateErr: NewRunnerError(sourceInfo(1, 0, 1, 0), RunnerErrorAssertFailure{
			Actual:   "integer <2>",
			Expected: "integer <3>",
		}, true),
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestEvalVersionStatusAsserts(t *testing.T) {
	r := httpfixture.XmlTwoUsersHttpResponse()
	got := evalVersionStatusAsserts(userResponse(), &r)
	want := []AssertResult{
		AssertResultImplicitVersion{Actual: "HTTP/1.0", Expected: "HTTP/1.0", SourceInfo: sourceInfo(2, 1, 2, 9)},
		AssertResultImplicitStatus{Actual: 200, Expected: 200, SourceInfo: sourceInfo(2, 10, 2, 13)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestEvalCaptures(t *testing.T) {
	vars := NewVariableSet()
	got, err := evalCaptures(userResponse(), responsesOf(httpfixture.XmlTwoUsersHttpResponse()), &bodyCache{}, vars, DefaultQueryOptions())
	if err != nil || !reflect.DeepEqual(got, []CaptureResult{{Name: "UserCount", Value: NewFloat(2)}}) {
		t.Errorf("got %#v, %v", got, err)
	}
	if v, ok := vars.Get("UserCount"); !ok || !Equal(v.Value, NewFloat(2)) {
		t.Errorf("variable = %#v", v)
	}
}

func TestEvalImplicitHeaderAsserts(t *testing.T) {
	r := defaultHTTPResponse()
	r.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("X-A", "1"), httpclient.NewHeader("X-A", "2"))
	kv := func(k, v string) ast.KeyValue {
		return ast.KeyValue{Key: templateString(k, sourceInfo(3, 1, 3, 4)), Value: templateString(v, sourceInfo(3, 6, 3, 7))}
	}
	response := &ast.Response{Headers: []ast.KeyValue{kv("X-A", "2"), kv("X-A", "3"), kv("X-B", "1")}}
	got := evalAsserts(response, NewVariableSet(), responsesOf(r), &bodyCache{}, util.ContextDir{}, DefaultQueryOptions())
	want := []AssertResult{
		AssertResultImplicitHeader{Actual: "2", Expected: "2", SourceInfo: sourceInfo(3, 6, 3, 7)},
		AssertResultImplicitHeader{Actual: `["1", "2"]`, Expected: "3", SourceInfo: sourceInfo(3, 6, 3, 7)},
		AssertResultImplicitHeader{ActualErr: NewRunnerError(sourceInfo(3, 1, 3, 4), RunnerErrorQueryHeaderNotFound{}, false), Expected: "1", SourceInfo: sourceInfo(3, 1, 3, 4)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}
