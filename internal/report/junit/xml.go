package junit

import (
	"bytes"
	"strings"
)

// xmlDocument is an in-memory XML document, used to read and write JUnit
// reports. Its serialization reproduces the default xml-rs 1.4 emitter used
// by Hurl, and its parsing the default xml-rs reader. XML namespaces are not
// supported.
type xmlDocument struct {
	root *element
}

// xmlNode is an element, a CDATA section, a text or a processing instruction.
type xmlNode interface{ write(w *xmlWriter) }

type element struct {
	name     string
	attrs    []attribute
	children []xmlNode
}

type attribute struct{ name, value string }

type cdata string

type text string

type procInst struct {
	name string
	data *string
}

func newElement(name string) *element { return &element{name: name} }

func (e *element) attr(name, value string) *element {
	e.attrs = append(e.attrs, attribute{name, value})
	return e
}

func (e *element) addChild(child *element) *element {
	e.children = append(e.children, child)
	return e
}

func (e *element) text(s string) *element {
	e.children = append(e.children, text(s))
	return e
}

// Writer

var (
	pcdataEscaper = strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	attrEscaper   = strings.NewReplacer("<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;", "&", "&amp;", "\n", "&#xA;", "\r", "&#xD;")
)

type xmlWriter struct {
	buf                   bytes.Buffer
	startDocumentEmitted  bool
	justWroteStartElement bool
}

func (w *xmlWriter) checkDocumentStarted() {
	if !w.startDocumentEmitted {
		w.startDocumentEmitted = true
		w.buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	}
}

// fixNonEmptyElement closes a pending start tag.
func (w *xmlWriter) fixNonEmptyElement() {
	if w.justWroteStartElement {
		w.justWroteStartElement = false
		w.buf.WriteByte('>')
	}
}

func (d *xmlDocument) bytes() []byte {
	var w xmlWriter
	if d.root != nil {
		d.root.write(&w)
	}
	return w.buf.Bytes()
}

func (e *element) write(w *xmlWriter) {
	w.checkDocumentStarted()
	w.fixNonEmptyElement()
	w.buf.WriteString("<" + e.name)
	for _, a := range e.attrs {
		w.buf.WriteString(" " + a.name + `="` + attrEscaper.Replace(a.value) + `"`)
	}
	w.justWroteStartElement = true
	for _, c := range e.children {
		c.write(w)
	}
	if w.justWroteStartElement {
		w.justWroteStartElement = false
		w.buf.WriteString(" />")
		return
	}
	w.buf.WriteString("</" + e.name + ">")
}

func (t text) write(w *xmlWriter) {
	w.checkDocumentStarted()
	w.fixNonEmptyElement()
	w.buf.WriteString(pcdataEscaper.Replace(string(t)))
}

func (c cdata) write(w *xmlWriter) {
	w.fixNonEmptyElement()
	w.buf.WriteString("<![CDATA[")
	w.buf.WriteString(strings.ReplaceAll(string(c), "]]>", "]]]]><![CDATA[>"))
	w.buf.WriteString("]]>")
}

func (p procInst) write(w *xmlWriter) {
	w.checkDocumentStarted()
	w.fixNonEmptyElement()
	w.buf.WriteString("<?" + p.name)
	if p.data != nil {
		w.buf.WriteString(" " + *p.data)
	}
	w.buf.WriteString("?>")
}
