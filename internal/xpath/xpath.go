// Package xpath parses HTML and XML documents and evaluates XPath 1.0 expressions against them,
// replicating Hurl's libxml2-based runner/xpath.rs on top of antchfx/xpath.
package xpath

import (
	"errors"

	axpath "github.com/antchfx/xpath"
)

// Format selects the parser used for a document.
type Format int

const (
	HTML Format = iota
	XML
)

// Error is an XPath evaluation error.
type Error int

const (
	// ErrEval is returned when the expression is invalid or fails to evaluate.
	ErrEval Error = iota
	// ErrUnsupported is returned when the result is not a boolean, number, string or node-set.
	ErrUnsupported
)

func (e Error) Error() string {
	switch e {
	case ErrEval:
		return "xpath eval error"
	default:
		return "xpath unsupported result"
	}
}

// Value is the result of an XPath evaluation: Bool, Number, String or Nodeset.
//
// Hurl maps these to runner values as Value::Bool, Value::Number(Number::Float) (always a
// float, NaN and infinities included, even for integral results like count()),
// Value::String and Value::Nodeset.
type Value interface {
	xpathValue()
}

type (
	Bool   bool
	Number float64
	String string
	// Nodeset is the size of a node-set; Hurl doesn't keep the nodes.
	Nodeset int
)

func (Bool) xpathValue()    {}
func (Number) xpathValue()  {}
func (String) xpathValue()  {}
func (Nodeset) xpathValue() {}

// Document is a parsed HTML or XML document.
type Document struct {
	root   *node
	format Format
	// namespaces maps prefixes to URIs: xml, plus for XML documents the declared prefixes and the
	// first default namespace as "_".
	namespaces map[string]string
	// noNSPrefix is a prefix bound to the empty namespace, used to restrict unprefixed name tests
	// to elements in no namespace when the document has a default namespace.
	noNSPrefix string
}

// Parse parses data as HTML or XML. Like libxml2 in recover mode, malformed input is accepted as
// long as a root element can be built.
func Parse(data string, format Format) (*Document, error) {
	if data == "" {
		return nil, errors.New("invalid input data")
	}
	var root *node
	switch format {
	case HTML:
		var err error
		if root, err = parseHTML(data); err != nil {
			return nil, errors.New("invalid input data")
		}
	default:
		root = parseXML(data)
	}
	if root.rootElement() == nil {
		return nil, errors.New("no root element")
	}
	doc := &Document{root: root, format: format, namespaces: map[string]string{"xml": xmlNamespace}}
	if format == XML {
		doc.registerNamespaces()
	}
	return doc, nil
}

func (d *Document) registerNamespaces() {
	defaultRegistered := false
	for _, ns := range documentNamespaces(d.root) {
		if ns.prefix == "" {
			if !defaultRegistered {
				d.namespaces["_"] = ns.href
				defaultRegistered = true
			}
		} else {
			d.namespaces[ns.prefix] = ns.href
		}
	}
	if !defaultRegistered {
		return
	}
	d.noNSPrefix = "hurl-no-ns"
	for _, ok := d.namespaces[d.noNSPrefix]; ok; _, ok = d.namespaces[d.noNSPrefix] {
		d.noNSPrefix += "_"
	}
	d.namespaces[d.noNSPrefix] = ""
}

// EvalXPath evaluates an XPath 1.0 expression against the document. The error is always an
// Error. HTML documents are evaluated without namespaces, so any prefix other than xml is an
// evaluation error.
func (d *Document) EvalXPath(expr string) (v Value, err error) {
	expr, ok := preprocess(expr, d.namespaces, d.noNSPrefix)
	if !ok {
		return nil, ErrEval
	}
	e, err := axpath.CompileWithNS(expr, d.namespaces)
	if err != nil {
		return nil, ErrEval
	}
	defer func() {
		if recover() != nil {
			v, err = nil, ErrEval
		}
	}()
	switch r := e.Evaluate(newNavigator(d.root)).(type) {
	case bool:
		return Bool(r), nil
	case float64:
		return Number(r), nil
	case string:
		return String(r), nil
	case *axpath.NodeIterator:
		n := 0
		for r.MoveNext() {
			n++
		}
		return Nodeset(n), nil
	default:
		return nil, ErrUnsupported
	}
}
