package parser

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func space(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	c, ok := r.Read()
	if !ok || (c != ' ' && c != '\t') {
		return ast.Whitespace{}, newError(start.Pos, true, ParseErrorSpace{})
	}
	return ast.Whitespace{Value: string(c), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func joinWhitespace(v []ast.Whitespace) string {
	var b strings.Builder
	for _, w := range v {
		b.WriteString(w.Value)
	}
	return b.String()
}

func oneOrMoreSpaces(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	v, err := oneOrMore(space, r)
	if err != nil {
		return ast.Whitespace{}, err
	}
	return ast.Whitespace{Value: joinWhitespace(v), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func zeroOrMoreSpaces(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	v, err := zeroOrMore(space, r)
	if err != nil {
		return ast.Whitespace{}, err
	}
	return ast.Whitespace{Value: joinWhitespace(v), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func lineTerminator(r *reader.Reader) (ast.LineTerminator, *ParseError) {
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.LineTerminator{}, err
	}
	c, err := optional(comment, r)
	if err != nil {
		return ast.LineTerminator{}, err
	}
	var nl ast.Whitespace
	if r.IsEOF() {
		nl = ast.Whitespace{SourceInfo: sourceInfo(r.Cursor().Pos, r.Cursor().Pos)}
	} else {
		nl, err = newline(r)
		if err != nil {
			return ast.LineTerminator{}, newError(err.Pos, false, ParseErrorExpecting{Value: "line_terminator"})
		}
	}
	return ast.LineTerminator{Space0: space0, Comment: c, Newline: nl}, nil
}

func optionalLineTerminators(r *reader.Reader) ([]ast.LineTerminator, *ParseError) {
	return zeroOrMore(func(r *reader.Reader) (ast.LineTerminator, *ParseError) {
		return recover(lineTerminator, r)
	}, r)
}

func comment(r *reader.Reader) (ast.Comment, *ParseError) {
	if err := tryLiteral("#", r); err != nil {
		return ast.Comment{}, err
	}
	start := r.Cursor()
	var value strings.Builder
	for !r.IsEOF() {
		save := r.Cursor()
		_, err := newline(r)
		r.Seek(save)
		if err == nil {
			break
		}
		if c, ok := r.Read(); ok {
			value.WriteRune(c)
		}
	}
	return ast.Comment{Value: value.String(), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

// literal consumes s. It is not recoverable; use tryLiteral or recover to
// make it so.
func literal(s string, r *reader.Reader) *ParseError {
	start := r.Cursor()
	for _, c := range s {
		x, ok := r.Read()
		if !ok || x != c {
			return newError(start.Pos, false, ParseErrorExpecting{Value: s})
		}
	}
	return nil
}

// tryLiteral is a recoverable literal that resets the cursor on failure.
func tryLiteral(s string, r *reader.Reader) *ParseError {
	save := r.Cursor()
	if err := literal(s, r); err != nil {
		r.Seek(save)
		return newError(err.Pos, true, err.Kind)
	}
	return nil
}

func newline(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	if tryLiteral("\r\n", r) == nil {
		return ast.Whitespace{Value: "\r\n", SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
	}
	if literal("\n", r) == nil {
		return ast.Whitespace{Value: "\n", SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
	}
	return ast.Whitespace{}, newError(start.Pos, false, ParseErrorExpecting{Value: "newline"})
}

func keyValue(r *reader.Reader) (ast.KeyValue, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	key, err := recover(keyString, r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	if err := recoverLiteral(":", r); err != nil {
		return ast.KeyValue{}, err
	}
	space2, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	value, err := unquotedTemplate(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.KeyValue{}, err
	}
	return ast.KeyValue{
		LineTerminators: lts,
		Space0:          space0,
		Key:             key,
		Space1:          space1,
		Space2:          space2,
		Value:           value,
		LineTerminator0: lt0,
	}, nil
}

// recoverLiteral is recover(|r| literal(s, r)): a recoverable literal that
// does not reset the cursor.
func recoverLiteral(s string, r *reader.Reader) *ParseError {
	if err := literal(s, r); err != nil {
		return err.toRecoverable()
	}
	return nil
}

func hex(r *reader.Reader) (ast.Hex, *ParseError) {
	if err := tryLiteral("hex", r); err != nil {
		return ast.Hex{}, err
	}
	if err := literal(",", r); err != nil {
		return ast.Hex{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Hex{}, err
	}
	var value []byte
	start := r.Cursor()
	current := -1
	for {
		s := r.Cursor()
		d, err := hexDigit(r)
		if err != nil {
			r.Seek(s)
			break
		}
		if current != -1 {
			value = append(value, byte(current*16+d))
			current = -1
		} else {
			current = d
		}
	}
	if current != -1 {
		return ast.Hex{}, newError(r.Cursor().Pos, false, ParseErrorOddNumberOfHexDigits{})
	}
	source := r.ReadFrom(start.Index)
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Hex{}, err
	}
	if err := literal(";", r); err != nil {
		return ast.Hex{}, err
	}
	return ast.Hex{Space0: space0, Value: value, Source: source, Space1: space1}, nil
}

// regex parses a /regex/ literal. Hurl escapes / so that it doesn't end the
// regex; other escape sequences are part of the regex itself.
//
// The pattern is compiled with Go's regexp, so syntax support and
// RegexExpr messages differ from Hurl's (Rust regex crate).
func regex(r *reader.Reader) (ast.Regex, *ParseError) {
	if err := tryLiteral("/", r); err != nil {
		return ast.Regex{}, err
	}
	start := r.Cursor()
	var unescaped, source strings.Builder
	source.WriteByte('/')
	for {
		c, ok := r.Read()
		if !ok {
			return ast.Regex{}, newError(r.Cursor().Pos, false, ParseErrorRegexExpr{Message: "unexpected end of file"})
		}
		source.WriteRune(c)
		if c == '/' {
			break
		}
		if c == '\\' {
			if next, ok := r.Peek(); ok && next == '/' {
				r.Read()
				source.WriteByte('/')
				unescaped.WriteByte('/')
			} else {
				unescaped.WriteByte('\\')
			}
			continue
		}
		unescaped.WriteRune(c)
	}
	inner, err := regexp.Compile(unescaped.String())
	if err != nil {
		message := strings.TrimPrefix(err.Error(), "error parsing regexp: ")
		return ast.Regex{}, newError(start.Pos, false, ParseErrorRegexExpr{Message: message})
	}
	return ast.Regex{Inner: inner, Source: source.String()}, nil
}

func null(r *reader.Reader) *ParseError {
	return tryLiteral("null", r)
}

func boolean(r *reader.Reader) (bool, *ParseError) {
	start := r.Cursor()
	if tryLiteral("true", r) == nil {
		return true, nil
	}
	if literal("false", r) == nil {
		return false, nil
	}
	return false, newError(start.Pos, true, ParseErrorExpecting{Value: "true|false"})
}

func file(r *reader.Reader) (ast.File, *ParseError) {
	if err := tryLiteral("file", r); err != nil {
		return ast.File{}, err
	}
	if err := literal(",", r); err != nil {
		return ast.File{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.File{}, err
	}
	f, err := filename(r)
	if err != nil {
		return ast.File{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.File{}, err
	}
	if err := literal(";", r); err != nil {
		return ast.File{}, err
	}
	return ast.File{Space0: space0, Filename: f, Space1: space1}, nil
}

func base64(r *reader.Reader) (ast.Base64, *ParseError) {
	if err := tryLiteral("base64", r); err != nil {
		return ast.Base64{}, err
	}
	if err := literal(",", r); err != nil {
		return ast.Base64{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Base64{}, err
	}
	save := r.Cursor()
	value := base64Parse(r)
	count := r.Cursor().Index - save.Index
	r.Seek(save)
	source := r.ReadN(count)
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Base64{}, err
	}
	if err := literal(";", r); err != nil {
		return ast.Base64{}, err
	}
	return ast.Base64{Space0: space0, Value: value, Source: source, Space1: space1}, nil
}

func eof(r *reader.Reader) *ParseError {
	if r.IsEOF() {
		return nil
	}
	return newError(r.Cursor().Pos, false, ParseErrorExpecting{Value: "eof"})
}

func hexDigitValue(c rune) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

func hexDigit(r *reader.Reader) (int, *ParseError) {
	start := r.Cursor()
	if c, ok := r.Read(); ok {
		if v, ok := hexDigitValue(c); ok {
			return v, nil
		}
	}
	return 0, newError(start.Pos, true, ParseErrorHexDigit{})
}

// isAlphanumeric matches Rust's char::is_alphanumeric (Alphabetic or Numeric).
func isAlphanumeric(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsNumber(c) || unicode.Is(unicode.Other_Alphabetic, c)
}

// isASCIIAlphanumeric matches Rust's char::is_ascii_alphanumeric.
func isASCIIAlphanumeric(c rune) bool {
	return c < 0x80 && (isASCIIDigit(c) || (c|0x20 >= 'a' && c|0x20 <= 'z'))
}

func isASCIIDigit(c rune) bool { return c >= '0' && c <= '9' }
