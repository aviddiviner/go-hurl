package parser

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// filenamePassword parses a filename with an optional password. Unlike
// filename, ':' (the password delimiter) can be escaped; it is kept escaped
// in the value so that it is not taken as the delimiter later.
func filenamePassword(r *reader.Reader) (ast.Template, *ParseError) {
	return filenameTemplate(filenamePasswordContent, r)
}

func filenamePasswordContent(r *reader.Reader) (string, *ParseError) {
	var s strings.Builder
	for {
		c, err := filenamePasswordEscapedChar(r)
		if err == nil {
			if c == ':' {
				s.WriteByte('\\')
			}
			s.WriteRune(c)
			continue
		}
		if !err.Recoverable {
			return "", err
		}
		s2 := filenameText(r)
		if s2 == "" {
			break
		}
		s.WriteString(s2)
	}
	return s.String(), nil
}

func filenamePasswordEscapedChar(r *reader.Reader) (rune, *ParseError) {
	if err := tryLiteral(`\`, r); err != nil {
		return 0, err
	}
	start := r.Cursor()
	c, _ := r.Read()
	switch c {
	case '\\', '#', ';', ' ', '{', '}', ':':
		return c, nil
	case 'b':
		return '\b', nil
	case 'f':
		return '\f', nil
	case 'n':
		return '\n', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		return unicodeChar(r)
	}
	return 0, newError(start.Pos, false, ParseErrorEscapeChar{})
}
