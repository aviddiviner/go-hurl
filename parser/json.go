package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

const maxNestingDepth = 128

// parseJSON parses a JSON value from the reader.
func parseJSON(r *reader.Reader) (ast.JSONValue, *ParseError) {
	return parseAtDepth(r, 0)
}

// parseAtDepth parses a JSON value, considering the current nesting depth.
func parseAtDepth(r *reader.Reader, depth int) (ast.JSONValue, *ParseError) {
	start := r.Cursor()
	v, err := choice([]func(*reader.Reader) (ast.JSONValue, *ParseError){
		nullValue,
		booleanValue,
		stringValue,
		numberValue,
		expressionValue,
	}, r)
	if err == nil || !err.Recoverable {
		return v, err
	}
	r.Seek(start)

	start = r.Cursor()
	v, err = listValue(r, depth)
	if err == nil || !err.Recoverable {
		return v, err
	}
	r.Seek(start)
	return objectValue(r, depth)
}

// parseInJSON is parseAtDepth, already knowing that we are inside a JSON
// body.
func parseInJSON(r *reader.Reader, depth int) (ast.JSONValue, *ParseError) {
	if c, ok := r.Peek(); ok && c == ',' {
		return nil, newError(r.Cursor().Pos, false, ParseErrorJSON{Value: JSONErrorEmptyElement{}})
	}
	v, err := parseAtDepth(r, depth)
	if err != nil {
		// The only recoverable error comes from object_value's
		// try_literal("{"), but we already know that we are in a JSON body.
		if err.Recoverable {
			return nil, newError(err.Pos, false, ParseErrorJSON{Value: JSONErrorExpectingElement{}})
		}
		return nil, err
	}
	return v, nil
}

func nullValue(r *reader.Reader) (ast.JSONValue, *ParseError) {
	if err := tryLiteral("null", r); err != nil {
		return nil, err
	}
	return ast.JSONNull{}, nil
}

func booleanValue(r *reader.Reader) (ast.JSONValue, *ParseError) {
	v, err := boolean(r)
	if err != nil {
		return nil, err
	}
	return ast.JSONBoolean{Value: v}, nil
}

func stringValue(r *reader.Reader) (ast.JSONValue, *ParseError) {
	t, err := stringTemplate(r)
	if err != nil {
		return nil, err
	}
	return ast.JSONString{Value: t}, nil
}

func stringTemplate(r *reader.Reader) (ast.Template, *ParseError) {
	if err := tryLiteral(`"`, r); err != nil {
		return ast.Template{}, err
	}
	var chars []EncodedChar
	start := r.Cursor()
	for {
		if c, ok := r.Peek(); !ok || c == '"' {
			break
		}
		c, err := jsonAnyChar(r)
		if err != nil {
			return ast.Template{}, err
		}
		chars = append(chars, c)
	}
	end := r.Cursor()

	encoded := EncodedString{SourceInfo: sourceInfo(start.Pos, end.Pos), Chars: chars}
	if err := literal(`"`, r); err != nil {
		return ast.Template{}, err
	}
	elements, err := Templatize(encoded)
	if err != nil {
		return ast.Template{}, err
	}
	return ast.Template{Delimiter: '"', Elements: elements, SourceInfo: sourceInfo(start.Pos, end.Pos)}, nil
}

func jsonAnyChar(r *reader.Reader) (EncodedChar, *ParseError) {
	start := r.Cursor()
	c, err := jsonEscapeChar(r)
	if err == nil {
		return EncodedChar{c, r.ReadFrom(start.Index), start.Pos}, nil
	}
	if !err.Recoverable {
		return EncodedChar{}, err
	}
	r.Seek(start)
	c, ok := r.Read()
	if !ok || c == '\\' || c == '\b' || c == '\n' || c == '\f' || c == '\r' || c == '\t' {
		return EncodedChar{}, newError(start.Pos, true, ParseErrorExpecting{Value: "char"})
	}
	return EncodedChar{c, r.ReadFrom(start.Index), start.Pos}, nil
}

func jsonEscapeChar(r *reader.Reader) (rune, *ParseError) {
	if err := tryLiteral(`\`, r); err != nil {
		return 0, err
	}
	start := r.Cursor()
	c, _ := r.Read()
	switch c {
	case '"', '\\', '/':
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
		return jsonUnicode(r)
	}
	return 0, newError(start.Pos, false, ParseErrorEscapeChar{})
}

// 0xd800-0xdc00 encodes the high 10 bits of a surrogate pair, 0xdc00-0xe000
// the low 10 bits. The value is those 20 bits plus 0x10000.
const (
	surr1    = 0xd800
	surr2    = 0xdc00
	surr3    = 0xe000
	surrSelf = 0x10000
)

func jsonUnicode(r *reader.Reader) (rune, *ParseError) {
	cp, err := jsonHexValue(r)
	if err != nil {
		return 0, err
	}
	if cp >= surr1 && cp < surr3 {
		if err := literal(`\u`, r); err != nil {
			return 0, err
		}
		start := r.Cursor()
		cp2, err := jsonHexValue(r)
		if err != nil {
			return 0, err
		}
		if !(cp >= surr1 && cp < surr2 && cp2 >= surr2 && cp2 < surr3) {
			return 0, newError(start.Pos, false, ParseErrorUnicode{})
		}
		cp = ((cp - surr1) << 10) | ((cp2 - surr2) + surrSelf)
	}
	return rune(cp), nil
}

func jsonHexValue(r *reader.Reader) (int, *ParseError) {
	value := 0
	for range 4 {
		d, err := nonRecover(hexDigit, r)
		if err != nil {
			return 0, err
		}
		value = value*16 + d
	}
	return value, nil
}

func numberValue(r *reader.Reader) (ast.JSONValue, *ParseError) {
	sign := ""
	if tryLiteral("-", r) == nil {
		sign = "-"
	}
	i, err := jsonInteger(r)
	if err != nil {
		return nil, err
	}
	frac, err := jsonFraction(r)
	if err != nil {
		return nil, err
	}
	exp := jsonExponent(r)
	return ast.JSONNumber{Value: sign + i + frac + exp}, nil
}

func jsonInteger(r *reader.Reader) (string, *ParseError) {
	start := r.Cursor()
	if tryLiteral("0", r) == nil {
		return "0", nil
	}
	digits := r.ReadWhile(isASCIIDigit)
	if digits == "" {
		return "", newError(start.Pos, true, ParseErrorExpecting{Value: "number"})
	}
	return digits, nil
}

func jsonFraction(r *reader.Reader) (string, *ParseError) {
	if tryLiteral(".", r) != nil {
		return "", nil
	}
	digits := r.ReadWhile(isASCIIDigit)
	if digits == "" {
		return "", newError(r.Cursor().Pos, false, ParseErrorExpecting{Value: "digits"})
	}
	return "." + digits, nil
}

func jsonExponent(r *reader.Reader) string {
	if c, ok := r.Peek(); !ok || (c != 'e' && c != 'E') {
		return ""
	}
	r.Read()
	sign := ""
	if tryLiteral("-", r) == nil {
		sign = "-"
	} else if tryLiteral("+", r) == nil {
		sign = "+"
	}
	return "e" + sign + r.ReadWhile(isASCIIDigit)
}

func expressionValue(r *reader.Reader) (ast.JSONValue, *ParseError) {
	p, err := placeholder(r)
	if err != nil {
		return nil, err
	}
	return ast.JSONPlaceholder{Value: p}, nil
}

func peekIs(r *reader.Reader, c rune) bool {
	next, ok := r.Peek()
	return ok && next == c
}

func listValue(r *reader.Reader, depth int) (ast.JSONValue, *ParseError) {
	start := r.Cursor()
	if err := tryLiteral("[", r); err != nil {
		return nil, err
	}
	if depth >= maxNestingDepth {
		return nil, newError(start.Pos, false, ParseErrorJSON{Value: JSONErrorMaxDepthExceeded{MaxDepth: maxNestingDepth}})
	}
	space0 := jsonWhitespace(r)
	var elements []ast.JSONListElement

	if !peekIs(r, ']') {
		first, err := listElement(r, depth+1)
		if err != nil {
			return nil, err
		}
		elements = append(elements, first)

		// Stop at anything other than ',' so that a missing last ']' (e.g.
		// [1, 2) is reported as "expecting ']'".
		for !peekIs(r, ']') && peekIs(r, ',') {
			save := r.Cursor()
			if err := literal(",", r); err != nil {
				return nil, err
			}
			// A trailing comma (e.g. [1, 2,]) is common, so report it.
			if c, ok := r.PeekIf(func(c rune) bool { return !isJSONWhitespace(c) }); ok && c == ']' {
				return nil, newError(save.Pos, false, ParseErrorJSON{Value: JSONErrorTrailingComma{}})
			}
			e, err := listElement(r, depth+1)
			if err != nil {
				return nil, err
			}
			elements = append(elements, e)
		}
	}
	if err := literal("]", r); err != nil {
		return nil, err
	}
	return ast.JSONList{Space0: space0, Elements: elements}, nil
}

func listElement(r *reader.Reader, depth int) (ast.JSONListElement, *ParseError) {
	space0 := jsonWhitespace(r)
	v, err := parseInJSON(r, depth)
	if err != nil {
		return ast.JSONListElement{}, err
	}
	space1 := jsonWhitespace(r)
	return ast.JSONListElement{Space0: space0, Value: v, Space1: space1}, nil
}

func objectValue(r *reader.Reader, depth int) (ast.JSONValue, *ParseError) {
	start := r.Cursor()
	if err := tryLiteral("{", r); err != nil {
		return nil, err
	}
	if depth >= maxNestingDepth {
		return nil, newError(start.Pos, false, ParseErrorJSON{Value: JSONErrorMaxDepthExceeded{MaxDepth: maxNestingDepth}})
	}
	space0 := jsonWhitespace(r)
	var elements []ast.JSONObjectElement
	if !peekIs(r, '}') {
		first, err := objectElement(r, depth+1)
		if err != nil {
			return nil, err
		}
		elements = append(elements, first)

		// Stop at anything other than ',' so that a missing last '}' (e.g.
		// {"name": "abc") is reported as "expecting '}'".
		for !peekIs(r, '}') && peekIs(r, ',') {
			save := r.Cursor()
			if err := literal(",", r); err != nil {
				return nil, err
			}
			// A trailing comma (e.g. {"a": "b",}) is common, so report it.
			if c, ok := r.PeekIf(func(c rune) bool { return !isJSONWhitespace(c) }); ok && c == '}' {
				return nil, newError(save.Pos, false, ParseErrorJSON{Value: JSONErrorTrailingComma{}})
			}
			e, err := objectElement(r, depth+1)
			if err != nil {
				return nil, err
			}
			elements = append(elements, e)
		}
	}
	if err := literal("}", r); err != nil {
		return nil, err
	}
	return ast.JSONObject{Space0: space0, Elements: elements}, nil
}

func jsonKey(r *reader.Reader) (ast.Template, *ParseError) {
	return nonRecover(stringTemplate, r)
}

func objectElement(r *reader.Reader, depth int) (ast.JSONObjectElement, *ParseError) {
	space0 := jsonWhitespace(r)
	name, err := jsonKey(r)
	if err != nil {
		return ast.JSONObjectElement{}, err
	}
	space1 := jsonWhitespace(r)
	if err := literal(":", r); err != nil {
		return ast.JSONObjectElement{}, err
	}
	save := r.Cursor()
	space2 := jsonWhitespace(r)
	// An empty element after ':' is common, so report it.
	if c, ok := r.Peek(); !ok || c == '}' {
		return ast.JSONObjectElement{}, newError(save.Pos, false, ParseErrorJSON{Value: JSONErrorEmptyElement{}})
	}
	v, err := parseInJSON(r, depth)
	if err != nil {
		return ast.JSONObjectElement{}, err
	}
	space3 := jsonWhitespace(r)
	return ast.JSONObjectElement{Space0: space0, Name: name, Space1: space1, Space2: space2, Value: v, Space3: space3}, nil
}

func isJSONWhitespace(c rune) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r'
}

func jsonWhitespace(r *reader.Reader) string {
	return r.ReadWhile(isJSONWhitespace)
}
