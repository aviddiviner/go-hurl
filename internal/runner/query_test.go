package runner

import (
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/httpclient/httpfixture"
)

func defaultHTTPResponse() httpclient.Response {
	u, _ := httpclient.ParseUrl("http://localhost")
	return httpclient.Response{Version: httpclient.HttpVersionHttp10, Status: 200, Url: u}
}

func responsesOf(rs ...httpclient.Response) []*httpclient.Response {
	ptrs := make([]*httpclient.Response, len(rs))
	for i := range rs {
		ptrs[i] = &rs[i]
	}
	return ptrs
}

func quotedTemplateAt(s string, si ast.SourceInfo) ast.Template {
	tmpl := templateString(s, si)
	tmpl.Delimiter = '"'
	return tmpl
}

// xpathInvalidQuery is `xpath "???"`.
func xpathInvalidQuery() ast.Query {
	return ast.Query{SourceInfo: sourceInfo(1, 1, 1, 13), Value: ast.QueryXPath{Expr: quotedTemplateAt("???", sourceInfo(1, 7, 1, 10))}}
}

func xpathCountUserQuery() ast.Query {
	return ast.Query{SourceInfo: sourceInfo(1, 1, 1, 13), Value: ast.QueryXPath{Expr: quotedTemplateAt("count(//user)", sourceInfo(0, 0, 0, 0))}}
}

func xpathUsersQuery() ast.Query {
	return ast.Query{SourceInfo: sourceInfo(1, 1, 1, 13), Value: ast.QueryXPath{Expr: quotedTemplateAt("//user", sourceInfo(0, 0, 0, 0))}}
}

func jsonpathQuery(expr string, si ast.SourceInfo) ast.Query {
	return ast.Query{SourceInfo: sourceInfo(1, 1, 1, 19), Value: ast.QueryJSONPath{Expr: quotedTemplateAt(expr, si)}}
}

func jsonpathSuccessQuery() ast.Query  { return jsonpathQuery("$.success", sourceInfo(1, 10, 1, 19)) }
func jsonpathErrorsQuery() ast.Query   { return jsonpathQuery("$.errors", sourceInfo(1, 10, 1, 18)) }
func jsonpathDurationQuery() ast.Query { return jsonpathQuery("$.duration", sourceInfo(1, 10, 1, 18)) }

func regexQuery(pattern string, si ast.SourceInfo) ast.Query {
	return ast.Query{SourceInfo: sourceInfo(1, 1, 1, 26), Value: ast.QueryRegex{Value: ast.RegexTemplate{Value: quotedTemplateAt(pattern, si)}}}
}

func simpleQuery(v ast.QueryValue) ast.Query {
	return ast.Query{SourceInfo: sourceInfo(0, 0, 0, 0), Value: v}
}

func runQuery(q ast.Query, rs ...httpclient.Response) (Value, *RunnerError) {
	return evalQuery(q, NewVariableSet(), responsesOf(rs...), &bodyCache{}, DefaultQueryOptions())
}

func wantQueryValue(t *testing.T, name string, q ast.Query, r httpclient.Response, want Value) {
	t.Helper()
	got, err := runQuery(q, r)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %#v, %v; want %#v", name, got, err, want)
	}
}

func wantQueryError(t *testing.T, name string, q ast.Query, r httpclient.Response, wantSI ast.SourceInfo, wantKind RunnerErrorKind) {
	t.Helper()
	_, err := runQuery(q, r)
	if err == nil || !reflect.DeepEqual(err.Kind, wantKind) || err.SourceInfo().Start != wantSI.Start {
		t.Errorf("%s: err = %#v; want %#v at %v", name, err, wantKind, wantSI.Start)
	}
}

func TestQueryStatus(t *testing.T) {
	wantQueryValue(t, "status", simpleQuery(ast.QueryStatus{}), httpfixture.HelloHttpResponse(), NewInteger(200))
}

func TestQueryHeader(t *testing.T) {
	header := func(name string) ast.Query {
		return simpleQuery(ast.QueryHeader{Name: quotedTemplateAt(name, sourceInfo(2, 8, 2, 14))})
	}
	wantQueryValue(t, "not found", header("Custom"), httpfixture.HelloHttpResponse(), nil)
	wantQueryValue(t, "content-type", header("Content-Type"), httpfixture.HelloHttpResponse(), NewString("text/html; charset=utf-8"))
}

func TestQueryCookie(t *testing.T) {
	response := defaultHTTPResponse()
	response.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("Set-Cookie", "LSID=DQAAAKEaem_vYg; Path=/accounts; Expires=Wed, 13 Jan 2021 22:23:01 GMT; Secure; HttpOnly"))
	cookie := func(kind *ast.CookieAttributeKind, value string) ast.Query {
		path := ast.CookiePath{Name: quotedTemplate("LSID")}
		if kind != nil {
			path.Attribute = &ast.CookieAttribute{Name: ast.CookieAttributeName{Kind: *kind, Value: value}}
		}
		return simpleQuery(ast.QueryCookie{Expr: path})
	}
	kind := func(k ast.CookieAttributeKind) *ast.CookieAttributeKind { return &k }
	wantQueryValue(t, "value", cookie(nil, ""), response, NewString("DQAAAKEaem_vYg"))
	wantQueryValue(t, "path", cookie(kind(ast.CookieAttributePath), "Path"), response, NewString("/accounts"))
	wantQueryValue(t, "secure", cookie(kind(ast.CookieAttributeSecure), "Secure"), response, ValueUnit{})
	wantQueryValue(t, "domain", cookie(kind(ast.CookieAttributeDomain), "Domain"), response, nil)
}

func TestEvalCookieAttributeName(t *testing.T) {
	str := func(s string) *string { return &s }
	cookie := httpclient.ResponseCookie{
		Name:  "LSID",
		Value: "DQAAAKEaem_vYg",
		Attributes: []httpclient.CookieAttribute{
			{Name: "Path", Value: str("/accounts")},
			{Name: "Expires", Value: str("Wed, 13 Jan 2021 22:23:01 GMT")},
			{Name: "Secure"},
			{Name: "HttpOnly"},
		},
	}
	tests := []struct {
		kind ast.CookieAttributeKind
		want Value
	}{
		{ast.CookieAttributeValue, NewString("DQAAAKEaem_vYg")},
		{ast.CookieAttributeDomain, nil},
		{ast.CookieAttributePath, NewString("/accounts")},
		{ast.CookieAttributeMaxAge, nil},
		{ast.CookieAttributeExpires, ValueDate{time.Date(2021, 1, 13, 22, 23, 1, 0, time.UTC)}},
		{ast.CookieAttributeSecure, ValueUnit{}},
		{ast.CookieAttributeHTTPOnly, ValueUnit{}},
		{ast.CookieAttributeSameSite, nil},
	}
	for _, tt := range tests {
		got, err := evalCookieAttributeName(ast.CookieAttributeName{Kind: tt.kind, Value: "_"}, cookie, sourceInfo(0, 0, 0, 0))
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%v: got %#v, %v; want %#v", tt.kind, got, err, tt.want)
		}
	}
}

func TestEvalCookieAttributeExpiresError(t *testing.T) {
	str := "???"
	cookie := httpclient.ResponseCookie{Name: "cookie1", Value: "value1", Attributes: []httpclient.CookieAttribute{{Name: "Expires", Value: &str}}}
	_, err := evalCookieAttributeName(ast.CookieAttributeName{Kind: ast.CookieAttributeExpires, Value: "_"}, cookie, sourceInfo(5, 1, 5, 20))
	want := RunnerErrorHTTP{Value: httpclient.HttpErrorCouldNotParseCookieExpires{Value: "???"}}
	if err == nil || err.SourceInfo() != sourceInfo(5, 1, 5, 20) || err.Kind != want {
		t.Errorf("err = %#v", err)
	}
}

func TestParseCookieExpires(t *testing.T) {
	want := time.Date(2021, 1, 13, 22, 23, 1, 0, time.UTC)
	for _, s := range []string{"Wed, 13 Jan 2021 22:23:01 GMT", "Wed, 13-Jan-2021 22:23:01 GMT", "Wed, 13 Jan 2021 23:23:01 +0100"} {
		if got, ok := parseCookieExpires(s); !ok || !got.Equal(want) {
			t.Errorf("parseCookieExpires(%q) = %v, %v", s, got, ok)
		}
	}
}

func TestQueryBody(t *testing.T) {
	wantQueryValue(t, "body", simpleQuery(ast.QueryBody{}), httpfixture.HelloHttpResponse(), NewString("Hello World!"))
	q := ast.Query{SourceInfo: sourceInfo(1, 1, 1, 2), Value: ast.QueryBody{}}
	wantQueryError(t, "invalid decoding", q, httpfixture.BytesHttpResponse(), sourceInfo(1, 1, 1, 2), RunnerErrorHTTP{Value: httpclient.HttpErrorInvalidDecoding{Charset: "UTF-8"}})
}

func TestQueryInvalidUTF8(t *testing.T) {
	r := defaultHTTPResponse()
	r.Body = []byte{200}
	wantQueryError(t, "xpath", xpathUsersQuery(), r, sourceInfo(1, 1, 1, 1), RunnerErrorHTTP{Value: httpclient.HttpErrorInvalidDecoding{Charset: "UTF-8"}})
}

func TestQueryXPathErrorEval(t *testing.T) {
	q := simpleQuery(ast.QueryXPath{Expr: quotedTemplateAt("^^^", sourceInfo(1, 7, 1, 10))})
	wantQueryError(t, "xpath", q, httpfixture.XmlTwoUsersHttpResponse(), sourceInfo(1, 7, 1, 7), RunnerErrorInvalidXPathEval{})
}

func TestQueryXPath(t *testing.T) {
	wantQueryValue(t, "users", xpathUsersQuery(), httpfixture.XmlTwoUsersHttpResponse(), ValueNodeset{2})
	wantQueryValue(t, "count", xpathCountUserQuery(), httpfixture.XmlTwoUsersHttpResponse(), NewFloat(2))
}

func TestQueryXPathWithHTML(t *testing.T) {
	q := ast.Query{SourceInfo: sourceInfo(1, 1, 1, 13), Value: ast.QueryXPath{Expr: quotedTemplate("normalize-space(/html/head/meta/@charset)")}}
	wantQueryValue(t, "charset", q, httpfixture.HtmlHttpResponse(), NewString("UTF-8"))
}

func TestQueryJSONPathInvalidExpression(t *testing.T) {
	q := simpleQuery(ast.QueryJSONPath{Expr: quotedTemplateAt("xxx", sourceInfo(1, 10, 1, 13))})
	wantQueryError(t, "jsonpath", q, httpfixture.JsonHttpResponse(), sourceInfo(1, 10, 1, 10), RunnerErrorQueryInvalidJSONPathExpression{Value: "xxx"})
}

func TestQueryInvalidJSON(t *testing.T) {
	r := defaultHTTPResponse()
	r.Body = []byte("xxx")
	wantQueryError(t, "jsonpath", jsonpathSuccessQuery(), r, sourceInfo(1, 1, 1, 1), RunnerErrorQueryInvalidJSON{})
}

func TestQueryJSONNotFound(t *testing.T) {
	r := defaultHTTPResponse()
	r.Body = []byte("{}")
	wantQueryValue(t, "not found", jsonpathSuccessQuery(), r, nil)
}

func TestQueryJSON(t *testing.T) {
	wantQueryValue(t, "success", jsonpathSuccessQuery(), httpfixture.JsonHttpResponse(), NewBool(false))
	want := ValueList{[]Value{
		ValueObject{[]KeyValue{{Key: "id", Value: NewString("error1")}}},
		ValueObject{[]KeyValue{{Key: "id", Value: NewString("error2")}}},
	}}
	wantQueryValue(t, "errors", jsonpathErrorsQuery(), httpfixture.JsonHttpResponse(), want)
}

func TestQueryRegex(t *testing.T) {
	wantQueryValue(t, "name", regexQuery("Hello ([a-zA-Z]+)!", sourceInfo(1, 7, 1, 26)), httpfixture.HelloHttpResponse(), NewString("World"))
	_, err := runQuery(regexQuery("???", sourceInfo(1, 7, 1, 10)), httpfixture.HelloHttpResponse())
	if err == nil || err.SourceInfo() != sourceInfo(1, 7, 1, 10) || err.Kind != (RunnerErrorInvalidRegex{}) {
		t.Errorf("err = %#v", err)
	}
}

func TestQueryBytes(t *testing.T) {
	wantQueryValue(t, "bytes", simpleQuery(ast.QueryBytes{}), httpfixture.HelloHttpResponse(), ValueBytes{[]byte("Hello World!")})
	wantQueryValue(t, "rawbytes", simpleQuery(ast.QueryRawBytes{}), httpfixture.HelloHttpResponse(), ValueBytes{[]byte("Hello World!")})
}

func TestQuerySHA256(t *testing.T) {
	r := defaultHTTPResponse()
	r.Body = []byte{0xff}
	want, _ := hex.DecodeString("a8100ae6aa1940d0b663bb31cd466142ebbdbd5187131b92d93818987832eb89")
	wantQueryValue(t, "sha256", simpleQuery(ast.QuerySHA256{}), r, ValueBytes{want})
}

func TestQueryCertificate(t *testing.T) {
	r := defaultHTTPResponse()
	if v := evalQueryCertificate(&r, ast.CertificateAttributeSubject); v != nil {
		t.Errorf("no certificate: got %#v", v)
	}
	str := func(s string) *string { return &s }
	r.Certificate = httpclient.NewCertificate(str("A=B, C=D"), str(""), nil, nil, str(""), str(""), nil)
	if v := evalQueryCertificate(&r, ast.CertificateAttributeSubject); !Equal(v, NewString("A=B, C=D")) {
		t.Errorf("subject: got %#v", v)
	}
}

func TestQueryRedirects(t *testing.T) {
	r1, r2, r3 := defaultHTTPResponse(), defaultHTTPResponse(), defaultHTTPResponse()
	r1.Status, r2.Status = 301, 302
	r2.Url, _ = httpclient.ParseUrl("http://localhost/b")
	r3.Url, _ = httpclient.ParseUrl("http://localhost/c")
	got, err := runQuery(simpleQuery(ast.QueryRedirects{}), r1, r2, r3)
	if err != nil {
		t.Fatal(err)
	}
	list := got.(ValueList).Value
	if len(list) != 2 || list[0].String() != "Response(location=http://localhost/b, status=301)" || list[1].String() != "Response(location=http://localhost/c, status=302)" {
		t.Errorf("got %v", got)
	}
}
