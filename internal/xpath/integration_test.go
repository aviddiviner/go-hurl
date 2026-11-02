package xpath

import (
	"compress/gzip"
	"io"
	"os"
	"testing"
)

// Bodies and expressions from the integration suite (integration/hurl/tests_*/*.py and .hurl).

func TestIntegrationAssertXPath(t *testing.T) {
	checkEval(t, mustParse(t, "<data>café</data>", XML), []evalCase{
		{"normalize-space(//data)", String("café")},
		{"//toto", Nodeset(0)},
	})

	svg := `<?xml version="1.0"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.0//EN"
              "http://www.w3.org/TR/2001/REC-SVG-20010904/DTD/svg10.dtd">
<svg xmlns="http://www.w3.org/2000/svg">
  <style type="text/css">
    circle:hover {fill-opacity:0.9;}
  </style>
  <g style="fill-opacity:0.7;">
    <circle cx="6.5cm" cy="2cm" r="100" style="fill:red; stroke:black; stroke-width:0.1cm" transform="translate(0,50)" />
    <circle cx="6.5cm" cy="2cm" r="100" style="fill:blue; stroke:black; stroke-width:0.1cm" transform="translate(70,150)" />
    <circle cx="6.5cm" cy="2cm" r="100" style="fill:green; stroke:black; stroke-width:0.1cm" transform="translate(-70,150)"/>
  </g>
</svg>
`
	checkEval(t, mustParse(t, svg, XML), []evalCase{
		{"//_:svg/_:g/_:circle", Nodeset(3)},
		{"//*[local-name()='svg']/*[local-name()='g']/*[local-name()='circle']", Nodeset(3)},
		{"//*[name()='svg']/*[name()='g']/*[name()='circle']", Nodeset(3)},
	})

	simpleNS := `<?xml version="1.0"?>
<!-- both namespace prefixes are available throughout -->
<bk:book xmlns:bk='urn:loc.gov:books'
         xmlns:isbn='urn:ISBN:0-395-36341-6'>
    <bk:title>Cheaper by the Dozen</bk:title>
    <isbn:number>1568491379</isbn:number>
</bk:book>
`
	checkEval(t, mustParse(t, simpleNS, XML), []evalCase{
		{"string(//bk:book/bk:title)", String("Cheaper by the Dozen")},
		{"string(//*[name()='bk:book']/*[name()='bk:title'])", String("Cheaper by the Dozen")},
		{"string(//*[local-name()='book']/*[local-name()='title'])", String("Cheaper by the Dozen")},
		{"string(//bk:book/isbn:number)", String("1568491379")},
		{"string(//*[name()='bk:book']/*[name()='isbn:number'])", String("1568491379")},
		{"string(//*[local-name()='book']/*[local-name()='number'])", String("1568491379")},
	})

	ns := `<?xml version="1.0"?>
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
	checkEval(t, mustParse(t, ns, XML), []evalCase{
		{"string(//_:book/_:title)", String("Cheaper by the Dozen")},
		{"string(//_:book/isbn:number)", String("1568491379")},
		{"//*[name()='book']/*[name()='notes']", Nodeset(1)},
		{"//*[local-name()='book']/*[local-name()='notes']", Nodeset(1)},
		{"//_:book/_:notes/*[local-name()='p']", Nodeset(1)},
	})
}

func TestIntegrationHTML(t *testing.T) {
	capturesXML := `<!DOCTYPE html><html><head><meta charset="utf-8"><title>title</title></head>` +
		`<body><p>Lorem ipsum dolor sit amet</p><p>Sed ut perspiciatis unde omnis</p></body></html>`
	checkEval(t, mustParse(t, capturesXML, HTML), []evalCase{{"//p", Nodeset(2)}})

	gb2312 := `<!DOCTYPE html>
<html>
    <head>
        <meta http-equiv='Content-Type' content='text/html; charset=gb2312'>
    </head>
    <body>你好世界</body>
</html>
`
	checkEval(t, mustParse(t, gb2312, HTML), []evalCase{{"string(//body)", String("你好世界")}})

	errorFormat := mustParse(t, "<html><head><title>Test</title></head></html>", HTML)
	checkEval(t, errorFormat, []evalCase{
		{"string(//head/title)", String("Test")},
		{"//foo", Nodeset(0)},
		{"//title", Nodeset(1)},
	})
	checkEvalError(t, errorFormat, "strong(//head/title)")

	// The xpath filter parses its input as HTML.
	checkEval(t, mustParse(t, "<node></node>", HTML), []evalCase{{"/", Nodeset(1)}})
}

func TestIntegrationErrors(t *testing.T) {
	// runner_errors/invalid-xml and invalid_xml: empty body.
	if _, err := Parse("", XML); err == nil {
		t.Error("Parse(\"\", XML) succeeded")
	}
	// filter: jsonpath "$.invalid_xml" xpath ...
	if _, err := Parse(`<?xml version="1.0"`, HTML); err == nil {
		t.Error("Parse(`<?xml version=\"1.0\"`, HTML) succeeded")
	}
	// assert_query_invalid_xpath and runner_errors: xpath "//".
	checkEvalError(t, mustParse(t, "<data>café</data>", XML), "//")
	checkEvalError(t, mustParse(t, "Hello World!", HTML), "//")
}

func TestIntegrationParseCache(t *testing.T) {
	f, err := os.Open("../../integration/hurl/tests_ok/parse_cache/parse_cache.html.gz")
	if err != nil {
		t.Skip(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	checkEval(t, mustParse(t, string(data), HTML), []evalCase{
		{"//h1", Nodeset(1)},
		{"//h2", Nodeset(6)},
		{"//h3", Nodeset(10)},
		{"//h4", Nodeset(0)},
		{"//p", Nodeset(397)},
		{"//a", Nodeset(3364)},
	})
}
