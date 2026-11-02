package xpath

import (
	"math"
	"strings"
	"testing"
)

func mustParse(t *testing.T, data string, format Format) *Document {
	t.Helper()
	doc, err := Parse(data, format)
	if err != nil {
		t.Fatalf("Parse(%q): %v", data, err)
	}
	return doc
}

type evalCase struct {
	expr string
	want Value
}

func checkEval(t *testing.T, doc *Document, tests []evalCase) {
	t.Helper()
	for _, tt := range tests {
		got, err := doc.EvalXPath(tt.expr)
		if err != nil {
			t.Errorf("EvalXPath(%q): %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("EvalXPath(%q) = %#v, want %#v", tt.expr, got, tt.want)
		}
	}
}

func checkEvalError(t *testing.T, doc *Document, exprs ...string) {
	t.Helper()
	for _, expr := range exprs {
		got, err := doc.EvalXPath(expr)
		if err != ErrEval {
			t.Errorf("EvalXPath(%q) = %#v, %v; want ErrEval", expr, got, err)
		}
	}
}

func TestXML(t *testing.T) {
	xml := `<?xml version="1.0" encoding="utf-8"?>
<food>
  <banana type="fruit" price="1.1"/>
  <apple type="fruit"/>
  <beef type="meat"/>
</food>
`
	doc := mustParse(t, xml, XML)
	checkEval(t, doc, []evalCase{
		{"count(//food/*)", Number(3.0)},
		{"//food/*", Nodeset(3)},
		{"count(//*[@type='fruit'])", Number(2.0)},
		{"number(//food/banana/@price)", Number(1.1)},
	})
}

func TestErrorEval(t *testing.T) {
	doc := mustParse(t, "<a/>", XML)
	checkEvalError(t, doc, "^^^", "//")
}

func TestInvalidXML(t *testing.T) {
	if _, err := Parse("??", XML); err == nil {
		t.Error("Parse(\"??\") succeeded, want error")
	}
}

func TestCafe(t *testing.T) {
	for _, format := range []Format{XML, HTML} {
		doc := mustParse(t, "<data>café</data>", format)
		checkEval(t, doc, []evalCase{{"normalize-space(//data)", String("café")}})
	}
}

func TestHTML(t *testing.T) {
	html := `<html>
  <head>
    <meta charset="UTF-8"\>
  </head>
  <body>
    <br>
  </body>
</html>`
	doc := mustParse(t, html, HTML)
	checkEval(t, doc, []evalCase{{"normalize-space(/html/head/meta/@charset)", String("UTF-8")}})
}

func TestBug(t *testing.T) {
	doc := mustParse(t, "<html></html>", HTML)
	checkEval(t, doc, []evalCase{{"boolean(count(//a[contains(@href,'xxx')]))", Bool(false)}})
}

func TestUnregisteredFunction(t *testing.T) {
	doc := mustParse(t, "<html></html>", HTML)
	checkEvalError(t, doc, "strong(//head/title)")
}

func TestNamespacesWithPrefix(t *testing.T) {
	xml := `<?xml version ="1.0"?>
<a:books xmlns:a="foo:" xmlns:b="bar:">
    <b:book xmlns:c="baz:">
        <b:title>Dune</b:title>
        <c:author>Franck Herbert</c:author>
    </b:book>
</a:books>`
	doc := mustParse(t, xml, XML)
	checkEval(t, doc, []evalCase{
		{"string(//a:books/b:book/b:title)", String("Dune")},
		{"string(//a:books/b:book/c:author)", String("Franck Herbert")},
		{"string(//*[name()='a:books']/*[name()='b:book']/*[name()='c:author'])", String("Franck Herbert")},
		{"string(//*[local-name()='books']/*[local-name()='book']/*[local-name()='author'])", String("Franck Herbert")},
	})
}

func TestDefaultNamespaces(t *testing.T) {
	xml := `<svg version="1.1" width="300" height="200" xmlns="http://www.w3.org/2000/svg">
    <rect width="100%" height="100%" fill="red" />
    <circle cx="150" cy="100" r="80" fill="green" />
    <text x="150" y="125" font-size="60" text-anchor="middle" fill="white">SVG</text>
</svg>`
	doc := mustParse(t, xml, XML)
	checkEval(t, doc, []evalCase{
		{"string(//_:svg/_:text)", String("SVG")},
		{"string(//*[name()='svg']/*[name()='text'])", String("SVG")},
		{"string(//*[local-name()='svg']/*[local-name()='text'])", String("SVG")},
	})
}

func TestSoap(t *testing.T) {
	xml := `<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/"
    xmlns:xsd="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
    <soap:Body xmlns:ns1="http://www.opentravel.org/OTA/2003/05">
        <ns1:OTA_AirAvailRS
            EchoToken="11868765275150-1300257934"
            PrimaryLangID="en-us"
            RetransmissionIndicator="false"
            SequenceNmbr="1"
            TransactionIdentifier="TID$16459590516432752971.demo2144"
            Version="2006.01">
        </ns1:OTA_AirAvailRS>
    </soap:Body>
</soap:Envelope>`
	doc := mustParse(t, xml, XML)
	want := String("TID$16459590516432752971.demo2144")
	checkEval(t, doc, []evalCase{
		{"string(//soap:Envelope/soap:Body/ns1:OTA_AirAvailRS/@TransactionIdentifier)", want},
		{"string(//*[name()='soap:Envelope']/*[name()='soap:Body']/*[name()='ns1:OTA_AirAvailRS']/@TransactionIdentifier)", want},
		{"string(//*[local-name()='Envelope']/*[local-name()='Body']/*[local-name()='OTA_AirAvailRS']/@TransactionIdentifier)", want},
	})
}

func TestNamespacesScoping(t *testing.T) {
	xml := `<?xml version="1.0"?>
<!-- initially, the default namespace is "books" -->
<book xmlns='urn:loc.gov:books'
      xmlns:isbn='urn:ISBN:0-395-36341-6'>
    <title>Cheaper by the Dozen</title>
    <isbn:number>1568491379</isbn:number>
    <notes>
      <!-- make HTML the default namespace for some commentary -->
      <p xmlns='http://www.w3.org/1999/xhtml'>
          This is a <i>funny</i> book!
      </p>
    </notes>
</book>
        `
	doc := mustParse(t, xml, XML)
	checkEval(t, doc, []evalCase{
		{"string(//_:book/_:title)", String("Cheaper by the Dozen")},
		{"string(//_:book/isbn:number)", String("1568491379")},
		{"//*[name()='book']/*[name()='notes']", Nodeset(1)},
		{"//_:book/_:notes/*[local-name()='p']", Nodeset(1)},
	})
}

// From runner/cache.rs.
func TestCacheHTML(t *testing.T) {
	html := "<!DOCTYPE html> <html> <body> <h1>My First Heading</h1> <p>My first paragraph.</p> </body> </html>"
	doc := mustParse(t, html, HTML)
	checkEval(t, doc, []evalCase{{"string(//h1)", String("My First Heading")}})
}

// Expected values below were checked with xmllint (libxml2 2.9.13, as linked by Hurl on macOS).

func TestParseErrors(t *testing.T) {
	tests := []struct {
		data   string
		format Format
		want   string
	}{
		{"", XML, "invalid input data"},
		{"", HTML, "invalid input data"},
		{"??", XML, "no root element"},
		{"junk<a/>", XML, "no root element"},
		{`<?xml version="1.0"`, XML, "no root element"},
		{"<!-- c -->", XML, "no root element"},
		{"  ", HTML, "no root element"},
		{"<!-- c -->", HTML, "no root element"},
		{"<!DOCTYPE html>", HTML, "no root element"},
		{`<?xml version="1.0"`, HTML, "no root element"},
		{"</p>", HTML, "no root element"},
	}
	for _, tt := range tests {
		_, err := Parse(tt.data, tt.format)
		if err == nil || err.Error() != tt.want {
			t.Errorf("Parse(%q, %v) error = %v, want %q", tt.data, tt.format, err, tt.want)
		}
	}
}

func TestXMLRecover(t *testing.T) {
	tests := []struct {
		data string
		want []evalCase
	}{
		{"<a>", []evalCase{{"count(/a)", Number(1)}}},
		{"<a><b>x</a><c/></a>", []evalCase{{"count(/a/*)", Number(2)}, {"string(/a/b)", String("x")}}},
		{"<a></b><c/></a>", []evalCase{{"count(//c)", Number(0)}}},
		{"<a><b><c></b><d/></a>", []evalCase{{"count(/a/b/*)", Number(2)}}},
		{"<a/><b/>", []evalCase{{"count(//b)", Number(0)}}},
		{"<a/>junk", []evalCase{{"count(/a)", Number(1)}}},
		{"<a>&amp;&#233;<![CDATA[x<y]]></a>", []evalCase{{"string(/a)", String("&éx<y")}}},
		{"<a><q:c/></a>", []evalCase{{"local-name(/a/*)", String("q:c")}, {"name(/a/*)", String("q:c")}}},
		{`<a xmlns:p="u" p:x="1" y="2"/>`, []evalCase{{"count(/a/@*)", Number(2)}, {"name(/a/@*[1])", String("p:x")}, {"string(/a/@p:x)", String("1")}}},
		{`<a xml:lang="en"/>`, []evalCase{{"string(/a/@xml:lang)", String("en")}}},
		{`<?xml version="1.0" encoding="ISO-8859-1"?><a>é</a>`, []evalCase{{"string(/a)", String("é")}}},
	}
	for _, tt := range tests {
		checkEval(t, mustParse(t, tt.data, XML), tt.want)
	}
}

func TestHTMLParse(t *testing.T) {
	doc := mustParse(t, "Hello World!", HTML)
	checkEval(t, doc, []evalCase{{"string(/html/body)", String("Hello World!")}})
	checkEvalError(t, doc, "//")

	for _, html := range []string{"<body>A</body>\n</html>\n", "<BODY>A</BODY> <!-- c --> </HTML> "} {
		checkEval(t, mustParse(t, html, HTML), []evalCase{{"string(//body)", String("A")}, {"count(//comment())", Number(strings.Count(html, "<!--"))}})
	}
	checkEval(t, mustParse(t, "<body>A\n</body>", HTML), []evalCase{{"string(//body)", String("A\n")}})

	doc = mustParse(t, `<html><body><svg><clipPath/></svg><div xlink:href="x" HREF="y">t</div></body></html>`, HTML)
	checkEval(t, doc, []evalCase{
		{"count(//clippath)", Number(1)},
		{"name(//div/@*[1])", String("xlink:href")},
		{"string(//div/@href)", String("y")},
	})
}

func TestNumbers(t *testing.T) {
	doc := mustParse(t, "<a>x</a>", XML)
	checkEval(t, doc, []evalCase{
		{"count(/a)", Number(1)},
		{"1 div 0", Number(math.Inf(1))},
		{"string(1 div 0)", String("Infinity")},
		{"string(0 div 0)", String("NaN")},
		{"boolean('')", Bool(false)},
		{"concat('a', 'b')", String("ab")},
	})
	got, err := doc.EvalXPath("number(/a)")
	if n, ok := got.(Number); err != nil || !ok || !math.IsNaN(float64(n)) {
		t.Errorf("EvalXPath(number(/a)) = %#v, %v; want NaN", got, err)
	}
}

func TestUnsupportedFunctions(t *testing.T) {
	doc := mustParse(t, "<a>x</a>", XML)
	checkEvalError(t, doc,
		"lower-case('A')", "upper-case('a')", "ends-with('ab', 'b')", "matches('a', 'a')",
		"replace('a', 'a', 'b')", "reverse(//a)", "string-join(//a, ',')", "fn:count(//a)", "$x",
		"count(//namespace::*)", "count(/a/namespace :: *)",
	)
	checkEval(t, doc, []evalCase{{"count (//a)", Number(1)}, {"count(//text ())", Number(1)}})
}

func TestNamespaceMatching(t *testing.T) {
	doc := mustParse(t, `<svg xmlns="http://www.w3.org/2000/svg"><text a="1">SVG</text><g xmlns=""><text/></g></svg>`, XML)
	checkEval(t, doc, []evalCase{
		{"count(//svg)", Number(0)},
		{"count(//*)", Number(4)},
		{"count(//_:svg)", Number(1)},
		{"count(//_:text)", Number(1)},
		{"count(//text)", Number(1)},
		{"count(//g/text)", Number(1)},
		{"string(//_:text/@a)", String("1")},
		{"count(//*[local-name()='text'])", Number(2)},
		{"namespace-uri(/*)", String("http://www.w3.org/2000/svg")},
		{"count(//_:*)", Number(2)},
		{"count(//_:*[1])", Number(1)},
		{"count(child::_:svg/child :: _:text)", Number(1)},
		{"count(//_:text[@a = 1 and 2 * 3 div 3 = 2 or 1 mod 1])", Number(1)},
	})
	checkEvalError(t, doc, "//x:svg", "//x:*")

	html := mustParse(t, "<html><body><a>t</a></body></html>", HTML)
	checkEvalError(t, html, "//x:a", "//_:a", "//_:*")
}

func TestPreprocess(t *testing.T) {
	tests := []struct{ expr, want string }{
		{"//a/b", "//n:a/n:b"},
		{"//_:a/b:c", "//_:a/b:c"},
		{"count(//a) div count(//b)", "count(//n:a) div count(//n:b)"},
		{"//div[mod and div]", "//n:div[n:mod and n:div]"},
		{"//*/a * 2", "//*/n:a * 2"},
		{"child::a/attribute::b/@c", "child::n:a/attribute::n:b/@n:c"},
		{"//text()|//node()|//comment()", "//text()|//node()|//comment()"},
		{"//a[@x='b c']/b:*", "//n:a[@n:x='b c']/*[namespace-uri()='u:b']"},
		{"//q:*", "//*[namespace-uri()=\"u'q\"]"},
		{"//z:*", "//z:*"},
		{"$v/a", "$v/n:a"},
		{"./a/../b-c.d", "./n:a/../n:b-c.d"},
		{"//a[. = 1.5]", "//n:a[. = 1.5]"},
		{"//a[1]", "//n:a[1]"},
	}
	namespaces := map[string]string{"b": "u:b", "q": "u'q"}
	for _, tt := range tests {
		got, ok := preprocess(tt.expr, namespaces, "n")
		if !ok || got != tt.want {
			t.Errorf("preprocess(%q) = %q, %v; want %q", tt.expr, got, ok, tt.want)
		}
	}
}
