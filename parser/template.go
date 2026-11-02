package parser

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// EncodedChar is a decoded rune, with its source text (e.g. `\u{20}` for a
// space) and its position.
type EncodedChar struct {
	Value  rune
	Source string
	Pos    reader.Pos
}

// EncodedString is a sequence of decoded runes, ready to be templatized.
type EncodedString struct {
	SourceInfo ast.SourceInfo
	Chars      []EncodedChar
}

// Templatize splits an encoded string into string and {{placeholder}}
// template elements.
func Templatize(encoded EncodedString) ([]ast.TemplateElement, *ParseError) {
	const (
		stateString = iota
		stateTemplate
		stateFirstOpenBracket
		stateFirstCloseBracket
	)

	var elements []ast.TemplateElement
	var value, source strings.Builder
	state := stateString
	var expressionStart *reader.Pos

	for _, ch := range encoded.Chars {
		c, s, pos := ch.Value, ch.Source, ch.Pos
		switch state {
		case stateString:
			if s == "{" {
				state = stateFirstOpenBracket
			} else {
				value.WriteRune(c)
				source.WriteString(s)
			}

		case stateFirstOpenBracket:
			if s == "{" {
				if value.Len() > 0 {
					elements = append(elements, ast.TemplateString{Value: value.String(), Source: source.String()})
					value.Reset()
					source.Reset()
				}
				state = stateTemplate
			} else {
				value.WriteByte('{')
				source.WriteByte('{')
				value.WriteRune(c)
				source.WriteString(s)
				state = stateString
			}

		case stateTemplate:
			if expressionStart == nil {
				expressionStart = &pos
			}
			if s == "}" {
				state = stateFirstCloseBracket
			} else {
				value.WriteRune(c)
				source.WriteString(s)
			}

		case stateFirstCloseBracket:
			if s == "}" {
				r := reader.WithPos(source.String(), *expressionStart)
				space0, err := zeroOrMoreSpaces(r)
				if err != nil {
					return nil, err
				}
				e, err := expr(r)
				if err != nil {
					return nil, err
				}
				space1, err := zeroOrMoreSpaces(r)
				if err != nil {
					return nil, err
				}
				elements = append(elements, ast.TemplatePlaceholder{Value: ast.Placeholder{Space0: space0, Expr: e, Space1: space1}})
				value.Reset()
				source.Reset()
				expressionStart = nil
				state = stateString
			} else {
				value.WriteByte('}')
				value.WriteRune(c)
				source.WriteByte('}')
				source.WriteString(s)
			}
		}
	}

	switch state {
	case stateFirstOpenBracket:
		value.WriteByte('{')
		source.WriteByte('{')
	case stateTemplate, stateFirstCloseBracket:
		return nil, newError(encoded.SourceInfo.End, false, ParseErrorExpecting{Value: "}}"})
	}

	if value.Len() > 0 {
		elements = append(elements, ast.TemplateString{Value: value.String(), Source: source.String()})
	}
	return elements, nil
}
