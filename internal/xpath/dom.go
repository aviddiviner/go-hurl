package xpath

import (
	"encoding/xml"
	"io"
	"strings"

	axpath "github.com/antchfx/xpath"
	"golang.org/x/net/html"
)

const xmlNamespace = "http://www.w3.org/XML/1998/namespace"

type node struct {
	kind                axpath.NodeType
	local, prefix, uri  string
	data                string
	attrs               []attr
	nsDecls             []namespace
	parent, first, last *node
	prev, next          *node
}

type attr struct {
	local, prefix, uri, value string
}

type namespace struct {
	prefix, href string
}

func (n *node) appendChild(c *node) {
	c.parent = n
	if n.last == nil {
		n.first = c
	} else {
		n.last.next = c
		c.prev = n.last
	}
	n.last = c
}

func (n *node) rootElement() *node {
	for c := n.first; c != nil; c = c.next {
		if c.kind == axpath.ElementNode {
			return c
		}
	}
	return nil
}

func (n *node) lookupNamespace(prefix string) (string, bool) {
	if prefix == "xml" {
		return xmlNamespace, true
	}
	for e := n; e != nil; e = e.parent {
		for _, ns := range e.nsDecls {
			if ns.prefix == prefix {
				return ns.href, true
			}
		}
	}
	return "", false
}

// parseXML builds a tree the way libxml2 does in recover mode: an end tag closes the current
// element whatever its name, unclosed elements are closed at EOF, and parsing stops at the first
// syntax error or at any content after the root element.
func parseXML(data string) *node {
	doc := &node{kind: axpath.RootNode}
	d := xml.NewDecoder(strings.NewReader(data))
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }

	cur := doc
	for {
		tok, err := d.RawToken()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if cur == doc && doc.rootElement() != nil {
				return doc
			}
			cur.appendChild(newXMLElement(t))
			cur = cur.last
			resolveXMLNames(cur, t)
		case xml.EndElement:
			if cur == doc {
				return doc
			}
			cur = cur.parent
		case xml.CharData:
			if cur == doc {
				if !isBlank(string(t)) {
					return doc
				}
				continue
			}
			cur.appendChild(&node{kind: axpath.TextNode, data: string(t)})
		case xml.Comment:
			cur.appendChild(&node{kind: axpath.CommentNode, data: string(t)})
		}
	}
	return doc
}

func newXMLElement(t xml.StartElement) *node {
	n := &node{kind: axpath.ElementNode}
	for _, a := range t.Attr {
		switch {
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			n.nsDecls = append(n.nsDecls, namespace{"", a.Value})
		case a.Name.Space == "xmlns":
			n.nsDecls = append(n.nsDecls, namespace{a.Name.Local, a.Value})
		}
	}
	return n
}

// resolveXMLNames sets the element and attribute names once n is attached to its parent, so that
// in-scope namespaces can be looked up. A name with an unbound prefix keeps its prefix in the
// local name and has no namespace, like libxml2.
func resolveXMLNames(n *node, t xml.StartElement) {
	n.local, n.prefix, n.uri = resolveName(n, t.Name, true)
	for _, a := range t.Attr {
		if (a.Name.Space == "" && a.Name.Local == "xmlns") || a.Name.Space == "xmlns" {
			continue
		}
		local, prefix, uri := resolveName(n, a.Name, false)
		n.attrs = append(n.attrs, attr{local, prefix, uri, a.Value})
	}
}

func resolveName(n *node, name xml.Name, useDefault bool) (local, prefix, uri string) {
	if name.Space == "" {
		if useDefault {
			uri, _ = n.lookupNamespace("")
		}
		return name.Local, "", uri
	}
	if uri, ok := n.lookupNamespace(name.Space); ok {
		return name.Local, name.Space, uri
	}
	return name.Space + ":" + name.Local, "", ""
}

// documentNamespaces returns all namespace declarations of the root element and its descendants,
// in document order.
func documentNamespaces(doc *node) []namespace {
	var all []namespace
	var walk func(*node)
	walk = func(n *node) {
		all = append(all, n.nsDecls...)
		for c := n.first; c != nil; c = c.next {
			walk(c)
		}
	}
	if root := doc.rootElement(); root != nil {
		walk(root)
	}
	return all
}

func parseHTML(data string) (*node, error) {
	if !htmlHasRoot(data) {
		return &node{kind: axpath.RootNode}, nil
	}
	h, err := html.Parse(strings.NewReader(trimAfterBody(data)))
	if err != nil {
		return nil, err
	}
	return convertHTML(h), nil
}

func isBlank(s string) bool {
	return strings.Trim(s, " \t\r\n") == ""
}

// trimAfterBody drops the whitespace following the final </body> or </html>. HTML5 appends it to
// the body text, whereas libxml2 keeps it out of <body>.
func trimAfterBody(data string) string {
	z := html.NewTokenizer(strings.NewReader(data))
	cut, offset := -1, 0
	var tail strings.Builder
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		raw := string(z.Raw())
		offset += len(raw)
		name, _ := z.TagName()
		switch {
		case tt == html.EndTagToken && (string(name) == "body" || string(name) == "html"):
			if cut < 0 {
				cut = offset
			} else {
				tail.WriteString(raw)
			}
		case cut >= 0 && tt == html.CommentToken:
			tail.WriteString(raw)
		case cut >= 0 && tt == html.TextToken && isBlank(raw):
		default:
			cut = -1
			tail.Reset()
		}
	}
	if cut < 0 {
		return data
	}
	return data[:cut] + tail.String()
}

// htmlHasRoot reports whether libxml2's HTML parser would create a root element, which it does
// on the first start tag or non-blank text. x/net/html always creates <html>.
func htmlHasRoot(data string) bool {
	z := html.NewTokenizer(strings.NewReader(data))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return false
		case html.StartTagToken, html.SelfClosingTagToken:
			return true
		case html.TextToken:
			if !isBlank(string(z.Text())) {
				return true
			}
		}
	}
}

// convertHTML converts an x/net/html tree. Element and attribute names are lowercased and
// foreign attribute namespaces folded into the name, as libxml2 has no namespaces in HTML.
func convertHTML(h *html.Node) *node {
	var n *node
	switch h.Type {
	case html.DocumentNode:
		n = &node{kind: axpath.RootNode}
	case html.ElementNode:
		n = &node{kind: axpath.ElementNode, local: strings.ToLower(h.Data)}
		for _, a := range h.Attr {
			name := strings.ToLower(a.Key)
			if a.Namespace != "" {
				name = a.Namespace + ":" + name
			}
			n.attrs = append(n.attrs, attr{local: name, value: a.Val})
		}
	case html.TextNode:
		return &node{kind: axpath.TextNode, data: h.Data}
	case html.CommentNode:
		return &node{kind: axpath.CommentNode, data: h.Data}
	default:
		return nil
	}
	for c := h.FirstChild; c != nil; c = c.NextSibling {
		if cn := convertHTML(c); cn != nil {
			n.appendChild(cn)
		}
	}
	return n
}

// navigator implements axpath.NodeNavigator over our tree. attr is the index of the current
// attribute of cur, or -1.
type navigator struct {
	root, cur *node
	attr      int
}

func newNavigator(root *node) *navigator {
	return &navigator{root: root, cur: root, attr: -1}
}

func (n *navigator) NodeType() axpath.NodeType {
	if n.attr >= 0 {
		return axpath.AttributeNode
	}
	return n.cur.kind
}

func (n *navigator) LocalName() string {
	if n.attr >= 0 {
		return n.cur.attrs[n.attr].local
	}
	return n.cur.local
}

func (n *navigator) Prefix() string {
	if n.attr >= 0 {
		return n.cur.attrs[n.attr].prefix
	}
	return n.cur.prefix
}

// NamespaceURL is used by antchfx/xpath for prefixed name tests and namespace-uri().
func (n *navigator) NamespaceURL() string {
	if n.attr >= 0 {
		return n.cur.attrs[n.attr].uri
	}
	return n.cur.uri
}

func (n *navigator) Value() string {
	if n.attr >= 0 {
		return n.cur.attrs[n.attr].value
	}
	switch n.cur.kind {
	case axpath.TextNode, axpath.CommentNode:
		return n.cur.data
	}
	var sb strings.Builder
	var walk func(*node)
	walk = func(e *node) {
		for c := e.first; c != nil; c = c.next {
			switch c.kind {
			case axpath.TextNode:
				sb.WriteString(c.data)
			case axpath.ElementNode:
				walk(c)
			}
		}
	}
	walk(n.cur)
	return sb.String()
}

func (n *navigator) Copy() axpath.NodeNavigator {
	c := *n
	return &c
}

func (n *navigator) MoveToRoot() {
	n.cur, n.attr = n.root, -1
}

func (n *navigator) MoveToParent() bool {
	if n.attr >= 0 {
		n.attr = -1
		return true
	}
	if n.cur.parent == nil {
		return false
	}
	n.cur = n.cur.parent
	return true
}

func (n *navigator) MoveToNextAttribute() bool {
	if n.attr+1 >= len(n.cur.attrs) {
		return false
	}
	n.attr++
	return true
}

func (n *navigator) MoveToChild() bool {
	if n.attr >= 0 || n.cur.first == nil {
		return false
	}
	n.cur = n.cur.first
	return true
}

func (n *navigator) MoveToFirst() bool {
	if n.attr >= 0 || n.cur.parent == nil {
		return false
	}
	n.cur = n.cur.parent.first
	return true
}

func (n *navigator) MoveToNext() bool {
	if n.attr >= 0 || n.cur.next == nil {
		return false
	}
	n.cur = n.cur.next
	return true
}

func (n *navigator) MoveToPrevious() bool {
	if n.attr >= 0 || n.cur.prev == nil {
		return false
	}
	n.cur = n.cur.prev
	return true
}

func (n *navigator) MoveTo(other axpath.NodeNavigator) bool {
	o, ok := other.(*navigator)
	if !ok || o.root != n.root {
		return false
	}
	*n = *o
	return true
}
