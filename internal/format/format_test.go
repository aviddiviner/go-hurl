package format

import (
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/parser"
	"github.com/aviddiviner/go-hurl/reader"
)

const integrationDir = "../../integration"

func zeroSourceInfo() ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{}, End: reader.Pos{}}
}

func stringTemplate(delimiter rune, value, source string) ast.Template {
	return ast.Template{
		Delimiter:  delimiter,
		Elements:   []ast.TemplateElement{ast.TemplateString{Value: value, Source: source}},
		SourceInfo: zeroSourceInfo(),
	}
}

func TestHTMLMultilineString(t *testing.T) {
	ms := ast.MultilineString{
		Space: ast.Whitespace{Value: "", SourceInfo: ast.SourceInfo{
			Start: reader.Pos{Line: 1, Column: 4}, End: reader.Pos{Line: 1, Column: 4},
		}},
		Newline: ast.Whitespace{Value: "\n", SourceInfo: ast.SourceInfo{
			Start: reader.Pos{Line: 1, Column: 4}, End: reader.Pos{Line: 2, Column: 1},
		}},
		Kind: ast.MultilineStringText{Value: stringTemplate(0, "line1\nline2\n", "line1\nline2\n")},
	}
	f := &htmlFormatter{}
	f.VisitMultilineString(&ms)
	want := "<span class=\"multiline\">```\nline1\nline2\n```</span>"
	if got := f.buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLJSON(t *testing.T) {
	value := ast.JSONObject{
		Space0: "",
		Elements: []ast.JSONObjectElement{{
			Space0: "\n   ",
			Name:   stringTemplate('"', "id", "id"),
			Space1: "",
			Space2: " ",
			Value:  ast.JSONNumber{Value: "1"},
			Space3: "\n",
		}},
	}
	f := &htmlFormatter{}
	f.VisitJSONBody(value)
	want := "<span class=\"json\">{\n   \"id\": 1\n}</span>"
	if got := f.buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLJSONEncodedNewline(t *testing.T) {
	value := ast.JSONString{Value: stringTemplate('"', "\n", `\n`)}
	f := &htmlFormatter{}
	f.VisitJSONBody(value)
	want := "<span class=\"json\">\"\\n\"</span>"
	if got := f.buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLXML(t *testing.T) {
	f := &htmlFormatter{}
	f.VisitXMLBody("<?xml version=\"1.0\"?>\n<drink>café</drink>")
	want := "<span class=\"xml\">&lt;?xml version=\"1.0\"?&gt;\n&lt;drink&gt;café&lt;/drink&gt;</span>"
	if got := f.buf.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestHTMLXMLEscape(t *testing.T) {
	tests := []struct{ in, want string }{
		{"hello", "hello"},
		{"<?xml version=\"1.0\"?>", "&lt;?xml version=\"1.0\"?&gt;"},
	}
	for _, tt := range tests {
		f := &htmlFormatter{}
		f.pushUntrusted(tt.in)
		if got := f.buf.String(); got != tt.want {
			t.Errorf("pushUntrusted(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestTextFormatHurlFile(t *testing.T) {
	src := `
GET https://foo.com
header1: value1
header2: value2
[Form]
foo: bar
baz: 123
HTTP 200
[Asserts]
jsonpath "$.name" == "toto"
`
	file, err := parser.ParseHurlFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := Text(file, false); got != src {
		t.Errorf("plain: got %q, want %q", got, src)
	}
	want := "\n" +
		"\x1b[33mGET\x1b[0m \x1b[32mhttps://foo.com\x1b[0m\n" +
		"\x1b[32mheader1\x1b[0m: \x1b[32mvalue1\x1b[0m\n" +
		"\x1b[32mheader2\x1b[0m: \x1b[32mvalue2\x1b[0m\n" +
		"\x1b[35m[Form]\x1b[0m\n" +
		"\x1b[32mfoo\x1b[0m: \x1b[32mbar\x1b[0m\n" +
		"\x1b[32mbaz\x1b[0m: \x1b[32m123\x1b[0m\n" +
		"HTTP 200\n" +
		"\x1b[35m[Asserts]\x1b[0m\n" +
		"\x1b[36mjsonpath\x1b[0m \x1b[32m\"$.name\"\x1b[0m \x1b[33m==\x1b[0m \x1b[32m\"toto\"\x1b[0m\n"
	if got := Text(file, true); got != want {
		t.Errorf("ansi: got %q, want %q", got, want)
	}
}

func whitespace() ast.Whitespace { return ast.Whitespace{SourceInfo: zeroSourceInfo()} }

func lineTerminator() ast.LineTerminator {
	return ast.LineTerminator{Space0: whitespace(), Newline: whitespace()}
}

func TestJSONRequest(t *testing.T) {
	r := ast.Request{
		Space0:          whitespace(),
		Method:          "GET",
		Space1:          whitespace(),
		URL:             stringTemplate(0, "http://example.com", "not_used"),
		LineTerminator0: lineTerminator(),
		Headers: []ast.KeyValue{{
			Space0:          whitespace(),
			Key:             stringTemplate(0, "Foo", "unused"),
			Space1:          whitespace(),
			Space2:          whitespace(),
			Value:           stringTemplate(0, "Bar", "unused"),
			LineTerminator0: lineTerminator(),
		}},
		SourceInfo: zeroSourceInfo(),
	}
	want := jobject{
		{"method", jstring("GET")},
		{"url", jstring("http://example.com")},
		{"headers", jlist{jobject{{"name", jstring("Foo")}, {"value", jstring("Bar")}}}},
	}
	if got := requestJSON(&r); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestJSONResponse(t *testing.T) {
	r := ast.Response{
		Version:         ast.Version{Value: ast.Version11, SourceInfo: zeroSourceInfo()},
		Space0:          whitespace(),
		Status:          ast.Status{Value: ast.StatusSpecific{Value: 200}, SourceInfo: zeroSourceInfo()},
		Space1:          whitespace(),
		LineTerminator0: lineTerminator(),
		SourceInfo:      zeroSourceInfo(),
	}
	want := jobject{{"version", jstring("HTTP/1.1")}, {"status", jnumber("200")}}
	if got := responseJSON(&r); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}

	r.Version.Value = ast.VersionAny
	r.Status.Value = ast.StatusAny{}
	if got := responseJSON(&r); !reflect.DeepEqual(got, jobject{}) {
		t.Errorf("got %#v, want empty object", got)
	}
}

func headerQuery() ast.Query {
	return ast.Query{
		SourceInfo: zeroSourceInfo(),
		Value: ast.QueryHeader{
			Space0: whitespace(),
			Name:   stringTemplate(0, "Content-Length", "Content-Length"),
		},
	}
}

func equalIntPredicate(value int64, source string) ast.Predicate {
	return ast.Predicate{
		Space0: whitespace(),
		PredicateFunc: ast.PredicateFunc{
			SourceInfo: zeroSourceInfo(),
			Value: ast.PredicateFuncEqual{
				Space0: whitespace(),
				Value:  ast.PredicateNumber{Value: ast.NumberInteger{Value: ast.I64{Value: value, Source: source}}},
			},
		},
	}
}

var headerQueryJSON = jobject{{"type", jstring("header")}, {"name", jstring("Content-Length")}}

func TestJSONQuery(t *testing.T) {
	q := headerQuery()
	if got := queryJSON(&q); !reflect.DeepEqual(got, headerQueryJSON) {
		t.Errorf("got %#v", got)
	}
}

func TestJSONCapture(t *testing.T) {
	c := ast.Capture{
		Space0:          whitespace(),
		Name:            stringTemplate(0, "size", "unused"),
		Space1:          whitespace(),
		Space2:          whitespace(),
		Query:           headerQuery(),
		Space3:          whitespace(),
		LineTerminator0: lineTerminator(),
	}
	want := jobject{{"name", jstring("size")}, {"query", headerQueryJSON}}
	if got := captureJSON(&c); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestJSONPredicate(t *testing.T) {
	p := equalIntPredicate(10, "10")
	want := jobject{{"type", jstring("==")}, {"value", jnumber("10")}}
	if got := predicateJSON(&p); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestJSONAssert(t *testing.T) {
	a := ast.Assert{
		Space0:          whitespace(),
		Query:           headerQuery(),
		Space1:          whitespace(),
		Predicate:       equalIntPredicate(10, "10"),
		LineTerminator0: lineTerminator(),
	}
	want := jobject{
		{"query", headerQueryJSON},
		{"predicate", jobject{{"type", jstring("==")}, {"value", jnumber("10")}}},
	}
	if got := assertJSON(&a); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestJSONFormatChar(t *testing.T) {
	tests := []struct {
		c    rune
		want string
	}{
		{'a', "a"},
		{'"', `\"`},
		{'\n', `\n`},
		{'\x07', `\u0007`},
	}
	for _, tt := range tests {
		if got := formatJSONChar(tt.c); got != tt.want {
			t.Errorf("formatJSONChar(%q) = %q, want %q", tt.c, got, tt.want)
		}
	}
}

func TestJSONFormatValues(t *testing.T) {
	tests := []struct {
		value jvalue
		want  string
	}{
		{jnull{}, "null"},
		{jnumber("1.0"), "1.0"},
		{jstring("hello"), `"hello"`},
		{jboolean(true), "true"},
		{jstring(`"`), `"\""`},
		{jlist{jnumber("1"), jnumber("2"), jnumber("3")}, "[1,2,3]"},
		{
			jobject{{"sp\"ecial\\key", jstring("sp\nvalue\twith\x08control")}},
			`{"sp\"ecial\\key":"sp\nvalue\twith\bcontrol"}`,
		},
		{jobject{{"name", jstring("Bob")}, {"age", jnumber("20")}}, `{"name":"Bob","age":20}`},
	}
	for _, tt := range tests {
		if got := jsonFormat(tt.value); got != tt.want {
			t.Errorf("got %s, want %s", got, tt.want)
		}
	}
}

func hurlFiles(t *testing.T, dirs ...string) []string {
	t.Helper()
	var files []string
	for _, dir := range dirs {
		err := filepath.WalkDir(filepath.Join(integrationDir, dir), func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() && filepath.Ext(path) == ".hurl" {
				files = append(files, path)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(files) == 0 {
		t.Fatal("no Hurl files found")
	}
	return files
}

func parseFile(t *testing.T, path string) (string, *ast.HurlFile) {
	t.Helper()
	content, err := input.New(path).ReadToString()
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	file, perr := parser.ParseHurlFile(content)
	if perr != nil {
		t.Fatalf("%s: %v", path, perr)
	}
	return content, file
}

// TestRoundTrip checks that formatting a parsed file gives back its source,
// both as text and as the text content of the HTML export (what upstream's
// test_html_output.py checks).
func TestRoundTrip(t *testing.T) {
	tags := regexp.MustCompile(`<[^>]*>`)
	files := hurlFiles(t, "hurl/tests_ok", "hurlfmt/tests_ok")
	for _, path := range files {
		src, file := parseFile(t, path)
		if got := Text(file, false); got != src {
			t.Errorf("%s: round trip differs", path)
		}
		if got := html.UnescapeString(tags.ReplaceAllString(HTML(file, false), "")); got != src {
			t.Errorf("%s: HTML text content differs", path)
		}
	}
	t.Logf("round-tripped %d files", len(files))
}

// TestExport compares the JSON and HTML exports with the expected upstream
// outputs. hurlfmt always ends its output with a newline.
func TestExport(t *testing.T) {
	files := hurlFiles(t, "hurlfmt/tests_export")
	count := 0
	for _, path := range files {
		if strings.HasSuffix(path, ".lint.hurl") {
			continue
		}
		_, file := parseFile(t, path)
		base := strings.TrimSuffix(path, ".hurl")
		for ext, got := range map[string]string{
			".json": JSON(file),
			".html": HTML(file, false),
		} {
			want, err := os.ReadFile(base + ext)
			if os.IsNotExist(err) {
				continue
			} else if err != nil {
				t.Fatal(err)
			}
			count++
			if !strings.HasSuffix(got, "\n") {
				got += "\n"
			}
			if got != string(want) {
				t.Errorf("%s%s differs:\ngot:  %s\nwant: %s", base, ext, got, want)
			}
		}
	}
	t.Logf("checked %d exports", count)
}

func TestExportHTMLStandalone(t *testing.T) {
	_, file := parseFile(t, filepath.Join(integrationDir, "hurlfmt/tests_ok/html_standalone.hurl"))
	want, err := os.ReadFile(filepath.Join(integrationDir, "hurlfmt/tests_ok/html_standalone.out"))
	if err != nil {
		t.Fatal(err)
	}
	if got := HTML(file, true); got != string(want) {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
