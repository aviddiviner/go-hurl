package format

import (
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/text"
)

// Text returns the Hurl file as Hurl text, using ANSI escape codes if color
// is true.
func Text(file *ast.HurlFile, color bool) string {
	format := text.FormatPlain
	if color {
		format = text.FormatANSI
	}
	f := &textFormatter{}
	ast.Walk(f, file)
	return f.buf.Render(format)
}

type textFormatter struct {
	ast.BaseVisitor
	buf text.StyledString
}

var (
	textGreen       = text.NewStyle().Green()
	textCyan        = text.NewStyle().Cyan()
	textYellow      = text.NewStyle().Yellow()
	textMagenta     = text.NewStyle().Magenta()
	textBrightBlack = text.NewStyle().BrightBlack()
)

func (f *textFormatter) VisitBase64Value(_ []byte, source string) {
	f.buf.PushWith(source, textGreen)
}

func (f *textFormatter) VisitBool(value bool) {
	f.buf.PushWith(strconv.FormatBool(value), textCyan)
}

func (f *textFormatter) VisitCookiePath(p *ast.CookiePath) {
	f.buf.PushWith(p.ToSource(), textGreen)
}

func (f *textFormatter) VisitComment(c *ast.Comment) {
	f.buf.PushWith(c.ToSource(), textBrightBlack)
}

func (f *textFormatter) VisitDurationUnit(u ast.DurationUnit) {
	f.buf.PushWith(u.String(), textCyan)
}

func (f *textFormatter) VisitFilename(filename *ast.Template) {
	f.buf.PushWith(filename.ToSource(), textGreen)
}

func (f *textFormatter) VisitFilterKind(kind ast.FilterValue) {
	f.buf.PushWith(kind.Identifier(), textYellow)
}

func (f *textFormatter) VisitHexValue(_ []byte, source string) {
	f.buf.PushWith(source, textGreen)
}

func (f *textFormatter) VisitI64(n int64) {
	f.buf.PushWith(strconv.FormatInt(n, 10), textCyan)
}

func (f *textFormatter) VisitJSONBody(json ast.JSONValue) {
	f.buf.PushWith(json.ToSource(), textGreen)
}

func (f *textFormatter) VisitLiteral(lit string) { f.buf.Push(lit) }

func (f *textFormatter) VisitMethod(m ast.Method) {
	f.buf.PushWith(m.ToSource(), textYellow)
}

func (f *textFormatter) VisitMultilineString(s *ast.MultilineString) {
	f.buf.PushWith(s.ToSource(), textGreen)
}

func (f *textFormatter) VisitNot(identifier string) {
	f.buf.PushWith(identifier, textYellow)
}

func (f *textFormatter) VisitNull(identifier string) {
	f.buf.PushWith(identifier, textCyan)
}

func (f *textFormatter) VisitNumber(n ast.Number) {
	f.buf.PushWith(n.ToSource(), textCyan)
}

func (f *textFormatter) VisitPlaceholder(p *ast.Placeholder) {
	f.buf.PushWith(p.ToSource(), textGreen)
}

func (f *textFormatter) VisitPredicateKind(kind ast.PredicateFuncValue) {
	f.buf.PushWith(kind.Identifier(), textYellow)
}

func (f *textFormatter) VisitQueryKind(kind ast.QueryValue) {
	f.buf.PushWith(kind.Identifier(), textCyan)
}

func (f *textFormatter) VisitRegex(r *ast.Regex) {
	f.buf.PushWith(r.ToSource(), textGreen)
}

func (f *textFormatter) VisitStatus(s ast.StatusValue) { f.buf.Push(s.String()) }

func (f *textFormatter) VisitString(value string) { f.buf.PushWith(value, textGreen) }

func (f *textFormatter) VisitSectionHeader(name string) {
	f.buf.PushWith(name, textMagenta)
}

func (f *textFormatter) VisitTemplate(t *ast.Template) {
	f.buf.PushWith(t.ToSource(), textGreen)
}

func (f *textFormatter) VisitURL(url *ast.Template) {
	f.buf.PushWith(url.ToSource(), textGreen)
}

func (f *textFormatter) VisitU64(n *ast.U64) { f.buf.PushWith(n.ToSource(), textCyan) }

func (f *textFormatter) VisitUsize(n int) { f.buf.PushWith(strconv.Itoa(n), textCyan) }

func (f *textFormatter) VisitVariableName(name string) { f.buf.Push(name) }

func (f *textFormatter) VisitVersion(v ast.VersionValue) { f.buf.Push(v.String()) }

func (f *textFormatter) VisitXMLBody(xml string) { f.buf.PushWith(xml, textGreen) }

func (f *textFormatter) VisitWhitespace(ws *ast.Whitespace) { f.buf.Push(ws.Value) }
