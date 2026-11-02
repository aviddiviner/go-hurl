package ast

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/reader"
)

type SourceInfo struct {
	Start reader.Pos
	End   reader.Pos
}

type KeyValue struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Key             Template
	Space1          Whitespace
	Space2          Whitespace
	Value           Template
	LineTerminator0 LineTerminator
}

type MultilineString struct {
	Space   Whitespace
	Newline Whitespace
	Kind    MultilineStringKind
}

func (m MultilineString) String() string {
	switch k := m.Kind.(type) {
	case MultilineStringText:
		return k.Value.String()
	case MultilineStringJSON:
		return k.Value.String()
	case MultilineStringXML:
		return k.Value.String()
	case MultilineStringRaw:
		return k.Value.String()
	case MultilineStringGraphQL:
		return k.Value.String()
	}
	return ""
}

func (m MultilineString) ToSource() string {
	return "```" + m.Lang() + m.Space.Value + m.Newline.Value + m.Kind.ToSource() + "```"
}

// Lang returns the language hint of this multiline string ("" for text).
func (m MultilineString) Lang() string {
	switch m.Kind.(type) {
	case MultilineStringJSON:
		return "json"
	case MultilineStringXML:
		return "xml"
	case MultilineStringRaw:
		return "raw"
	case MultilineStringGraphQL:
		return "graphql"
	}
	return ""
}

// Value returns the template content of this multiline string.
func (m MultilineString) Value() Template {
	switch k := m.Kind.(type) {
	case MultilineStringText:
		return k.Value
	case MultilineStringJSON:
		return k.Value
	case MultilineStringXML:
		return k.Value
	case MultilineStringRaw:
		return k.Value
	case MultilineStringGraphQL:
		return k.Value.Value
	}
	return Template{}
}

// MultilineStringKind is one of MultilineStringText, MultilineStringJSON,
// MultilineStringXML, MultilineStringRaw or MultilineStringGraphQL.
type MultilineStringKind interface {
	multilineStringKind()
	ToSource() string
}

type MultilineStringText struct{ Value Template }
type MultilineStringJSON struct{ Value Template }
type MultilineStringXML struct{ Value Template }
type MultilineStringRaw struct{ Value Template }
type MultilineStringGraphQL struct{ Value GraphQL }

func (MultilineStringText) multilineStringKind()    {}
func (MultilineStringJSON) multilineStringKind()    {}
func (MultilineStringXML) multilineStringKind()     {}
func (MultilineStringRaw) multilineStringKind()     {}
func (MultilineStringGraphQL) multilineStringKind() {}

func (k MultilineStringText) ToSource() string    { return k.Value.ToSource() }
func (k MultilineStringJSON) ToSource() string    { return k.Value.ToSource() }
func (k MultilineStringXML) ToSource() string     { return k.Value.ToSource() }
func (k MultilineStringRaw) ToSource() string     { return k.Value.ToSource() }
func (k MultilineStringGraphQL) ToSource() string { return k.Value.ToSource() }

type GraphQL struct {
	Value     Template
	Variables *GraphQLVariables
}

func (g GraphQL) String() string {
	s := g.Value.String()
	if g.Variables != nil {
		s += g.Variables.ToSource()
	}
	return s
}

func (g GraphQL) ToSource() string {
	s := g.Value.ToSource()
	if g.Variables != nil {
		s += g.Variables.ToSource()
	}
	return s
}

type GraphQLVariables struct {
	Space      Whitespace
	Value      JSONValue
	Whitespace Whitespace
}

func (g GraphQLVariables) ToSource() string {
	return "variables" + g.Space.Value + g.Value.ToSource() + g.Whitespace.Value
}

type Base64 struct {
	Space0 Whitespace
	Value  []byte
	Source string
	Space1 Whitespace
}

type File struct {
	Space0   Whitespace
	Filename Template
	Space1   Whitespace
}

type Template struct {
	// Delimiter is the quote delimiting the template ('"' or '`'), or 0 if none.
	Delimiter  rune
	Elements   []TemplateElement
	SourceInfo SourceInfo
}

func (t Template) String() string {
	var b strings.Builder
	for _, e := range t.Elements {
		b.WriteString(e.String())
	}
	return b.String()
}

func (t Template) ToSource() string {
	var b strings.Builder
	if t.Delimiter != 0 {
		b.WriteRune(t.Delimiter)
	}
	for _, e := range t.Elements {
		b.WriteString(e.ToSource())
	}
	if t.Delimiter != 0 {
		b.WriteRune(t.Delimiter)
	}
	return b.String()
}

func (t Template) IsEmpty() bool { return len(t.Elements) == 0 }

// TemplateElement is either a TemplateString or a TemplatePlaceholder.
type TemplateElement interface {
	templateElement()
	String() string
	ToSource() string
}

type TemplateString struct {
	Value  string
	Source string
}

type TemplatePlaceholder struct{ Value Placeholder }

func (TemplateString) templateElement()      {}
func (TemplatePlaceholder) templateElement() {}

func (e TemplateString) String() string      { return e.Value }
func (e TemplatePlaceholder) String() string { return "{{" + e.Value.String() + "}}" }

func (e TemplateString) ToSource() string      { return e.Source }
func (e TemplatePlaceholder) ToSource() string { return e.Value.ToSource() }

type Comment struct {
	Value      string
	SourceInfo SourceInfo
}

func (c Comment) ToSource() string { return "#" + c.Value }

type Whitespace struct {
	Value      string
	SourceInfo SourceInfo
}

func (w Whitespace) String() string { return w.Value }

// Number is one of NumberFloat, NumberInteger or NumberBigInteger.
type Number interface {
	number()
	String() string
	ToSource() string
}

type NumberFloat struct{ Value Float }
type NumberInteger struct{ Value I64 }
type NumberBigInteger struct{ Value string }

func (NumberFloat) number()      {}
func (NumberInteger) number()    {}
func (NumberBigInteger) number() {}

func (n NumberFloat) String() string      { return n.Value.String() }
func (n NumberInteger) String() string    { return n.Value.String() }
func (n NumberBigInteger) String() string { return n.Value }

func (n NumberFloat) ToSource() string      { return n.Value.ToSource() }
func (n NumberInteger) ToSource() string    { return n.Value.ToSource() }
func (n NumberBigInteger) ToSource() string { return n.Value }

// Float is a float literal; Source keeps its spelling (1.01 vs 1.010).
type Float struct {
	Value  float64
	Source string
}

// String formats the value like Rust's f64 Display: shortest round-trip
// digits, never in exponent form.
func (f Float) String() string { return FormatFloat(f.Value) }

func (f Float) ToSource() string { return f.Source }

// FormatFloat formats v like Rust's f64 Display.
func FormatFloat(v float64) string {
	switch {
	case math.IsInf(v, 1):
		return "inf"
	case math.IsInf(v, -1):
		return "-inf"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

type U64 struct {
	Value  uint64
	Source string
}

func (u U64) String() string   { return strconv.FormatUint(u.Value, 10) }
func (u U64) ToSource() string { return u.Source }

type I64 struct {
	Value  int64
	Source string
}

func (i I64) String() string   { return strconv.FormatInt(i.Value, 10) }
func (i I64) ToSource() string { return i.Source }

type LineTerminator struct {
	Space0  Whitespace
	Comment *Comment
	Newline Whitespace
}

// Bytes is a body value: one of BytesJSON, BytesXML, BytesMultilineString,
// BytesOnelineString, BytesBase64, BytesFile or BytesHex.
type Bytes interface{ bytes() }

type BytesJSON struct{ Value JSONValue }
type BytesXML struct{ Value string }
type BytesMultilineString struct{ Value MultilineString }
type BytesOnelineString struct{ Value Template }
type BytesBase64 struct{ Value Base64 }
type BytesFile struct{ Value File }
type BytesHex struct{ Value Hex }

func (BytesJSON) bytes()            {}
func (BytesXML) bytes()             {}
func (BytesMultilineString) bytes() {}
func (BytesOnelineString) bytes()   {}
func (BytesBase64) bytes()          {}
func (BytesFile) bytes()            {}
func (BytesHex) bytes()             {}

type Hex struct {
	Space0 Whitespace
	Value  []byte
	Source string
	Space1 Whitespace
}

func (h Hex) String() string {
	return "hex," + h.Space0.Value + h.Source + h.Space1.Value + ";"
}

// Regex is a literal regex (/.../). Inner is the compiled expression and
// Source its spelling in the Hurl file.
type Regex struct {
	Inner  *regexp.Regexp
	Source string
}

func (r Regex) String() string   { return r.Inner.String() }
func (r Regex) ToSource() string { return r.Source }

// Equal reports whether both regexes have the same pattern.
func (r Regex) Equal(other Regex) bool { return r.Inner.String() == other.Inner.String() }

type Placeholder struct {
	Space0 Whitespace
	Expr   Expr
	Space1 Whitespace
}

func (p Placeholder) String() string { return p.Expr.String() }

func (p Placeholder) ToSource() string {
	return "{{" + p.Space0.Value + p.Expr.ToSource() + p.Space1.Value + "}}"
}

type Expr struct {
	SourceInfo SourceInfo
	Kind       ExprKind
}

func (e Expr) String() string   { return e.Kind.String() }
func (e Expr) ToSource() string { return e.Kind.String() }

// ExprKind is either an ExprVariable or an ExprFunction.
type ExprKind interface {
	exprKind()
	String() string
}

type ExprVariable struct{ Value Variable }
type ExprFunction struct{ Value Function }

func (ExprVariable) exprKind() {}
func (ExprFunction) exprKind() {}

func (e ExprVariable) String() string { return e.Value.String() }
func (e ExprFunction) String() string { return e.Value.String() }

type Variable struct {
	Name       string
	SourceInfo SourceInfo
}

func (v Variable) String() string { return v.Name }

type Function int

const (
	FunctionNewDate Function = iota
	FunctionNewUUID
)

func (f Function) String() string {
	switch f {
	case FunctionNewDate:
		return "newDate"
	case FunctionNewUUID:
		return "newUuid"
	}
	return ""
}
