package html

import (
	"math"
	"strconv"
	"strings"
)

// svgElement is an SVG element, with attributes, children and an optional
// content. Only the elements needed by the Hurl waterfall are supported.
type svgElement struct {
	name     string
	attrs    []svgAttr
	children []*svgElement
	content  *string
}

type svgAttr struct{ name, value string }

func newSVGElement(name string) *svgElement { return &svgElement{name: name} }

func (e *svgElement) addAttr(name, value string) { e.attrs = append(e.attrs, svgAttr{name, value}) }

func (e *svgElement) addAttrF(name string, value float64) { e.addAttr(name, fmtF64(value)) }

func (e *svgElement) addChild(child *svgElement) { e.children = append(e.children, child) }

func (e *svgElement) setContent(content string) { e.content = &content }

func (e *svgElement) String() string {
	var b strings.Builder
	e.toSVG(&b)
	return b.String()
}

func (e *svgElement) toSVG(b *strings.Builder) {
	b.WriteString("<" + e.name)
	if e.name == "svg" {
		b.WriteString(` xmlns="http://www.w3.org/2000/svg"`)
	}
	for _, a := range e.attrs {
		b.WriteString(" " + a.name + `="` + a.value + `"`)
	}
	if len(e.children) == 0 && e.content == nil {
		b.WriteString(" />")
		return
	}
	b.WriteByte('>')
	for _, c := range e.children {
		c.toSVG(b)
	}
	if e.content != nil {
		b.WriteString(*e.content)
	}
	b.WriteString("</" + e.name + ">")
}

// fmtF64 formats f like Rust's f64 Display.
func fmtF64(f float64) string {
	switch {
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func viewBox(minX, minY, width, height float64) string {
	return fmtF64(minX) + " " + fmtF64(minY) + " " + fmtF64(width) + " " + fmtF64(height)
}

func newSVGA(href string) *svgElement {
	e := newSVGElement("a")
	e.addAttr("href", href)
	return e
}

func newSVG() *svgElement { return newSVGElement("svg") }

func newSVGGroup() *svgElement { return newSVGElement("g") }

func newSVGStyle(content string) *svgElement {
	e := newSVGElement("style")
	e.setContent(content)
	return e
}

func newSVGText(x, y float64, content string) *svgElement {
	e := newSVGElement("text")
	e.addAttrF("x", x)
	e.addAttrF("y", y)
	e.setContent(content)
	return e
}

func newSVGLine(x1, y1, x2, y2 float64) *svgElement {
	e := newSVGElement("line")
	e.addAttrF("x1", x1)
	e.addAttrF("y1", y1)
	e.addAttrF("x2", x2)
	e.addAttrF("y2", y2)
	return e
}

func newSVGRect(x, y, width, height float64, fill string) *svgElement {
	e := newSVGElement("rect")
	e.addAttrF("x", x)
	e.addAttrF("y", y)
	e.addAttrF("width", width)
	e.addAttrF("height", height)
	e.addAttr("fill", fill)
	return e
}

func newSVGPath(d string) *svgElement {
	e := newSVGElement("path")
	e.addAttr("d", d)
	return e
}

func newSVGTitle(content string) *svgElement {
	e := newSVGElement("title")
	e.setContent(content)
	return e
}
