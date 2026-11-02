package parser

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// keyString parses a key string (https://hurl.dev/docs/grammar.html#key-string).
func keyString(r *reader.Reader) (ast.Template, *ParseError) {
	start := r.Cursor()
	var elements []ast.TemplateElement
	for {
		save := r.Cursor()
		p, err := placeholder(r)
		if err == nil {
			elements = append(elements, ast.TemplatePlaceholder{Value: p})
			continue
		}
		if !err.Recoverable {
			return ast.Template{}, err
		}
		r.Seek(save)
		value, err := keyStringContent(r)
		if err != nil {
			return ast.Template{}, err
		}
		if value == "" {
			break
		}
		// As in Hurl, the source runs from the start of the key string.
		source := r.ReadFrom(start.Index)
		elements = append(elements, ast.TemplateString{Value: value, Source: source})
	}
	if len(elements) == 0 {
		return ast.Template{}, newError(start.Pos, false, ParseErrorExpecting{Value: "key-string"})
	}
	if s, ok := elements[0].(ast.TemplateString); ok && strings.HasPrefix(s.Source, "[") {
		return ast.Template{}, newError(start.Pos, false, ParseErrorExpecting{Value: "key-string"})
	}
	return ast.Template{Elements: elements, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func keyStringContent(r *reader.Reader) (string, *ParseError) {
	var s strings.Builder
	for {
		c, err := keyStringEscapedChar(r)
		if err == nil {
			s.WriteRune(c)
			continue
		}
		if !err.Recoverable {
			return "", err
		}
		s2 := keyStringText(r)
		if s2 == "" {
			break
		}
		s.WriteString(s2)
	}
	return s.String(), nil
}

func keyStringText(r *reader.Reader) string {
	return r.ReadWhile(func(c rune) bool {
		return isAlphanumeric(c) || strings.ContainsRune("_-.[]@$", c)
	})
}

func keyStringEscapedChar(r *reader.Reader) (rune, *ParseError) {
	if err := tryLiteral(`\`, r); err != nil {
		return 0, err
	}
	start := r.Cursor()
	c, _ := r.Read()
	switch c {
	case '#', ':', '\\', '/':
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
