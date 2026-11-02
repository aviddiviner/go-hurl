package jsonpath

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/reader"
)

// tryLiteral parses a JSON primitive: null, a boolean, a number or a string
// (single or double quoted). As in serde_json, a number with a fraction or an
// exponent is a float (110 is an integer, 110.0 and 1.1e2 are floats).
func tryLiteral(r *reader.Reader) (*literal, *ParseError) {
	if matchStr("null", r) {
		return &literal{value: nil}, nil
	}
	if matchStr("true", r) {
		return &literal{value: true}, nil
	}
	if matchStr("false", r) {
		return &literal{value: false}, nil
	}
	if n, err := tryNumber(r); err != nil {
		return nil, err
	} else if n != nil {
		return &literal{value: n}, nil
	}
	if s, ok, err := tryStringLiteral(r); err != nil {
		return nil, err
	} else if ok {
		return &literal{value: s}, nil
	}
	return nil, nil
}

var (
	errInf = errors.New("infinite float")
	errNaN = errors.New("NaN float")
)

func tryFiniteFloat(v float64) (floatNumber, error) {
	switch {
	case math.IsNaN(v):
		return 0, errNaN
	case math.IsInf(v, 0):
		return 0, errInf
	}
	return floatNumber(v), nil
}

// tryInteger parses an integer relevant to JSONPath processing (an index or a
// step), which must be within the I-JSON exact range ±(2⁵³-1). Integer
// literals are parsed by tryNumber instead, and can be arbitrarily large.
func tryInteger(r *reader.Reader) (int64, bool, *ParseError) {
	savedPos := r.Cursor().Pos
	s, ok, err := tryStringInteger(r)
	if err != nil || !ok {
		return 0, false, err
	}
	value, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		msg := "number too large to fit in target type"
		if strings.HasPrefix(s, "-") {
			msg = "number too small to fit in target type"
		}
		return 0, false, parseError(savedPos, Expecting(msg))
	}
	if value < -9007199254740991 || value > 9007199254740991 {
		return 0, false, parseError(savedPos, Expecting("Integer value inside the allowed range ±(2⁵³-1, 2⁵³-1)"))
	}
	return value, true, nil
}

func tryNumber(r *reader.Reader) (number, *ParseError) {
	save := r.Cursor()
	var stringInteger string
	if matchStr("-0", r) {
		stringInteger = "0"
	} else {
		s, ok, err := tryStringInteger(r)
		if err != nil {
			return nil, err
		}
		if !ok {
			r.Seek(save)
			return nil, nil
		}
		stringInteger = s
	}

	fraction, hasFraction, err := tryFraction(r)
	if err != nil {
		return nil, err
	}
	exponent, hasExponent, err := tryExponent(r)
	if err != nil {
		return nil, err
	}

	if !hasFraction && !hasExponent {
		if v, err := strconv.ParseInt(stringInteger, 10, 64); err == nil {
			return integerNumber(v), nil
		}
		return bigIntegerNumber(stringInteger), nil
	}

	// Hurl builds the float arithmetically rather than parsing the whole
	// lexeme; keep the same steps to get the same values (and quirks: the
	// fraction is always added, so -1.5 is -0.5).
	value, _ := strconv.ParseFloat(stringInteger, 64)
	if hasFraction {
		value += fraction
	}
	if hasExponent && value != 0 {
		value *= powi(10, exponent)
	}
	f, ferr := tryFiniteFloat(value)
	if ferr != nil {
		msg := "Number is too big"
		if ferr == errNaN {
			msg = "NaN is not allowed"
		}
		return nil, parseError(save.Pos, Expecting(msg))
	}
	return f, nil
}

// powi computes a^b like Rust's f64::powi (compiler-rt __powidf2).
func powi(a float64, b int32) float64 {
	recip := b < 0
	r := 1.0
	for {
		if b&1 != 0 {
			r *= a
		}
		b /= 2
		if b == 0 {
			break
		}
		a *= a
	}
	if recip {
		return 1 / r
	}
	return r
}

// tryStringInteger parses a decimal integer as a string.
func tryStringInteger(r *reader.Reader) (string, bool, *ParseError) {
	if matchStr("0", r) {
		return "0", true, nil
	}
	negative := matchStr("-", r)
	savedPos := r.Cursor().Pos
	s := r.ReadWhile(isDigit)
	if s == "" || strings.HasPrefix(s, "0") {
		if negative {
			return "", false, parseError(savedPos, Expecting("strictly positive digit"))
		}
		return "", false, nil
	}
	if negative {
		s = "-" + s
	}
	return s, true, nil
}

func tryFraction(r *reader.Reader) (float64, bool, *ParseError) {
	if !matchStr(".", r) {
		return 0, false, nil
	}
	digits := r.ReadWhile(isDigit)
	if digits == "" {
		return 0, false, parseError(r.Cursor().Pos, Expecting("digit after decimal point"))
	}
	v, _ := strconv.ParseFloat("0."+digits, 64)
	return v, true, nil
}

func tryExponent(r *reader.Reader) (int32, bool, *ParseError) {
	const maxExponentDigits = 9
	if !matchStr("e", r) && !matchStr("E", r) {
		return 0, false, nil
	}
	sign := ""
	if matchStr("+", r) {
		sign = "+"
	} else if matchStr("-", r) {
		sign = "-"
	}
	digits := r.ReadWhile(isDigit)
	if digits == "" {
		return 0, false, parseError(r.Cursor().Pos, Expecting("digit in exponent"))
	}
	if len(digits) > maxExponentDigits {
		return 0, false, parseError(r.Cursor().Pos, Expecting("at most 9 digits in exponent"))
	}
	v, err := strconv.ParseInt(sign+digits, 10, 32)
	if err != nil {
		return 0, false, parseError(r.Cursor().Pos, Expecting(err.Error()))
	}
	return int32(v), true, nil
}

// String literals

// tryStringLiteral parses a single or double quoted string.
func tryStringLiteral(r *reader.Reader) (string, bool, *ParseError) {
	if s, ok, err := tryQuotedString(r, '"'); err != nil || ok {
		return s, ok, err
	}
	return tryQuotedString(r, '\'')
}

func tryQuotedString(r *reader.Reader, quote rune) (string, bool, *ParseError) {
	if !matchStr(string(quote), r) {
		return "", false, nil
	}
	var result strings.Builder
	for !r.IsEOF() {
		pos := r.Cursor().Pos
		ch, _ := r.Peek()
		switch {
		case ch == quote:
			r.Read()
			return result.String(), true, nil
		case ch == '"' || ch == '\'':
			// The other quote is allowed unescaped.
			result.WriteRune(ch)
			r.Read()
		case ch == '\\':
			r.Read()
			escaped, err := parseEscapeSequence(r, quote)
			if err != nil {
				return "", false, err
			}
			result.WriteRune(escaped)
		case isUnescapedChar(ch):
			result.WriteRune(ch)
			r.Read()
		default:
			return "", false, parseError(pos, InvalidCharacter(ch))
		}
	}
	return "", false, parseError(r.Cursor().Pos, Expecting(string(quote)))
}

// isUnescapedChar: %x20-21 / %x23-26 / %x28-5B / %x5D-D7FF / %xE000-10FFFF
func isUnescapedChar(ch rune) bool {
	return 0x20 <= ch && ch <= 0x21 ||
		0x23 <= ch && ch <= 0x26 ||
		0x28 <= ch && ch <= 0x5B ||
		0x5D <= ch && ch <= 0xD7FF ||
		0xE000 <= ch && ch <= 0x10FFFF
}

func parseEscapeSequence(r *reader.Reader, quote rune) (rune, *ParseError) {
	pos := r.Cursor().Pos
	ch, ok := r.Read()
	if !ok {
		return 0, parseError(pos, Expecting("escape character"))
	}
	switch {
	case ch == 'b':
		return '\b', nil
	case ch == 'f':
		return '\f', nil
	case ch == 'n':
		return '\n', nil
	case ch == 'r':
		return '\r', nil
	case ch == 't':
		return '\t', nil
	case ch == '/':
		return '/', nil
	case ch == '\\':
		return '\\', nil
	case ch == quote:
		return quote, nil
	case ch == 'u':
		return parseUnicodeEscape(r)
	}
	return 0, parseError(pos, InvalidEscapeSequence("\\"+string(ch)))
}

func parseUnicodeEscape(r *reader.Reader) (rune, *ParseError) {
	if ch, ok, err := tryNonSurrogate(r); err != nil || ok {
		return ch, err
	}
	if ch, ok, err := trySurrogatePair(r); err != nil || ok {
		return ch, err
	}
	return 0, parseError(r.Cursor().Pos, InvalidUnicodeEscape("invalid unicode escape"))
}

func tryNonSurrogate(r *reader.Reader) (rune, bool, *ParseError) {
	save := r.Cursor()
	digits := make([]rune, 4)
	for i := range digits {
		d, err := hexDigit(r)
		if err != nil {
			return 0, false, err
		}
		digits[i] = d
		if i == 1 && digits[0] == 13 && d >= 8 {
			r.Seek(save)
			return 0, false, nil
		}
	}
	codePoint := digits[0]*4096 + digits[1]*256 + digits[2]*16 + digits[3]
	if !utf8.ValidRune(codePoint) {
		return 0, false, parseError(save.Pos, InvalidUnicodeEscape(fmt.Sprintf("%04X", codePoint)))
	}
	return codePoint, true, nil
}

func trySurrogatePair(r *reader.Reader) (rune, bool, *ParseError) {
	pos := r.Cursor().Pos
	high, ok, err := tryHighSurrogate(r)
	if err != nil || !ok {
		return 0, false, err
	}
	if err := expectStr("\\u", r); err != nil {
		return 0, false, err
	}
	low, err := lowSurrogate(r)
	if err != nil {
		return 0, false, err
	}
	combined := 0x10000 + high<<10 + low
	if !utf8.ValidRune(combined) {
		return 0, false, parseError(pos, InvalidUnicodeEscape(fmt.Sprintf("%06X", combined)))
	}
	return combined, true, nil
}

// tryHighSurrogate returns the 10 value bits of a high surrogate.
func tryHighSurrogate(r *reader.Reader) (rune, bool, *ParseError) {
	if !matchStr("D", r) {
		return 0, false, nil
	}
	c1, err := hexDigit(r)
	if err != nil {
		return 0, false, err
	}
	if c1 < 8 || c1 > 11 {
		return 0, false, nil
	}
	c2, err := hexDigit(r)
	if err != nil {
		return 0, false, err
	}
	c3, err := hexDigit(r)
	if err != nil {
		return 0, false, err
	}
	return (c1-8)*256 + c2*16 + c3, true, nil
}

// lowSurrogate returns the 10 value bits of a low surrogate.
func lowSurrogate(r *reader.Reader) (rune, *ParseError) {
	pos := r.Cursor().Pos
	if expectStr("D", r) != nil {
		return 0, parseError(pos, Expecting("low surrogate"))
	}
	c1, err := hexDigit(r)
	if err != nil {
		return 0, err
	}
	if c1 < 12 {
		return 0, parseError(pos, Expecting("low surrogate"))
	}
	c2, err := hexDigit(r)
	if err != nil {
		return 0, err
	}
	c3, err := hexDigit(r)
	if err != nil {
		return 0, err
	}
	return (c1-12)*256 + c2*16 + c3, nil
}

func hexDigit(r *reader.Reader) (rune, *ParseError) {
	pos := r.Cursor().Pos
	ch, ok := r.Read()
	switch {
	case ok && '0' <= ch && ch <= '9':
		return ch - '0', nil
	case ok && 'a' <= ch && ch <= 'f':
		return ch - 'a' + 10, nil
	case ok && 'A' <= ch && ch <= 'F':
		return ch - 'A' + 10, nil
	}
	return 0, parseError(pos, Expecting("hex digit"))
}
