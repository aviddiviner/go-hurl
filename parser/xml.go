package parser

import (
	"encoding/xml"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/reader"
)

// xmlParse reads an XML body: from the opening '<' up to the end of the root
// element. There is no formal end of body in a Hurl file, so we parse until
// the byte where a complete XML document has been read.
//
// Hurl feeds libxml2's SAX push parser one char at a time and stops at the
// closing root element; an error is reported at the char that the parser
// failed on. We get the same behaviour from encoding/xml: tokens are read
// until the root element is closed, and on error the position is the last
// char consumed by the decoder.
func xmlParse(r *reader.Reader) (string, *ParseError) {
	// If the first char is not the start of an XML text, the error is
	// recoverable. Any subsequent error is not.
	if c, ok := r.Peek(); !ok || c != '<' {
		return "", newError(r.Cursor().Pos, true, ParseErrorXML{})
	}

	start := r.Cursor()
	rest := r.ReadWhile(func(rune) bool { return true })
	r.Seek(start)

	src := &highWaterReader{Reader: strings.NewReader(rest)}
	d := xml.NewDecoder(src)
	// We only check the structure, so any declared encoding is accepted
	// (the text is already decoded).
	d.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) { return input, nil }

	// fail reports an error at the char ending at byte offset.
	fail := func(offset int) *ParseError {
		n := utf8.RuneCountInString(rest[:min(offset, len(rest))])
		r.ReadN(max(n-1, 0))
		return newError(r.Cursor().Pos, false, ParseErrorXML{})
	}

	depth := 0
	for {
		before := int(d.InputOffset())
		tok, err := d.Token()
		if err != nil {
			return "", fail(src.max)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				end := int(d.InputOffset())
				return r.ReadN(utf8.RuneCountInString(rest[:end])), nil
			}
		case xml.CharData:
			// Only whitespace is allowed outside the root element.
			if depth == 0 {
				if i := strings.IndexFunc(string(t), func(c rune) bool { return !isJSONWhitespace(c) }); i >= 0 {
					return "", fail(before + i + 1)
				}
			}
		}
	}
}

// highWaterReader records the furthest byte offset read. The decoder can
// unread the byte it failed on, so this (rather than InputOffset) gives the
// char where the error was detected.
type highWaterReader struct {
	*strings.Reader
	max int
}

func (h *highWaterReader) ReadByte() (byte, error) {
	b, err := h.Reader.ReadByte()
	if err == nil {
		h.max = max(h.max, int(h.Size())-h.Len())
	}
	return b, err
}
