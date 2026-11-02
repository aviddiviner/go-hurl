package jsonpath

import (
	"math"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func TestTryFiniteFloat(t *testing.T) {
	f, err := tryFiniteFloat(1.23)
	if err != nil || f != 1.23 {
		t.Errorf("got %v, %v", f, err)
	}
	if _, err := tryFiniteFloat(math.Inf(1)); err != errInf {
		t.Errorf("got %v", err)
	}
	if _, err := tryFiniteFloat(math.NaN()); err != errNaN {
		t.Errorf("got %v", err)
	}
}

func TestLiteral(t *testing.T) {
	r := reader.New("null")
	l, _ := tryLiteral(r)
	assertEqual(t, l, &literal{value: nil})
	assertIndex(t, r, 4)

	r = reader.New("true")
	l, _ = tryLiteral(r)
	assertEqual(t, l, &literal{value: true})
	assertIndex(t, r, 4)
}

func TestLiteralError(t *testing.T) {
	r := reader.New("NULL")
	l, err := tryLiteral(r)
	if l != nil || err != nil {
		t.Errorf("got %v, %v", l, err)
	}
	assertIndex(t, r, 0)

	r = reader.New("1e999")
	_, err = tryLiteral(r)
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("Number is too big")})
	assertIndex(t, r, 5)
}

// Numbers

func TestNumber(t *testing.T) {
	tests := []struct {
		input string
		want  number
		index int
	}{
		{"110", integerNumber(110), 3},
		{"99999999999", integerNumber(99999999999), 11},
		{"9223372036854775808", bigIntegerNumber("9223372036854775808"), 19}, // i64::MAX + 1
		{"110.0", floatNumber(110.0), 5},
		{"1.1e2", floatNumber(110.00000000000001), 5},
		// Float
		{"1.0", floatNumber(1.0), 3},
		{"1e2", floatNumber(100.0), 3},
		{"0e309", floatNumber(0.0), 5},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		n, err := tryNumber(r)
		assertEqual(t, err, (*ParseError)(nil))
		assertEqual(t, n, tt.want)
		assertIndex(t, r, tt.index)
	}
}

func TestFloatError(t *testing.T) {
	r := reader.New("1e999")
	_, err := tryNumber(r)
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("Number is too big")})
	assertIndex(t, r, 5)

	r = reader.New("-1e309")
	_, err = tryNumber(r)
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("Number is too big")})
	assertIndex(t, r, 6)
}

func TestStringInteger(t *testing.T) {
	r := reader.New("110")
	s, ok, _ := tryStringInteger(r)
	if !ok || s != "110" {
		t.Errorf("got %q, %v", s, ok)
	}
	assertIndex(t, r, 3)

	r = reader.New("01")
	s, ok, _ = tryStringInteger(r)
	if !ok || s != "0" {
		t.Errorf("got %q, %v", s, ok)
	}
	assertIndex(t, r, 1)
}

func TestStringIntegerError(t *testing.T) {
	_, _, err := tryStringInteger(reader.New("--1"))
	assertEqual(t, err, &ParseError{pos(1, 2), Expecting("strictly positive digit")})
}

func TestInteger(t *testing.T) {
	r := reader.New("1")
	v, ok, _ := tryInteger(r)
	if !ok || v != 1 {
		t.Errorf("got %v, %v", v, ok)
	}
	assertIndex(t, r, 1)

	_, _, err := tryInteger(reader.New("9007199254740992"))
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("Integer value inside the allowed range ±(2⁵³-1, 2⁵³-1)")})
}

func TestFraction(t *testing.T) {
	r := reader.New(".02")
	v, ok, _ := tryFraction(r)
	if !ok || v != 0.02 {
		t.Errorf("got %v, %v", v, ok)
	}
	assertIndex(t, r, 3)

	r = reader.New("e+2")
	if _, ok, _ := tryFraction(r); ok {
		t.Error("expected no fraction")
	}
	assertIndex(t, r, 0)
}

func TestExponent(t *testing.T) {
	tests := []struct {
		input string
		want  int32
		index int
	}{
		{"e-2", -2, 3},
		{"E+2", 2, 3},
		{"e999999999", 999999999, 10},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		v, ok, err := tryExponent(r)
		if !ok || err != nil || v != tt.want {
			t.Errorf("%s: got %v, %v, %v", tt.input, v, ok, err)
		}
		assertIndex(t, r, tt.index)
	}

	r := reader.New("e9999999999")
	_, _, err := tryExponent(r)
	assertEqual(t, err, &ParseError{pos(1, 12), Expecting("at most 9 digits in exponent")})
	assertIndex(t, r, 11)

	r = reader.New("]")
	if _, ok, _ := tryExponent(r); ok {
		t.Error("expected no exponent")
	}
	assertIndex(t, r, 0)
}

// Strings

func TestStringLiteral(t *testing.T) {
	for _, input := range []string{"'store'", "\"store\""} {
		r := reader.New(input)
		s, ok, _ := tryStringLiteral(r)
		if !ok || s != "store" {
			t.Errorf("%s: got %q, %v", input, s, ok)
		}
		assertIndex(t, r, 7)
	}

	r := reader.New("0")
	if _, ok, _ := tryStringLiteral(r); ok {
		t.Error("expected no string")
	}
	assertIndex(t, r, 0)
}

func TestStringLiteralValues(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// escaped quotes
		{`'quoted\' literal'`, "quoted' literal"},
		{`"quoted\" literal"`, `quoted" literal`},
		// standard escape sequences
		{`"line1\nline2"`, "line1\nline2"},
		{`"tab\there"`, "tab\there"},
		{`"back\\slash"`, `back\slash`},
		{`"slash\/here"`, "slash/here"},
		// unicode escapes
		{`"Hello \u0041"`, "Hello A"},
		{`"\u03C0"`, "π"},
		{`"\u00A9"`, "©"},
		{`"\uD83D\uDE00"`, "😀"},
		// mixed quotes
		{`"it's fine"`, "it's fine"},
		{`'say "hello"'`, `say "hello"`},
	}
	for _, tt := range tests {
		s, ok, err := tryStringLiteral(reader.New(tt.input))
		if !ok || err != nil || s != tt.want {
			t.Errorf("%s: got %q, %v, %v", tt.input, s, ok, err)
		}
	}
}

func TestStringLiteralError(t *testing.T) {
	_, _, err := tryStringLiteral(reader.New("'store"))
	assertEqual(t, err, &ParseError{pos(1, 7), Expecting("'")})

	_, _, err = tryStringLiteral(reader.New("\"store"))
	assertEqual(t, err, &ParseError{pos(1, 7), Expecting("\"")})
}

func TestInvalidEscapeSequences(t *testing.T) {
	for _, input := range []string{`"invalid\x escape"`, `"incomplete\u123"`, `"incomplete\u"`} {
		if _, _, err := tryStringLiteral(reader.New(input)); err == nil {
			t.Errorf("%s: expected an error", input)
		}
	}
}

func TestParseEscapeSequence(t *testing.T) {
	r := reader.New("/")
	c, _ := parseEscapeSequence(r, '"')
	assertEqual(t, c, '/')
	assertIndex(t, r, 1)

	// Unicode Character 'GRINNING FACE' (U+1F600)
	r = reader.New(`uD83D\uDE00"`)
	c, _ = parseEscapeSequence(r, '"')
	assertEqual(t, c, '😀')
	assertIndex(t, r, 11)
}

func TestParseUnicodeEscape(t *testing.T) {
	r := reader.New("00E9")
	c, _ := parseUnicodeEscape(r)
	assertEqual(t, c, 'é')
	assertIndex(t, r, 4)

	r = reader.New(`D83D\uDE00"`)
	c, _ = parseUnicodeEscape(r)
	assertEqual(t, c, '😀')
	assertIndex(t, r, 10)
}

func TestNonSurrogate(t *testing.T) {
	r := reader.New("00E9")
	c, ok, _ := tryNonSurrogate(r)
	if !ok || c != 'é' {
		t.Errorf("got %q, %v", c, ok)
	}
	assertIndex(t, r, 4)

	r = reader.New("D83D")
	if _, ok, _ := tryNonSurrogate(r); ok {
		t.Error("expected no non-surrogate")
	}
	assertIndex(t, r, 0)
}

func TestSurrogatePairs(t *testing.T) {
	r := reader.New(`D83D\uDE00"`)
	c, ok, _ := trySurrogatePair(r)
	if !ok || c != '😀' {
		t.Errorf("got %q, %v", c, ok)
	}
	assertIndex(t, r, 10)

	r = reader.New("00E9")
	if _, ok, _ := trySurrogatePair(r); ok {
		t.Error("expected no surrogate pair")
	}
	assertIndex(t, r, 0)

	_, _, err := trySurrogatePair(reader.New(`D83D\u00E9"`))
	assertEqual(t, err, &ParseError{pos(1, 7), Expecting("low surrogate")})
}

func TestHighSurrogate(t *testing.T) {
	r := reader.New("D83D")
	v, ok, _ := tryHighSurrogate(r)
	if !ok || v != 61 {
		t.Errorf("got %v, %v", v, ok)
	}
	assertIndex(t, r, 4)

	r = reader.New("00E9")
	if _, ok, _ := tryHighSurrogate(r); ok {
		t.Error("expected no high surrogate")
	}
	assertIndex(t, r, 0)
}

func TestLowSurrogate(t *testing.T) {
	r := reader.New("DE00")
	v, _ := lowSurrogate(r)
	assertEqual(t, v, rune(512))
	assertIndex(t, r, 4)

	_, err := lowSurrogate(reader.New("00E9"))
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("low surrogate")})
}

func TestHexDigit(t *testing.T) {
	r := reader.New("D83D")
	v, _ := hexDigit(r)
	assertEqual(t, v, rune(13))
	assertIndex(t, r, 1)

	_, err := hexDigit(reader.New("x"))
	assertEqual(t, err, &ParseError{pos(1, 1), Expecting("hex digit")})
}
