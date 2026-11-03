package junit

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
)

// errInvalidXML is a structural error of a parsed XML document.
type errInvalidXML string

func (e errInvalidXML) Error() string { return string(e) }

// parseXML parses an XML document like the default xml-rs reader used by
// Hurl: comments are ignored, adjacent texts are coalesced (across comments),
// whitespace-only texts are ignored and CDATA sections are kept.
func parseXML(data []byte) (*xmlDocument, error) {
	toks, err := tokenize(data)
	if err != nil {
		return nil, err
	}
	doc := &xmlDocument{}
	for i := 0; i < len(toks); {
		switch tok := toks[i].(type) {
		case xml.StartElement:
			if doc.root != nil {
				return nil, errInvalidXML("Invalid start of document")
			}
			doc.root, i, err = parseElement(toks, i)
			if err != nil {
				return nil, err
			}
		case xml.EndElement:
			return nil, errInvalidXML("Invalid end of element")
		case text:
			return nil, errInvalidXML("Unexpected characters outside the root element: " + string(tok))
		default:
			i++
		}
	}
	return doc, nil
}

// parseElement parses the element starting at toks[i], and returns the index
// of the next token.
func parseElement(toks []any, i int) (*element, int, error) {
	start := toks[i].(xml.StartElement)
	elt := newElement(start.Name.Local)
	for _, a := range start.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		name := a.Name.Local
		if a.Name.Space != "" {
			name = a.Name.Space + ":" + name
		}
		elt.attr(name, a.Value)
	}
	for i++; i < len(toks); {
		switch tok := toks[i].(type) {
		case xml.Directive:
			return nil, 0, errInvalidXML("Invalid doc type")
		case xml.ProcInst:
			if tok.Target == "xml" {
				return nil, 0, errInvalidXML("Invalid start of document")
			}
			pi := procInst{name: tok.Target}
			if len(tok.Inst) > 0 {
				data := string(tok.Inst)
				pi.data = &data
			}
			elt.children = append(elt.children, pi)
			i++
		case xml.StartElement:
			child, next, err := parseElement(toks, i)
			if err != nil {
				return nil, 0, err
			}
			elt.children = append(elt.children, child)
			i = next
		case xml.EndElement:
			if tok.Name != start.Name {
				name := tok.Name.Local
				if tok.Name.Space != "" {
					name = tok.Name.Space + ":" + name
				}
				return nil, 0, errInvalidXML("Bag closing element " + name)
			}
			return elt, i + 1, nil
		case text:
			elt.children = append(elt.children, tok)
			i++
		case cdata:
			elt.children = append(elt.children, tok)
			i++
		}
	}
	return nil, 0, errInvalidXML("Invalid stop of document")
}

// tokenize returns the raw tokens of data, with comments removed, texts
// coalesced and whitespace-only texts removed. Texts are text values and
// CDATA sections cdata values.
func tokenize(data []byte) ([]any, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	var toks []any
	var buf []byte
	inText := false
	flush := func() {
		if inText && !isWhitespace(buf) {
			toks = append(toks, text(buf))
		}
		buf = nil
		inText = false
	}
	for {
		offset := dec.InputOffset()
		tok, err := dec.RawToken()
		if errors.Is(err, io.EOF) {
			flush()
			return toks, nil
		}
		if err != nil {
			return nil, err
		}
		switch tok := tok.(type) {
		case xml.Comment:
		case xml.CharData:
			if bytes.HasPrefix(data[offset:], []byte("<![CDATA[")) {
				flush()
				toks = append(toks, cdata(tok))
				continue
			}
			buf = append(buf, tok...)
			inText = true
		default:
			flush()
			toks = append(toks, xml.CopyToken(tok))
		}
	}
}

func isWhitespace(b []byte) bool {
	for _, c := range b {
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			return false
		}
	}
	return true
}
