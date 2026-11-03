package runner

import (
	"reflect"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/httpclient/httpfixture"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func simpleKeyValue(key, value ast.Template) ast.KeyValue {
	return ast.KeyValue{Space0: space(), Key: key, Space1: space(), Space2: space(), Value: value}
}

// helloRequest is `GET {{base_url}}/hello`.
func helloRequest() *ast.Request {
	return &ast.Request{
		Method: "GET",
		URL: ast.Template{
			Elements: []ast.TemplateElement{
				ast.TemplatePlaceholder{Value: placeholder("base_url", sourceInfo(1, 7, 1, 15))},
				ast.TemplateString{Value: "/hello", Source: "/hello"},
			},
			SourceInfo: sourceInfo(0, 0, 0, 0),
		},
	}
}

func queryRequest() *ast.Request {
	param1 := ast.Template{Elements: []ast.TemplateElement{ast.TemplatePlaceholder{Value: placeholder("param1", sourceInfo(1, 7, 1, 15))}}}
	return &ast.Request{
		Method: "GET",
		URL:    templateString("http://localhost:8000/querystring-params", sourceInfo(0, 0, 0, 0)),
		Sections: []ast.Section{{Value: ast.SectionQueryParams{Value: []ast.KeyValue{
			simpleKeyValue(templateString("param1", sourceInfo(0, 0, 0, 0)), param1),
			simpleKeyValue(templateString("param2", sourceInfo(0, 0, 0, 0)), templateString("a b", sourceInfo(0, 0, 0, 0))),
		}}}},
	}
}

func TestEvalRequestErrorVariable(t *testing.T) {
	_, err := evalRequest(helloRequest(), NewVariableSet(), util.ContextDir{})
	if err == nil || err.SourceInfo() != sourceInfo(1, 7, 1, 15) || err.Kind != (RunnerErrorTemplateVariableNotDefined{Name: "base_url"}) {
		t.Errorf("err = %#v", err)
	}
}

func TestEvalHelloRequest(t *testing.T) {
	vars := NewVariableSet()
	vars.Insert("base_url", NewString("http://localhost:8000"))
	got, err := evalRequest(helloRequest(), vars, util.ContextDir{})
	if err != nil || !reflect.DeepEqual(got, httpfixture.HelloHttpRequest()) {
		t.Errorf("got %#v, %v", got, err)
	}
}

func TestEvalQueryRequest(t *testing.T) {
	vars := NewVariableSet()
	vars.Insert("param1", NewString("value1"))
	got, err := evalRequest(queryRequest(), vars, util.ContextDir{})
	if err != nil || !reflect.DeepEqual(got, httpfixture.QueryHttpRequest()) {
		t.Errorf("got %#v, %v", got, err)
	}
}

func TestEvalRequestInvalidURL(t *testing.T) {
	r := &ast.Request{Method: "GET", URL: templateString("localhost", sourceInfo(1, 5, 1, 14))}
	_, err := evalRequest(r, NewVariableSet(), util.ContextDir{})
	if err == nil || err.SourceInfo() != sourceInfo(1, 5, 1, 14) {
		t.Fatalf("err = %#v", err)
	}
	if k, ok := err.Kind.(RunnerErrorInvalidURL); !ok || k.URL != "localhost" {
		t.Errorf("kind = %#v", err.Kind)
	}
}

func TestEvalRequestBasicAuthAndContentType(t *testing.T) {
	tmpl := func(s string) ast.Template { return templateString(s, sourceInfo(0, 0, 0, 0)) }
	r := &ast.Request{
		Method: "POST",
		URL:    tmpl("http://localhost/form"),
		Sections: []ast.Section{
			{Value: ast.SectionBasicAuth{Value: &ast.KeyValue{Key: tmpl("bob"), Value: tmpl("secret")}}},
			{Value: ast.SectionFormParams{Value: []ast.KeyValue{simpleKeyValue(tmpl("a"), tmpl("b"))}}},
		},
	}
	got, err := evalRequest(r, NewVariableSet(), util.ContextDir{})
	if err != nil {
		t.Fatal(err)
	}
	if h, _ := got.Headers.Get("Authorization"); h.Value != "Basic Ym9iOnNlY3JldA==" {
		t.Errorf("authorization = %q", h.Value)
	}
	if got.ImplicitContentType != "application/x-www-form-urlencoded" || !reflect.DeepEqual(got.Form, []httpclient.Param{httpclient.NewParam("a", "b")}) {
		t.Errorf("got %#v", got)
	}
	if implicitBodyContentType(ast.BytesJSON{}) != "application/json" || implicitBodyContentType(ast.BytesXML{}) != "application/xml" || implicitBodyContentType(ast.BytesOnelineString{}) != "" {
		t.Error("implicit body content type")
	}
}

func requestWithComment(comment string) *ast.Request {
	r := helloRequest()
	r.LineTerminators = []ast.LineTerminator{{Space0: space(), Comment: &ast.Comment{Value: comment}, Newline: space()}}
	return r
}

func TestCmdCookieStorageClear(t *testing.T) {
	if cmdCookieStorageClear(helloRequest()) || !cmdCookieStorageClear(requestWithComment("@cookie_storage_clear")) {
		t.Error("cookie storage clear")
	}
}

func TestCmdCookieStorageSet(t *testing.T) {
	if _, ok := cmdCookieStorageSet(helloRequest()); ok {
		t.Error("no comment")
	}
	got, ok := cmdCookieStorageSet(requestWithComment("@cookie_storage_set: localhost\tFALSE\t/\tFALSE\t0\tcookie1\tvalueA"))
	if !ok || got != "localhost\tFALSE\t/\tFALSE\t0\tcookie1\tvalueA" {
		t.Errorf("got %q", got)
	}
}

func filenameValue(filename string, contentType *ast.Template) ast.FilenameValue {
	return ast.FilenameValue{Space0: space(), Filename: templateString(filename, sourceInfo(1, 1, 1, 9)), Space1: space(), Space2: space(), ContentType: contentType}
}

func TestEvalFileParam(t *testing.T) {
	param := ast.FilenameParam{Key: templateString("upload1", sourceInfo(0, 0, 0, 0)), Value: filenameValue("testdata/hello.txt", nil)}
	got, err := evalFileParam(param, util.NewContextDir("/home", ""), NewVariableSet())
	want := httpclient.FileParam{Name: "upload1", Filename: "testdata/hello.txt", Data: []byte("Hello World!"), ContentType: "text/plain"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, %v", got, err)
	}
}

func TestFileValueContentType(t *testing.T) {
	vars := NewVariableSet()
	vars.Insert("ct", NewString("application/json"))
	textHTML := templateString("text/html", sourceInfo(0, 0, 0, 0))
	ct := ast.Template{
		Elements:   []ast.TemplateElement{ast.TemplatePlaceholder{Value: placeholder("ct", sourceInfo(1, 11, 1, 13))}},
		SourceInfo: sourceInfo(1, 9, 1, 15),
	}
	tests := []struct {
		value ast.FilenameValue
		want  string
	}{
		{filenameValue("hello.txt", nil), "text/plain"},
		{filenameValue("hello.html", nil), "text/html"},
		{filenameValue("hello.txt", &textHTML), "text/html"},
		{filenameValue("hello", nil), "application/octet-stream"},
		{filenameValue("hello.txt", &ct), "application/json"},
		{filenameValue(".png", nil), "application/octet-stream"},
		{filenameValue("dir.d/image.jpeg", nil), "image/jpeg"},
	}
	for _, tt := range tests {
		got, err := evalContentType(tt.value, vars)
		if err != nil || got != tt.want {
			t.Errorf("%s: got %q, %v; want %q", tt.value.Filename, got, err, tt.want)
		}
	}
}
