package junit

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/reader"
)

func dump(root *element) string { return string((&xmlDocument{root: root}).bytes()) }

func srcInfo(line, col, endLine, endCol int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: line, Column: col}, End: reader.Pos{Line: endLine, Column: endCol}}
}

func resultWithError(err *runner.RunnerError, entrySrc ast.SourceInfo, duration time.Duration) *runner.HurlResult {
	return &runner.HurlResult{
		Entries: []runner.EntryResult{{
			EntryIndex: ast.NewIndex(1),
			SourceInfo: entrySrc,
			Errors:     []*runner.RunnerError{err},
		}},
		Duration: duration,
		Success:  true,
	}
}

func assertStatusError() *runner.RunnerError {
	return runner.NewRunnerError(srcInfo(2, 10, 2, 13), runner.RunnerErrorAssertStatus{Actual: "404"}, true)
}

func httpError() *runner.RunnerError {
	return runner.NewRunnerError(srcInfo(1, 5, 1, 19), runner.RunnerErrorHTTP{Value: httpclient.HttpErrorLibcurl{Code: 6, Detail: "Could not resolve host: unknown"}}, false)
}

func TestWriteXML(t *testing.T) {
	root := newElement("catalog").
		addChild(newElement("book").
			attr("id", "bk101").
			addChild(newElement("author").text("Gambardella, Matthew")).
			addChild(newElement("title").text("XML Developer's Guide")).
			addChild(newElement("genre").text("Computer")).
			addChild(newElement("price").text("44.95")).
			addChild(newElement("publish_date").text("2000-10-01")).
			addChild(newElement("description").text("An in-depth look at creating applications with XML."))).
		addChild(newElement("book").
			attr("id", "bk102").
			addChild(newElement("author").text("Ralls, Kim")).
			addChild(newElement("title").text("Midnight Rain")).
			addChild(newElement("genre").text("Fantasy")).
			addChild(newElement("price").text("5.95")).
			addChild(newElement("publish_date").text("2000-12-16")).
			addChild(newElement("description").text("A former architect battles corporate zombies, an evil sorceress, and her own childhood to become queen of the world.")))
	want := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<catalog>` +
		`<book id="bk101">` +
		`<author>Gambardella, Matthew</author>` +
		`<title>XML Developer's Guide</title>` +
		`<genre>Computer</genre>` +
		`<price>44.95</price>` +
		`<publish_date>2000-10-01</publish_date>` +
		`<description>An in-depth look at creating applications with XML.</description>` +
		`</book>` +
		`<book id="bk102">` +
		`<author>Ralls, Kim</author>` +
		`<title>Midnight Rain</title>` +
		`<genre>Fantasy</genre>` +
		`<price>5.95</price>` +
		`<publish_date>2000-12-16</publish_date>` +
		`<description>A former architect battles corporate zombies, an evil sorceress, and her own childhood to become queen of the world.</description>` +
		`</book>` +
		`</catalog>`
	if got := dump(root); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteXMLEscapes(t *testing.T) {
	root := newElement("a").attr("b", "<>\"'&\n\r").text("<>\"'&").addChild(newElement("c").text(""))
	root.children = append(root.children, cdata("x]]>y"), procInst{name: "pi"})
	want := `<?xml version="1.0" encoding="UTF-8"?><a b="&lt;&gt;&quot;&apos;&amp;&#xA;&#xD;">&lt;&gt;"'&amp;<c></c><![CDATA[x]]]]><![CDATA[>y]]><?pi?></a>`
	if got := dump(root); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func parseStr(t *testing.T, s string) *element {
	t.Helper()
	doc, err := parseXML([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return doc.root
}

func TestReadXML0(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<names>
    <name first="bob" last="jones" />
    <name first="elizabeth" last="smith" />
</names>
`
	want := newElement("names").
		addChild(newElement("name").attr("first", "bob").attr("last", "jones")).
		addChild(newElement("name").attr("first", "elizabeth").attr("last", "smith"))
	if got := parseStr(t, xml); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestReadXML1(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<project name="project-name">
    <libraries>
        <library groupId="org.example" artifactId="&lt;name&gt;" version="0.1"/>
        <library groupId="com.example" artifactId="&quot;cool-lib&amp;" version="999"/>
    </libraries>
    <module name="module-1">
        <files>
            <file name="somefile.java" type="java">
                Some &lt;java&gt; class
            </file>
            <file name="another_file.java" type="java">
                Another &quot;java&quot; class
            </file>
            <file name="config.xml" type="xml">
                Weird &apos;XML&apos; config
            </file>
        </files>
        <libraries>
            <library groupId="junit" artifactId="junit" version="1.9.5"/>
        </libraries>
    </module>
    <module name="module-2">
        <files>
            <file name="program.js" type="javascript">
                JavaScript &amp; program
            </file>
            <file name="style.css" type="css">
                Cascading style sheet: &#xA9; - &#1161;
            </file>
        </files>
    </module>
</project>
        `
	want := newElement("project").
		attr("name", "project-name").
		addChild(newElement("libraries").
			addChild(newElement("library").attr("groupId", "org.example").attr("artifactId", "<name>").attr("version", "0.1")).
			addChild(newElement("library").attr("groupId", "com.example").attr("artifactId", "\"cool-lib&").attr("version", "999"))).
		addChild(newElement("module").
			attr("name", "module-1").
			addChild(newElement("files").
				addChild(newElement("file").attr("name", "somefile.java").attr("type", "java").text("\n                Some <java> class\n            ")).
				addChild(newElement("file").attr("name", "another_file.java").attr("type", "java").text("\n                Another \"java\" class\n            ")).
				addChild(newElement("file").attr("name", "config.xml").attr("type", "xml").text("\n                Weird 'XML' config\n            "))).
			addChild(newElement("libraries").
				addChild(newElement("library").attr("groupId", "junit").attr("artifactId", "junit").attr("version", "1.9.5")))).
		addChild(newElement("module").
			attr("name", "module-2").
			addChild(newElement("files").
				addChild(newElement("file").attr("name", "program.js").attr("type", "javascript").text("\n                JavaScript & program\n            ")).
				addChild(newElement("file").attr("name", "style.css").attr("type", "css").text("\n                Cascading style sheet: © - \u0489\n            "))))
	if got := parseStr(t, xml); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s\nwant %s", dump(got), dump(want))
	}
}

func TestReadXMLWithNamespaces(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<p:data xmlns:p="urn:example:namespace" xmlns:d="urn:example:double" xmlns:h="urn:example:header">
  <p:datum id="34">
    <p:name>Name</p:name>
    <d:name>Another name</d:name>
    <d:arg>0.3</d:arg>
    <d:arg>0.2</d:arg>
    <p:arg>0.1</p:arg>
    <p:arg>0.01</p:arg>
    <h:header name="Header-1">header 1 value</h:header>
    <h:header name="Header-2">
      Some bigger value
    </h:header>
  </p:datum>
</p:data>`
	want := newElement("data").addChild(
		newElement("datum").
			attr("id", "34").
			addChild(newElement("name").text("Name")).
			addChild(newElement("name").text("Another name")).
			addChild(newElement("arg").text("0.3")).
			addChild(newElement("arg").text("0.2")).
			addChild(newElement("arg").text("0.1")).
			addChild(newElement("arg").text("0.01")).
			addChild(newElement("header").attr("name", "Header-1").text("header 1 value")).
			addChild(newElement("header").attr("name", "Header-2").text("\n      Some bigger value\n    ")))
	if got := parseStr(t, xml); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s\nwant %s", dump(got), dump(want))
	}
}

func TestReadJUnitXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?><testsuites><testsuite tests="3" errors="0" failures="1"><testcase id="/tmp/a.hurl" name="/tmp/a.hurl" time="0.438" /><testcase id="/tmp/b.hurl" name="/tmp/b.hurl" time="0.234"><failure>Assert status code
  --> /tmp/b.hurl:4:6
   |
 4 | HTTP 300
   |      ^^^ actual value is &lt;200>
   |</failure></testcase><testcase id="/tmp/c.hurl" name="/tmp/c.hurl" time="0.236" /></testsuite><testsuite tests="2" errors="0" failures="1"><testcase id="/tmp/a.hurl" name="/tmp/a.hurl" time="0.370" /><testcase id="/tmp/c.hurl" name="/tmp/c.hurl" time="0.247"><failure>Assert failure
  --> /tmp/c.hurl:6:0
   |
 6 | xpath "normalize-space(//title)" == "Hello World!"
   |   actual:   string &lt;Hurl - Run and Test HTTP Requests>
   |   expected: string &lt;Hello World!>
   |</failure></testcase></testsuite></testsuites>`
	want := newElement("testsuites").
		addChild(newElement("testsuite").
			attr("tests", "3").attr("errors", "0").attr("failures", "1").
			addChild(newElement("testcase").attr("id", "/tmp/a.hurl").attr("name", "/tmp/a.hurl").attr("time", "0.438")).
			addChild(newElement("testcase").attr("id", "/tmp/b.hurl").attr("name", "/tmp/b.hurl").attr("time", "0.234").
				addChild(newElement("failure").text(`Assert status code
  --> /tmp/b.hurl:4:6
   |
 4 | HTTP 300
   |      ^^^ actual value is <200>
   |`))).
			addChild(newElement("testcase").attr("id", "/tmp/c.hurl").attr("name", "/tmp/c.hurl").attr("time", "0.236"))).
		addChild(newElement("testsuite").
			attr("tests", "2").attr("errors", "0").attr("failures", "1").
			addChild(newElement("testcase").attr("id", "/tmp/a.hurl").attr("name", "/tmp/a.hurl").attr("time", "0.370")).
			addChild(newElement("testcase").attr("id", "/tmp/c.hurl").attr("name", "/tmp/c.hurl").attr("time", "0.247").
				addChild(newElement("failure").text(`Assert failure
  --> /tmp/c.hurl:6:0
   |
 6 | xpath "normalize-space(//title)" == "Hello World!"
   |   actual:   string <Hurl - Run and Test HTTP Requests>
   |   expected: string <Hello World!>
   |`))))
	if got := parseStr(t, xml); !reflect.DeepEqual(got, want) {
		t.Errorf("got %s\nwant %s", dump(got), dump(want))
	}
}

func TestReadXMLCoalescing(t *testing.T) {
	xml := "<a>x<!-- c -->y <![CDATA[<z>]]>  <!-- c --> <?pi data?><b/></a>"
	data := "data"
	want := &element{name: "a", children: []xmlNode{text("xy "), cdata("<z>"), procInst{name: "pi", data: &data}, newElement("b")}}
	if got := parseStr(t, xml); !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestReadXMLErrors(t *testing.T) {
	for _, s := range []string{"", "<a>", "<a></b>", "<a/><b/>", "<a/>text", "<a><!DOCTYPE a></a>", "<a>&nbsp;</a>"} {
		doc, err := parseXML([]byte(s))
		if err == nil && doc.root != nil {
			t.Errorf("%q: expected error", s)
		}
	}
}

func TestCreateTestcaseSuccess(t *testing.T) {
	result := &runner.HurlResult{Duration: 230 * time.Millisecond, Success: true}
	elt := NewTestcase(result, "", input.New("test.hurl")).toXML(nil)
	want := `<?xml version="1.0" encoding="UTF-8"?><testcase id="test.hurl" name="test.hurl" time="0.230" />`
	if got := dump(elt); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateTestcaseFailure(t *testing.T) {
	content := "GET http://localhost:8000/not_found\nHTTP/1.0 200\n"
	result := resultWithError(assertStatusError(), srcInfo(1, 1, 1, 35), 230*time.Millisecond)
	result.Success = false
	elt := NewTestcase(result, content, input.New("test.hurl")).toXML(nil)
	want := `<?xml version="1.0" encoding="UTF-8"?><testcase id="test.hurl" name="test.hurl" time="0.230"><failure>Assert status code
  --&gt; test.hurl:2:10
   |
   | GET http://localhost:8000/not_found
 2 | HTTP/1.0 200
   |          ^^^ actual value is &lt;404&gt;
   |</failure></testcase>`
	if got := dump(elt); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateTestcaseError(t *testing.T) {
	content := "GET http://unknown"
	result := resultWithError(httpError(), srcInfo(1, 1, 1, 18), 230*time.Millisecond)
	result.Success = false
	elt := NewTestcase(result, content, input.New("test.hurl")).toXML([]string{"unknown"})
	want := `<?xml version="1.0" encoding="UTF-8"?><testcase id="test.hurl" name="test.hurl" time="0.230"><error>HTTP connection
  --&gt; test.hurl:1:5
   |
 1 | GET http://***
   |     ^^^^^^^^^^^^^^ (6) Could not resolve host: ***
   |</error></testcase>`
	if got := dump(elt); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestCreateJUnitReport(t *testing.T) {
	content := "GET http://localhost:8000/not_found\nHTTP/1.0 200"
	filename := input.New("test.hurl")
	testcases := []Testcase{
		NewTestcase(&runner.HurlResult{Duration: 124 * time.Millisecond, Success: true}, content, filename),
		NewTestcase(resultWithError(assertStatusError(), srcInfo(1, 1, 1, 35), 200*time.Millisecond), content, filename),
		NewTestcase(resultWithError(httpError(), srcInfo(1, 1, 1, 35), 230*time.Millisecond), content, filename),
	}
	want := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>" +
		"<testsuite tests=\"3\" errors=\"1\" failures=\"1\">" +
		"<testcase id=\"test.hurl\" name=\"test.hurl\" time=\"0.124\" />" +
		"<testcase id=\"test.hurl\" name=\"test.hurl\" time=\"0.200\">" +
		"<failure>Assert status code\n  " +
		"--&gt; test.hurl:2:10\n   " +
		"|\n   " +
		"| GET http://localhost:8000/not_found\n " +
		"2 | HTTP/1.0 200\n   " +
		"|          ^^^ actual value is &lt;404&gt;\n   " +
		"|" +
		"</failure>" +
		"</testcase>" +
		"<testcase id=\"test.hurl\" name=\"test.hurl\" time=\"0.230\">" +
		"<error>HTTP connection\n  --&gt; test.hurl:1:5\n   |\n 1 | GET http://localhost:8000/not_found\n   |     ^^^^^^^^^^^^^^ (6) Could not resolve host: unknown\n   |" +
		"</error>" +
		"</testcase>" +
		"</testsuite>"
	if got := dump(createTestsuite(testcases, nil)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

const integrationDir = "../../../integration/hurl"

// bodyResult returns a result failing on a body assert at line, like
// tests_ok/junit/test.{2,4}.hurl.
func bodyResult(line int, expected string) *runner.HurlResult {
	pos := srcInfo(line, 1, line, len([]rune(expected))+3)
	err := runner.NewRunnerError(pos, runner.RunnerErrorAssertBodyValueError{Actual: "Hello World!", Expected: expected}, true)
	result := resultWithError(err, srcInfo(1, 1, 1, 32), 10*time.Millisecond)
	result.Success = false
	return result
}

func junitTestcase(t *testing.T, name string, result *runner.HurlResult) Testcase {
	t.Helper()
	filename := "tests_ok/junit/" + name
	content, err := os.ReadFile(filepath.Join(integrationDir, filename))
	if err != nil {
		t.Fatal(err)
	}
	return NewTestcase(result, string(content), input.New(filename))
}

// TestWriteReportAppend follows integration/hurl/tests_ok/junit/junit.sh.
func TestWriteReportAppend(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "build", "junit", "result.xml")
	ok := &runner.HurlResult{Duration: 5 * time.Millisecond, Success: true}
	runs := [][]Testcase{
		{junitTestcase(t, "test.1.hurl", ok), junitTestcase(t, "test.2.hurl", bodyResult(8, "Goodbye World!"))},
		{junitTestcase(t, "test.3.hurl", ok)},
		{junitTestcase(t, "test.4.hurl", bodyResult(3, `Hello\tWorld!`))},
	}
	for _, run := range runs {
		if err := WriteReport(filename, run, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	pattern, err := os.ReadFile(filepath.Join(integrationDir, "tests_ok/junit/junit.out.pattern"))
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(pattern), "<<<.*?>>>")
	for i, p := range parts {
		parts[i] = regexp.QuoteMeta(p)
	}
	re := regexp.MustCompile(`^` + strings.Join(parts, `.*?`) + `$`)
	if !re.Match(got) {
		t.Errorf("report doesn't match pattern:\n%s", got)
	}
}

func TestWriteReportInvalidExisting(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "result.xml")
	if err := os.WriteFile(filename, []byte("<testsuites>"), 0o666); err != nil {
		t.Fatal(err)
	}
	err := WriteReport(filename, nil, nil)
	if err == nil || !strings.HasPrefix(err.Error(), "Issue reading JUnit report "+filename+" ") {
		t.Errorf("err = %v", err)
	}
}
