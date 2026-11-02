package parser

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// filename parses a filename. As in a shell, a few characters (such as
// space) must be escaped: file\ with\ space.txt
func filename(r *reader.Reader) (ast.Template, *ParseError) {
	return filenameTemplate(filenameContent, r)
}

func filenameTemplate(content func(*reader.Reader) (string, *ParseError), r *reader.Reader) (ast.Template, *ParseError) {
	start := r.Cursor()
	var elements []ast.TemplateElement
	for {
		elementStart := r.Cursor()
		p, err := placeholder(r)
		if err == nil {
			elements = append(elements, ast.TemplatePlaceholder{Value: p})
			continue
		}
		if !err.Recoverable {
			return ast.Template{}, err
		}
		value, err := content(r)
		if err != nil {
			return ast.Template{}, err
		}
		if value == "" {
			break
		}
		elements = append(elements, ast.TemplateString{Value: value, Source: r.ReadFrom(elementStart.Index)})
	}
	if len(elements) == 0 {
		return ast.Template{}, newError(start.Pos, false, ParseErrorFilename{})
	}
	if s, ok := elements[0].(ast.TemplateString); ok && strings.HasPrefix(s.Source, "[") {
		return ast.Template{}, newError(start.Pos, false, ParseErrorExpecting{Value: "filename"})
	}
	return ast.Template{Elements: elements, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func filenameContent(r *reader.Reader) (string, *ParseError) {
	var s strings.Builder
	for {
		c, err := filenameEscapedChar(r)
		if err == nil {
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

func filenameText(r *reader.Reader) string {
	return r.ReadWhile(func(c rune) bool { return !strings.ContainsRune("#;{} \n\r\\", c) })
}

func filenameEscapedChar(r *reader.Reader) (rune, *ParseError) {
	if err := tryLiteral(`\`, r); err != nil {
		return 0, err
	}
	start := r.Cursor()
	c, _ := r.Read()
	switch c {
	case '\\', '#', ';', ' ', '{', '}':
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
