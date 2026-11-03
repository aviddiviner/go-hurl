package format

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
)

//go:embed hurl.css
var hurlCSS string

// HurlCSS returns the stylesheet for the HTML produced by HTML.
func HurlCSS() string { return hurlCSS }

// HTML returns the Hurl file as syntax-highlighted HTML.
//
// If standalone is true, a complete HTML document with inline styling is
// returned. Otherwise, a <pre> element is returned, without styling.
func HTML(file *ast.HurlFile, standalone bool) string {
	f := &htmlFormatter{}
	ast.Walk(f, file)
	body := f.buf.String()
	if !standalone {
		return body
	}
	return `<!DOCTYPE html>
<html>
    <head>
        <meta charset="utf-8">
        <title>Hurl File</title>
        <style>
` + hurlCSS + `
        </style>
    </head>
    <body>
` + body + `
    </body>
</html>
`
}

const (
	htmlBase64ValueClass     = "base64"
	htmlBooleanClass         = "boolean"
	htmlCommentClass         = "comment"
	htmlDurationUnitClass    = "unit"
	htmlEntryClass           = "entry"
	htmlHexClass             = "hex"
	htmlFilenameClass        = "filename"
	htmlFilterKindClass      = "filter-type"
	htmlJSONClass            = "json"
	htmlLangClass            = "language-hurl"
	htmlMethodClass          = "method"
	htmlMultilineStringClass = "multiline"
	htmlNullClass            = "null"
	htmlNumberClass          = "number"
	htmlNotClass             = "not"
	htmlPlaceholderClass     = "expr"
	htmlPredicateTypeClass   = "predicate-type"
	htmlQueryTypeClass       = "query-type"
	htmlRegexClass           = "regex"
	htmlRequestClass         = "request"
	htmlResponseClass        = "response"
	htmlSectionHeaderClass   = "section-header"
	htmlStringClass          = "string"
	htmlURLClass             = "url"
	htmlVersionClass         = "version"
	htmlXMLClass             = "xml"
)

type htmlFormatter struct {
	ast.BaseVisitor
	buf strings.Builder
}

func (f *htmlFormatter) preOpen(class string) {
	f.buf.WriteString(`<pre><code class="` + class + `">`)
}

func (f *htmlFormatter) preClose() { f.buf.WriteString("</code></pre>") }

func (f *htmlFormatter) spanOpen(class string) {
	f.buf.WriteString(`<span class="` + class + `">`)
}

func (f *htmlFormatter) spanClose() { f.buf.WriteString("</span>") }

var htmlEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func (f *htmlFormatter) pushUntrusted(s string) { htmlEscaper.WriteString(&f.buf, s) }

func (f *htmlFormatter) pushTrusted(s string) { f.buf.WriteString(s) }

func (f *htmlFormatter) span(class, untrusted string) {
	f.spanOpen(class)
	f.pushUntrusted(untrusted)
	f.spanClose()
}

func (f *htmlFormatter) trustedSpan(class, trusted string) {
	f.spanOpen(class)
	f.pushTrusted(trusted)
	f.spanClose()
}

func (f *htmlFormatter) VisitBase64Value(_ []byte, source string) {
	f.span(htmlBase64ValueClass, source)
}

func (f *htmlFormatter) VisitBool(value bool) {
	f.trustedSpan(htmlBooleanClass, strconv.FormatBool(value))
}

func (f *htmlFormatter) VisitCookiePath(p *ast.CookiePath) {
	f.span(htmlStringClass, p.ToSource())
}

func (f *htmlFormatter) VisitComment(c *ast.Comment) {
	f.span(htmlCommentClass, c.ToSource())
}

func (f *htmlFormatter) VisitDurationUnit(u ast.DurationUnit) {
	f.trustedSpan(htmlDurationUnitClass, u.String())
}

func (f *htmlFormatter) VisitEntry(e *ast.Entry) bool {
	f.spanOpen(htmlEntryClass)
	ast.WalkEntry(f, e)
	f.spanClose()
	return false
}

func (f *htmlFormatter) VisitFilename(filename *ast.Template) {
	f.span(htmlFilenameClass, filename.ToSource())
}

func (f *htmlFormatter) VisitFilterKind(kind ast.FilterValue) {
	f.trustedSpan(htmlFilterKindClass, kind.Identifier())
}

func (f *htmlFormatter) VisitHexValue(_ []byte, source string) {
	f.span(htmlHexClass, source)
}

func (f *htmlFormatter) VisitHurlFile(file *ast.HurlFile) bool {
	f.preOpen(htmlLangClass)
	ast.WalkHurlFile(f, file)
	f.preClose()
	return false
}

func (f *htmlFormatter) VisitI64(n int64) {
	f.trustedSpan(htmlNumberClass, strconv.FormatInt(n, 10))
}

func (f *htmlFormatter) VisitJSONBody(json ast.JSONValue) {
	f.span(htmlJSONClass, json.ToSource())
}

func (f *htmlFormatter) VisitLiteral(lit string) { f.pushTrusted(lit) }

func (f *htmlFormatter) VisitMethod(m ast.Method) {
	f.trustedSpan(htmlMethodClass, m.String())
}

func (f *htmlFormatter) VisitMultilineString(s *ast.MultilineString) {
	f.span(htmlMultilineStringClass, s.ToSource())
}

func (f *htmlFormatter) VisitNot(identifier string) {
	f.trustedSpan(htmlNotClass, identifier)
}

func (f *htmlFormatter) VisitNull(identifier string) {
	f.trustedSpan(htmlNullClass, identifier)
}

func (f *htmlFormatter) VisitNumber(n ast.Number) {
	f.span(htmlNumberClass, n.ToSource())
}

func (f *htmlFormatter) VisitPlaceholder(p *ast.Placeholder) {
	f.span(htmlPlaceholderClass, p.ToSource())
}

func (f *htmlFormatter) VisitPredicateKind(kind ast.PredicateFuncValue) {
	f.span(htmlPredicateTypeClass, kind.ToSource())
}

func (f *htmlFormatter) VisitQueryKind(kind ast.QueryValue) {
	f.trustedSpan(htmlQueryTypeClass, kind.Identifier())
}

func (f *htmlFormatter) VisitRequest(r *ast.Request) bool {
	f.spanOpen(htmlRequestClass)
	ast.WalkRequest(f, r)
	f.spanClose()
	return false
}

func (f *htmlFormatter) VisitResponse(r *ast.Response) bool {
	f.spanOpen(htmlResponseClass)
	ast.WalkResponse(f, r)
	f.spanClose()
	return false
}

func (f *htmlFormatter) VisitRegex(r *ast.Regex) {
	f.span(htmlRegexClass, r.ToSource())
}

func (f *htmlFormatter) VisitStatus(s ast.StatusValue) {
	f.trustedSpan(htmlNumberClass, s.String())
}

func (f *htmlFormatter) VisitString(value string) {
	f.span(htmlStringClass, value)
}

func (f *htmlFormatter) VisitSectionHeader(name string) {
	f.trustedSpan(htmlSectionHeaderClass, name)
}

func (f *htmlFormatter) VisitTemplate(t *ast.Template) {
	f.span(htmlStringClass, t.ToSource())
}

func (f *htmlFormatter) VisitURL(url *ast.Template) {
	f.span(htmlURLClass, url.ToSource())
}

func (f *htmlFormatter) VisitU64(n *ast.U64) {
	f.trustedSpan(htmlNumberClass, n.ToSource())
}

func (f *htmlFormatter) VisitUsize(n int) {
	f.trustedSpan(htmlNumberClass, strconv.Itoa(n))
}

func (f *htmlFormatter) VisitVariableName(name string) { f.pushTrusted(name) }

func (f *htmlFormatter) VisitVersion(v ast.VersionValue) {
	f.trustedSpan(htmlVersionClass, v.String())
}

func (f *htmlFormatter) VisitXMLBody(xml string) {
	f.span(htmlXMLClass, xml)
}

func (f *htmlFormatter) VisitWhitespace(ws *ast.Whitespace) { f.pushTrusted(ws.Value) }
