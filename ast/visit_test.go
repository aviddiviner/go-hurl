package ast

import (
	"strconv"
	"strings"
	"testing"
)

func ws(s string) Whitespace { return Whitespace{Value: s} }

func lt(nl string) LineTerminator { return LineTerminator{Newline: ws(nl)} }

func tmpl(delim rune, s string) Template {
	return Template{Delimiter: delim, Elements: []TemplateElement{TemplateString{Value: s, Source: s}}}
}

func kv(k, v string) KeyValue {
	return KeyValue{Key: tmpl(0, k), Space2: ws(" "), Value: tmpl(0, v), LineTerminator0: lt("\n")}
}

func jsonpathAssert(expr, value string) Assert {
	return Assert{
		Query:  Query{Value: QueryJSONPath{Space0: ws(" "), Expr: tmpl('"', expr)}},
		Space1: ws(" "),
		Predicate: Predicate{PredicateFunc: PredicateFunc{
			Value: PredicateFuncEqual{Space0: ws(" "), Value: PredicateString{Value: tmpl('"', value)}},
		}},
		LineTerminator0: lt("\n"),
	}
}

func section(value SectionValue) Section {
	return Section{LineTerminator0: lt("\n"), Value: value}
}

func entry(asserts ...Assert) Entry {
	return Entry{
		Request: Request{
			Method:          "GET",
			Space1:          ws(" "),
			URL:             tmpl(0, "https://foo.com"),
			LineTerminator0: lt("\n"),
		},
		Response: &Response{
			Version:         Version{Value: VersionAny},
			Space1:          ws(" "),
			Status:          Status{Value: StatusSpecific{Value: 200}},
			LineTerminator0: lt("\n"),
			Sections:        []Section{section(SectionAsserts{Value: asserts})},
		},
	}
}

type assertWalker struct {
	BaseVisitor
	count int
}

func (w *assertWalker) VisitAssert(*Assert) bool {
	w.count++
	return false
}

func TestWalkAssert(t *testing.T) {
	// GET https://foo.com
	// HTTP 200
	// [Asserts]
	// jsonpath "$.toto[0]" == "tata"
	// ... (5 asserts)
	//
	// GET https://foo.com
	// HTTP 200
	// [Asserts]
	// status == 200
	// header "Location" not exists
	e1 := entry(
		jsonpathAssert("$.toto[0]", "tata"),
		jsonpathAssert("$.toto[1]", "toto"),
		jsonpathAssert("$.toto[2]", "titi"),
		jsonpathAssert("$.toto[3]", "tata"),
		jsonpathAssert("$.toto[4]", "tutu"),
	)
	e2 := entry(
		Assert{
			Query:     Query{Value: QueryStatus{}},
			Space1:    ws(" "),
			Predicate: Predicate{PredicateFunc: PredicateFunc{Value: PredicateFuncEqual{Space0: ws(" "), Value: PredicateNumber{Value: NumberInteger{Value: I64{Value: 200, Source: "200"}}}}}},
		},
		Assert{
			Query:     Query{Value: QueryHeader{Space0: ws(" "), Name: tmpl('"', "Location")}},
			Space1:    ws(" "),
			Predicate: Predicate{Not: true, Space0: ws(" "), PredicateFunc: PredicateFunc{Value: PredicateFuncExist{}}},
		},
	)
	e2.Request.LineTerminators = []LineTerminator{lt("\n")}
	file := HurlFile{Entries: []Entry{e1, e2}}

	w := &assertWalker{}
	Walk(w, &file)
	if w.count != 7 {
		t.Errorf("count = %d, want 7", w.count)
	}
}

// textFormatter is the plain-text part of hurlfmt's TextFormatter: it
// outputs every leaf, so it reproduces the source when the traversal order
// is right.
type textFormatter struct {
	BaseVisitor
	b strings.Builder
}

func (f *textFormatter) VisitBase64Value(_ []byte, source string) { f.b.WriteString(source) }
func (f *textFormatter) VisitBool(v bool)                         { f.b.WriteString(strconv.FormatBool(v)) }
func (f *textFormatter) VisitCookiePath(p *CookiePath)            { f.b.WriteString(p.ToSource()) }
func (f *textFormatter) VisitComment(c *Comment)                  { f.b.WriteString(c.ToSource()) }
func (f *textFormatter) VisitDurationUnit(u DurationUnit)         { f.b.WriteString(u.String()) }
func (f *textFormatter) VisitFilename(t *Template)                { f.b.WriteString(t.ToSource()) }
func (f *textFormatter) VisitFilterKind(k FilterValue)            { f.b.WriteString(k.Identifier()) }
func (f *textFormatter) VisitHexValue(_ []byte, source string)    { f.b.WriteString(source) }
func (f *textFormatter) VisitI64(n int64)                         { f.b.WriteString(strconv.FormatInt(n, 10)) }
func (f *textFormatter) VisitJSONBody(j JSONValue)                { f.b.WriteString(j.ToSource()) }
func (f *textFormatter) VisitLiteral(lit string)                  { f.b.WriteString(lit) }
func (f *textFormatter) VisitMethod(m Method)                     { f.b.WriteString(m.ToSource()) }
func (f *textFormatter) VisitMultilineString(s *MultilineString)  { f.b.WriteString(s.ToSource()) }
func (f *textFormatter) VisitNot(id string)                       { f.b.WriteString(id) }
func (f *textFormatter) VisitNull(id string)                      { f.b.WriteString(id) }
func (f *textFormatter) VisitNumber(n Number)                     { f.b.WriteString(n.ToSource()) }
func (f *textFormatter) VisitPlaceholder(p *Placeholder)          { f.b.WriteString(p.ToSource()) }
func (f *textFormatter) VisitPredicateKind(k PredicateFuncValue)  { f.b.WriteString(k.Identifier()) }
func (f *textFormatter) VisitQueryKind(k QueryValue)              { f.b.WriteString(k.Identifier()) }
func (f *textFormatter) VisitRegex(r *Regex)                      { f.b.WriteString(r.ToSource()) }
func (f *textFormatter) VisitStatus(s StatusValue)                { f.b.WriteString(s.String()) }
func (f *textFormatter) VisitString(s string)                     { f.b.WriteString(s) }
func (f *textFormatter) VisitSectionHeader(name string)           { f.b.WriteString(name) }
func (f *textFormatter) VisitTemplate(t *Template)                { f.b.WriteString(t.ToSource()) }
func (f *textFormatter) VisitURL(t *Template)                     { f.b.WriteString(t.ToSource()) }
func (f *textFormatter) VisitU64(n *U64)                          { f.b.WriteString(n.ToSource()) }
func (f *textFormatter) VisitUsize(n int)                         { f.b.WriteString(strconv.Itoa(n)) }
func (f *textFormatter) VisitVariableName(name string)            { f.b.WriteString(name) }
func (f *textFormatter) VisitVersion(v VersionValue)              { f.b.WriteString(v.String()) }
func (f *textFormatter) VisitXMLBody(xml string)                  { f.b.WriteString(xml) }
func (f *textFormatter) VisitWhitespace(w *Whitespace)            { f.b.WriteString(w.Value) }

func format(file *HurlFile) string {
	f := &textFormatter{}
	Walk(f, file)
	return f.b.String()
}

func TestTextVisitorRoundTrip(t *testing.T) {
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
	e := entry(jsonpathAssert("$.name", "toto"))
	e.Request.LineTerminators = []LineTerminator{lt("\n")}
	e.Request.Headers = []KeyValue{kv("header1", "value1"), kv("header2", "value2")}
	e.Request.Sections = []Section{section(SectionFormParams{Value: []KeyValue{kv("foo", "bar"), kv("baz", "123")}, Short: true})}
	file := HurlFile{Entries: []Entry{e}}
	if got := format(&file); got != src {
		t.Errorf("got:\n%s\nwant:\n%s", got, src)
	}
}

func TestTextVisitorOptionsAndCapture(t *testing.T) {
	src := `GET https://foo.com # comment
[Options]
retry : -1
delay: 2s
variable: a = 1.10
[Multipart]
file: file,data.bin; image/png
HTTP 200
[Captures]
id: header "X-Id" split "," nth 1 redact
`
	unit := DurationUnitSecond
	ct := tmpl(0, "image/png")
	e := entry()
	comment := Comment{Value: " comment"}
	e.Request.LineTerminator0 = LineTerminator{Space0: ws(" "), Comment: &comment, Newline: ws("\n")}
	e.Request.Sections = []Section{
		section(SectionOptions{Value: []EntryOption{
			{Space1: ws(" "), Space2: ws(" "), Kind: OptionRetry{Value: CountLiteral{Value: CountInfinite{}}}, LineTerminator0: lt("\n")},
			{Space2: ws(" "), Kind: OptionDelay{Value: DurationLiteral{Value: Duration{Value: U64{Value: 2, Source: "2"}, Unit: &unit}}}, LineTerminator0: lt("\n")},
			{Space2: ws(" "), Kind: OptionVariable{Value: VariableDefinition{
				Name: "a", Space0: ws(" "), Space1: ws(" "),
				Value: VariableNumber{Value: NumberFloat{Value: Float{Value: 1.1, Source: "1.10"}}},
			}}, LineTerminator0: lt("\n")},
		}}),
		section(SectionMultipartFormData{Value: []MultipartParam{
			MultipartParamFilenameParam{Value: FilenameParam{
				Key:    tmpl(0, "file"),
				Space2: ws(" "),
				Value: FilenameValue{
					Filename:    tmpl(0, "data.bin"),
					Space2:      ws(" "),
					ContentType: &ct,
				},
				LineTerminator0: lt("\n"),
			}},
		}, Short: true}),
	}
	e.Response.Sections = []Section{section(SectionCaptures{Value: []Capture{{
		Name:   tmpl(0, "id"),
		Space2: ws(" "),
		Query:  Query{Value: QueryHeader{Space0: ws(" "), Name: tmpl('"', "X-Id")}},
		Filters: []SpacedFilter{
			{Space0: ws(" "), Filter: Filter{Value: FilterSplit{Space0: ws(" "), Sep: tmpl('"', ",")}}},
			{Space0: ws(" "), Filter: Filter{Value: FilterNth{Space0: ws(" "), N: IntegerLiteral{Value: I64{Value: 1, Source: "1"}}}}},
		},
		Space3:          ws(" "),
		Redacted:        true,
		LineTerminator0: lt("\n"),
	}}})}
	file := HurlFile{Entries: []Entry{e}}
	if got := format(&file); got != src {
		t.Errorf("got:\n%s\nwant:\n%s", got, src)
	}
}

// spanVisitor checks that a composite visit can wrap its children by
// calling the Walk function itself.
type spanVisitor struct {
	BaseVisitor
	b strings.Builder
}

func (s *spanVisitor) VisitEntry(e *Entry) bool {
	s.b.WriteString("<entry>")
	WalkEntry(s, e)
	s.b.WriteString("</entry>")
	return false
}

func (s *spanVisitor) VisitRequest(r *Request) bool {
	s.b.WriteString("<request>")
	WalkRequest(s, r)
	s.b.WriteString("</request>")
	return false
}

func (s *spanVisitor) VisitMethod(m Method)                { s.b.WriteString(m.String()) }
func (s *spanVisitor) VisitStatus(v StatusValue)           { s.b.WriteString(v.String()) }
func (s *spanVisitor) VisitWhitespace(w *Whitespace)       { s.b.WriteString(w.Value) }
func (s *spanVisitor) VisitURL(*Template)                  { s.b.WriteString("url") }
func (s *spanVisitor) VisitVersion(v VersionValue)         { s.b.WriteString(v.String()) }
func (s *spanVisitor) VisitSectionHeader(name string)      { s.b.WriteString(name) }
func (s *spanVisitor) VisitAssert(*Assert) bool            { s.b.WriteString("assert"); return false }
func (s *spanVisitor) VisitSectionValue(SectionValue) bool { return true }

func TestVisitorWrapsChildren(t *testing.T) {
	file := HurlFile{Entries: []Entry{entry(jsonpathAssert("$", "a"))}}
	v := &spanVisitor{}
	Walk(v, &file)
	want := "<entry><request>GET url\n</request>HTTP 200\n[Asserts]\nassert</entry>"
	if got := v.b.String(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
