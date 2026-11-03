package runner

import (
	"reflect"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/internal/xpath"
)

func placeholder(name string, si ast.SourceInfo) ast.Placeholder {
	return ast.Placeholder{
		Space0: ast.Whitespace{SourceInfo: si},
		Expr:   variableExpr(name, si),
		Space1: ast.Whitespace{SourceInfo: si},
	}
}

func TestTemplateElement(t *testing.T) {
	variables := NewVariableSet()
	if got, err := evalTemplateElement(ast.TemplateString{Value: "World", Source: "World"}, variables); err != nil || got != "World" {
		t.Errorf("got %q, %v", got, err)
	}
	variables.Insert("name", NewString("World"))
	elem := ast.TemplatePlaceholder{Value: placeholder("name", sourceInfo(1, 3, 1, 7))}
	if got, err := evalTemplateElement(elem, variables); err != nil || got != "World" {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestTemplateElementError(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("name", ValueList{[]Value{NewInteger(1), NewInteger(2)}})
	elem := ast.TemplatePlaceholder{Value: placeholder("name", sourceInfo(1, 3, 1, 7))}
	_, err := evalTemplateElement(elem, variables)
	if err == nil || err.SourceInfo() != sourceInfo(1, 3, 1, 7) || err.Kind != (RunnerErrorUnrenderableExpression{Value: "[1,2]"}) {
		t.Errorf("err = %#v", err)
	}
}

func TestRenderExpression(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("status", NewBool(true))
	expr := variableExpr("status", sourceInfo(0, 0, 0, 0))
	if v, err := evalExpr(expr, variables); err != nil || !Equal(v, NewBool(true)) {
		t.Errorf("evalExpr = %v, %v", v, err)
	}
	if s, err := renderExpr(expr, variables); err != nil || s != "true" {
		t.Errorf("renderExpr = %q, %v", s, err)
	}

	date := time.Date(2023, 1, 10, 8, 29, 52, 0, time.UTC)
	variables.Insert("now", ValueDate{date})
	expr = variableExpr("now", sourceInfo(0, 0, 0, 0))
	if v, err := evalExpr(expr, variables); err != nil || !Equal(v, ValueDate{date}) {
		t.Errorf("evalExpr = %v, %v", v, err)
	}
	if s, err := renderExpr(expr, variables); err != nil || s != "2023-01-10T08:29:52.000000Z" {
		t.Errorf("renderExpr = %q, %v", s, err)
	}
}

func TestEvalFunction(t *testing.T) {
	v, err := evalFunction(ast.FunctionNewUUID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := valueIsUUID(v); !ok || len(v.String()) != 36 || v.String()[14] != '4' {
		t.Errorf("newUuid = %v", v)
	}
	v, _ = evalFunction(ast.FunctionNewDate)
	if d, ok := v.(ValueDate); !ok || time.Since(d.Value) > time.Minute || d.Value.Location() != time.UTC {
		t.Errorf("newDate = %v", v)
	}
}

func jsonHelloWorldValue() ast.JSONValue {
	// "hello\u0020{{name}}!"
	return ast.JSONString{Value: ast.Template{
		Delimiter: '"',
		Elements: []ast.TemplateElement{
			ast.TemplateString{Value: "Hello ", Source: "Hello\\u0020"},
			ast.TemplatePlaceholder{Value: ast.Placeholder{
				Space0: ast.Whitespace{SourceInfo: sourceInfo(1, 15, 1, 15)},
				Expr:   variableExpr("name", sourceInfo(1, 15, 1, 19)),
				Space1: ast.Whitespace{SourceInfo: sourceInfo(1, 19, 1, 19)},
			}},
			ast.TemplateString{Value: "!", Source: "!"},
		},
		SourceInfo: sourceInfo(1, 2, 1, 22),
	}}
}

func jsonPersonValue() ast.JSONValue {
	return ast.JSONObject{
		Space0: "\n    ",
		Elements: []ast.JSONObjectElement{{
			Name:   templateString("firstName", sourceInfo(1, 1, 1, 1)),
			Space2: " ",
			Value:  ast.JSONString{Value: templateString("John", sourceInfo(1, 1, 1, 1))},
			Space3: "\n",
		}},
	}
}

func TestJSONScalarValue(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("name", NewString("Bob"))
	tests := []struct {
		v    ast.JSONValue
		want string
	}{
		{ast.JSONNull{}, "null"},
		{ast.JSONNumber{Value: "3.14"}, "3.14"},
		{ast.JSONBoolean{Value: false}, "false"},
		{jsonHelloWorldValue(), `"Hello\u0020Bob!"`},
	}
	for _, tt := range tests {
		if got, err := evalJSONValue(tt.v, variables, true); err != nil || got != tt.want {
			t.Errorf("evalJSONValue(%v) = %q, %v; want %q", tt.v, got, err, tt.want)
		}
	}
}

func TestJSONError(t *testing.T) {
	_, err := evalJSONValue(jsonHelloWorldValue(), NewVariableSet(), true)
	if err == nil || err.SourceInfo() != sourceInfo(1, 15, 1, 19) || err.Kind != (RunnerErrorTemplateVariableNotDefined{Name: "name"}) {
		t.Errorf("err = %#v", err)
	}
}

func TestJSONListValue(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("name", NewString("Bob"))
	tests := []struct {
		v    ast.JSONValue
		want string
	}{
		{ast.JSONList{}, "[]"},
		{ast.JSONList{Elements: []ast.JSONListElement{
			{Value: ast.JSONNumber{Value: "1"}},
			{Space0: " ", Value: ast.JSONNumber{Value: "-2"}},
			{Space0: " ", Value: ast.JSONNumber{Value: "3.0"}},
		}}, "[1, -2, 3.0]"},
		{ast.JSONList{Elements: []ast.JSONListElement{
			{Value: ast.JSONString{Value: ast.Template{Delimiter: '"', Elements: []ast.TemplateElement{ast.TemplateString{Value: "Hi", Source: "Hi"}}}}},
			{Space0: " ", Value: jsonHelloWorldValue()},
		}}, `["Hi", "Hello\u0020Bob!"]`},
	}
	for _, tt := range tests {
		if got, err := evalJSONValue(tt.v, variables, true); err != nil || got != tt.want {
			t.Errorf("got %q, %v; want %q", got, err, tt.want)
		}
	}
}

func TestJSONObjectValue(t *testing.T) {
	variables := NewVariableSet()
	if got, err := evalJSONValue(ast.JSONObject{}, variables, true); err != nil || got != "{}" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := evalJSONValue(jsonPersonValue(), variables, true); err != nil || got != "{\n    \"firstName\": \"John\"\n}" {
		t.Errorf("got %q, %v", got, err)
	}
	if got, err := evalJSONValue(jsonPersonValue(), variables, false); err != nil || got != `{"firstName":"John"}` {
		t.Errorf("not preserving spaces: got %q, %v", got, err)
	}
}

func TestJSONEscapeSequence(t *testing.T) {
	v := ast.JSONString{Value: ast.Template{Elements: []ast.TemplateElement{ast.TemplateString{Value: "\n", Source: "\\n"}}}}
	if got, err := evalJSONValue(v, NewVariableSet(), true); err != nil || got != `"\n"` {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestEvalJSONTemplate(t *testing.T) {
	tmpl := ast.Template{Elements: []ast.TemplateElement{ast.TemplateString{Value: "\n", Source: "\\n"}}}
	if got, err := evalJSONTemplate(tmpl, NewVariableSet()); err != nil || got != `\n` {
		t.Errorf("got %q, %v", got, err)
	}
	variables := NewVariableSet()
	variables.Insert("quote", NewString(`"`))
	tmpl = ast.Template{Delimiter: '"', Elements: []ast.TemplateElement{
		ast.TemplateString{Value: "Hello ", Source: "Hello "},
		ast.TemplatePlaceholder{Value: placeholder("quote", sourceInfo(0, 0, 0, 0))},
	}}
	if got, err := evalJSONTemplate(tmpl, variables); err != nil || got != `Hello \"` {
		t.Errorf("got %q, %v", got, err)
	}
}

func TestEncodeJSONString(t *testing.T) {
	for in, want := range map[string]string{"a": "a", `"`: `\"`, `\`: `\\`} {
		if got := encodeJSONString(in); got != want {
			t.Errorf("encodeJSONString(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestJSONPlaceholder(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("n", NewInteger(42))
	variables.Insert("s", NewString("abc"))
	p := ast.JSONPlaceholder{Value: placeholder("n", sourceInfo(1, 3, 1, 4))}
	if got, err := evalJSONValue(p, variables, true); err != nil || got != "42" {
		t.Errorf("got %q, %v", got, err)
	}
	p = ast.JSONPlaceholder{Value: placeholder("s", sourceInfo(1, 3, 1, 4))}
	if _, err := evalJSONValue(p, variables, true); err == nil || err.Kind != (RunnerErrorInvalidJSON{Value: "abc"}) {
		t.Errorf("err = %#v", err)
	}
}

const graphqlQuery = `{
  human(id: "1000") {
    name
    height(unit: FOOT)
  }
}`

func TestEvalGraphQLMultilineSimple(t *testing.T) {
	multiline := ast.MultilineString{
		Kind: ast.MultilineStringGraphQL{Value: ast.GraphQL{Value: templateString(graphqlQuery, sourceInfo(0, 0, 0, 0))}},
	}
	got, err := evalMultiline(multiline, NewVariableSet())
	want := `{"query":"{\n  human(id: \"1000\") {\n    name\n    height(unit: FOOT)\n  }\n}"}`
	if err != nil || got != want {
		t.Errorf("got %s, %v; want %s", got, err, want)
	}
}

func TestEvalGraphQLMultilineWithGraphQLVariables(t *testing.T) {
	quoted := func(s string) ast.Template {
		tmpl := templateString(s, sourceInfo(0, 0, 0, 0))
		tmpl.Delimiter = '"'
		return tmpl
	}
	vars := &ast.GraphQLVariables{Value: ast.JSONObject{Elements: []ast.JSONObjectElement{
		{Name: quoted("episode"), Value: ast.JSONString{Value: quoted("JEDI")}},
		{Name: quoted("withFriends"), Value: ast.JSONBoolean{Value: false}},
	}}}
	multiline := ast.MultilineString{
		Kind: ast.MultilineStringGraphQL{Value: ast.GraphQL{Value: templateString(graphqlQuery, sourceInfo(0, 0, 0, 0)), Variables: vars}},
	}
	got, err := evalMultiline(multiline, NewVariableSet())
	want := `{"query":"{\n  human(id: \"1000\") {\n    name\n    height(unit: FOOT)\n  }\n}","variables":{"episode":"JEDI","withFriends":false}}`
	if err != nil || got != want {
		t.Errorf("got %s, %v; want %s", got, err, want)
	}
}

func TestJSONQuote(t *testing.T) {
	for in, want := range map[string]string{"<a&b>": `"<a&b>"`, "\x01\u2028é": `"\u0001` + "\u2028é\"", "\b\f": `"\b\f"`} {
		if got := jsonQuote(in); got != want {
			t.Errorf("jsonQuote(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestBodyFile(t *testing.T) {
	bytes := ast.BytesFile{Value: ast.File{Filename: templateString("testdata/data.bin", sourceInfo(1, 7, 1, 15))}}
	contextDir := util.NewContextDir("/my_home", "")
	got, err := evalBytes(bytes, NewVariableSet(), contextDir)
	want := httpclient.BodyFile{Value: []byte("Hello World!"), Filename: "testdata/data.bin"}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, %v", got, err)
	}
}

func TestBodyFileError(t *testing.T) {
	bytes := ast.BytesFile{Value: ast.File{Filename: templateString("data.bin", sourceInfo(1, 7, 1, 15))}}
	contextDir := util.NewContextDir("/my_home", "file_root")
	_, err := evalBytes(bytes, NewVariableSet(), contextDir)
	if err == nil || err.Kind != (RunnerErrorFileReadAccess{Path: "data.bin"}) || err.SourceInfo() != sourceInfo(1, 7, 1, 15) {
		t.Errorf("err = %#v", err)
	}
}

func TestXPathXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<food>
  <banana type="fruit" price="1.1"/>
  <apple type="fruit"/>
  <beef type="meat"/>
</food>
`
	doc, err := xpath.Parse(xml, xpath.XML)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		expr string
		want Value
	}{
		{"count(//food/*)", NewFloat(3.0)},
		{"//food/*", ValueNodeset{3}},
		{"count(//*[@type='fruit'])", NewFloat(2.0)},
		{"number(//food/banana/@price)", NewFloat(1.1)},
		{"boolean(count(//a[contains(@href,'xxx')]))", NewBool(false)},
	}
	for _, tt := range tests {
		got, err := xpathEval(doc, tt.expr)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("xpathEval(%q) = %#v, %v; want %#v", tt.expr, got, err, tt.want)
		}
	}
	for _, expr := range []string{"^^^", "//", "strong(//head/title)"} {
		if _, err := xpathEval(doc, expr); err != xpath.ErrEval {
			t.Errorf("xpathEval(%q) err = %v", expr, err)
		}
	}
}

func TestXPathCafe(t *testing.T) {
	for _, format := range []xpath.Format{xpath.XML, xpath.HTML} {
		doc, err := xpath.Parse("<data>café</data>", format)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := xpathEval(doc, "normalize-space(//data)"); err != nil || !Equal(got, NewString("café")) {
			t.Errorf("got %v, %v", got, err)
		}
	}
}
