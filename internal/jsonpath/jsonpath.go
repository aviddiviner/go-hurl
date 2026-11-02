// Package jsonpath implements JSONPath queries as defined in RFC 9535,
// ported from Hurl's hand-written implementation.
//
// Documents are the values produced by encoding/json with Decoder.UseNumber:
// nil, bool, json.Number, string, []any and map[string]any. Object members
// are visited in sorted key order (Hurl uses serde_json without
// preserve_order), so results are deterministic.
//
// Parse compiles an expression and Query.Eval returns every matching node, in
// order, as a NodeList. There is no notion of a singular result here: Hurl's
// runner applies its own coercion (an empty list is no value, a single node is
// that node, otherwise the whole list as an array).
package jsonpath

import (
	"fmt"

	"github.com/aviddiviner/go-hurl/reader"
)

// NodeList is the list of JSON values selected by a query.
type NodeList = []any

// Query is a parsed JSONPath query, starting with the root identifier $.
type Query struct {
	segments []segment
}

// Parse parses a JSONPath query.
func Parse(s string) (*Query, *ParseError) {
	return parseQuery(reader.New(s))
}

// Eval evaluates the query against the document root. The result is never nil.
func (q *Query) Eval(root any) NodeList {
	if nodes := evalSegments(q.segments, root, root); nodes != nil {
		return nodes
	}
	return NodeList{}
}

// ParseError is a JSONPath syntax error at a position in the expression.
type ParseError struct {
	Pos  reader.Pos
	Kind ParseErrorKind
}

func (e *ParseError) Error() string {
	var msg string
	switch k := e.Kind.(type) {
	case Expecting:
		msg = "expecting " + string(k)
	case InvalidCharacter:
		msg = fmt.Sprintf("invalid character %q", rune(k))
	case InvalidEscapeSequence:
		msg = "invalid escape sequence " + string(k)
	case InvalidUnicodeEscape:
		msg = "invalid unicode escape " + string(k)
	case MaxDepthExceeded:
		msg = fmt.Sprintf("maximum nesting depth %d exceeded", k.MaxDepth)
	}
	return fmt.Sprintf("invalid JSONPath expression at %d:%d: %s", e.Pos.Line, e.Pos.Column, msg)
}

// ParseErrorKind is one of Expecting, InvalidCharacter, InvalidEscapeSequence,
// InvalidUnicodeEscape or MaxDepthExceeded.
type ParseErrorKind interface {
	isParseErrorKind()
}

type (
	Expecting             string
	InvalidCharacter      rune
	InvalidEscapeSequence string
	InvalidUnicodeEscape  string
	MaxDepthExceeded      struct{ MaxDepth int }
)

func (Expecting) isParseErrorKind()             {}
func (InvalidCharacter) isParseErrorKind()      {}
func (InvalidEscapeSequence) isParseErrorKind() {}
func (InvalidUnicodeEscape) isParseErrorKind()  {}
func (MaxDepthExceeded) isParseErrorKind()      {}
