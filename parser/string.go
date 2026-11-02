package parser

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// unquotedTemplate parses a string until the end of the line (or a comment),
// without trailing spaces, then templatizes it.
func unquotedTemplate(r *reader.Reader) (ast.Template, *ParseError) {
	start := r.Cursor()
	var chars, spaces []EncodedChar
	end := start
	for {
		pos := r.Cursor().Pos
		c, s, err := anyChar([]rune{'#'}, r)
		if err != nil {
			if err.Recoverable {
				break
			}
			return ast.Template{}, err
		}
		if s == "\n" {
			break
		}
		if s == " " {
			spaces = append(spaces, EncodedChar{c, s, pos})
		} else {
			if len(spaces) > 0 {
				chars = append(chars, spaces...)
				spaces = nil
			}
			chars = append(chars, EncodedChar{c, s, pos})
			end = r.Cursor()
		}
	}
	r.Seek(end)
	elements, err := Templatize(EncodedString{SourceInfo: sourceInfo(start.Pos, end.Pos), Chars: chars})
	if err != nil {
		return ast.Template{}, err
	}
	return ast.Template{Elements: elements, SourceInfo: sourceInfo(start.Pos, end.Pos)}, nil
}

func quotedOnelineString(r *reader.Reader) (string, *ParseError) {
	if err := literal(`"`, r); err != nil {
		return "", err
	}
	s := r.ReadWhile(func(c rune) bool { return c != '"' && c != '\n' })
	if err := literal(`"`, r); err != nil {
		return "", err
	}
	return s, nil
}

func quotedTemplate(r *reader.Reader) (ast.Template, *ParseError) {
	return delimitedTemplate('"', []rune{'"'}, r)
}

func backtickTemplate(r *reader.Reader) (ast.Template, *ParseError) {
	return delimitedTemplate('`', []rune{'`', '\n'}, r)
}

func delimitedTemplate(delimiter rune, except []rune, r *reader.Reader) (ast.Template, *ParseError) {
	start := r.Cursor()
	end := start
	if err := tryLiteral(string(delimiter), r); err != nil {
		return ast.Template{}, err
	}
	var chars []EncodedChar
	for {
		pos := r.Cursor().Pos
		save := r.Cursor()
		c, s, err := anyChar(except, r)
		if err != nil {
			if err.Recoverable {
				r.Seek(save)
				break
			}
			return ast.Template{}, err
		}
		chars = append(chars, EncodedChar{c, s, pos})
		end = r.Cursor()
	}
	if err := literal(string(delimiter), r); err != nil {
		return ast.Template{}, err
	}
	elements, err := Templatize(EncodedString{SourceInfo: sourceInfo(start.Pos, end.Pos), Chars: chars})
	if err != nil {
		return ast.Template{}, err
	}
	return ast.Template{Delimiter: delimiter, Elements: elements, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func anyChar(except []rune, r *reader.Reader) (rune, string, *ParseError) {
	start := r.Cursor()
	c, err := escapeChar(r)
	if err == nil {
		return c, r.ReadFrom(start.Index), nil
	}
	if !err.Recoverable {
		return 0, "", err
	}
	r.Seek(start)
	c, ok := r.Read()
	if !ok || slices.Contains(except, c) || strings.ContainsRune("\\\b\n\f\r\t", c) {
		return 0, "", newError(start.Pos, true, ParseErrorExpecting{Value: "char"})
	}
	return c, r.ReadFrom(start.Index), nil
}

func escapeChar(r *reader.Reader) (rune, *ParseError) {
	if err := tryLiteral(`\`, r); err != nil {
		return 0, err
	}
	start := r.Cursor()
	c, _ := r.Read()
	switch c {
	case '#', '"', '`', '\\', '/':
		return c, nil
	case 'b':
		return '\b', nil
	case 'n':
		return '\n', nil
	case 'f':
		return '\f', nil
	case 'r':
		return '\r', nil
	case 't':
		return '\t', nil
	case 'u':
		return unicodeChar(r)
	}
	return 0, newError(start.Pos, false, ParseErrorEscapeChar{})
}

func unicodeChar(r *reader.Reader) (rune, *ParseError) {
	if err := literal("{", r); err != nil {
		return 0, err
	}
	v, err := hexValue(r)
	if err != nil {
		return 0, err
	}
	if v > utf8.MaxRune || !utf8.ValidRune(rune(v)) {
		return 0, newError(r.Cursor().Pos, false, ParseErrorUnicode{})
	}
	if err := literal("}", r); err != nil {
		return 0, err
	}
	return rune(v), nil
}

func hexValue(r *reader.Reader) (uint32, *ParseError) {
	digits, err := oneOrMore(hexDigit, r)
	if err != nil {
		return 0, err
	}
	var v uint32
	for _, d := range digits {
		v = v*16 + uint32(d)
	}
	return v, nil
}
