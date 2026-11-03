package runner

import (
	"math"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
)

// Number is a Hurl number: NumberFloat, NumberInteger or NumberBigInteger.
type Number interface {
	number()
	String() string
}

type NumberFloat struct{ Value float64 }
type NumberInteger struct{ Value int64 }

// NumberBigInteger is an integer (or a decimal, from JSON) that doesn't fit
// in an int64, kept as its decimal representation.
type NumberBigInteger struct{ Value string }

func (NumberFloat) number()      {}
func (NumberInteger) number()    {}
func (NumberBigInteger) number() {}

func (n NumberFloat) String() string      { return formatFloat(n.Value) }
func (n NumberInteger) String() string    { return strconv.FormatInt(n.Value, 10) }
func (n NumberBigInteger) String() string { return n.Value }

const epsilon = 2.220446049250313e-16

// formatFloat is Hurl's format_float. Like Rust, a negative fractional value
// also gets ".0" appended (fract() is negative): -1.5 renders as "-1.5.0".
func formatFloat(v float64) string {
	if v-math.Trunc(v) < epsilon {
		return ast.FormatFloat(v) + ".0"
	}
	return ast.FormatFloat(v)
}

// numberEqual is Rust's PartialEq for Number (no cross-variant equality).
func numberEqual(a, b Number) bool {
	switch a := a.(type) {
	case NumberFloat:
		b, ok := b.(NumberFloat)
		return ok && math.Abs(a.Value-b.Value) < epsilon
	case NumberInteger:
		b, ok := b.(NumberInteger)
		return ok && a.Value == b.Value
	case NumberBigInteger:
		b, ok := b.(NumberBigInteger)
		return ok && a.Value == b.Value
	}
	return false
}

// cmpNumber is Number::cmp_value: it returns -1, 0 or +1.
func cmpNumber(a, b Number) int {
	switch a := a.(type) {
	case NumberInteger:
		switch b := b.(type) {
		case NumberInteger:
			switch {
			case a.Value < b.Value:
				return -1
			case a.Value > b.Value:
				return 1
			}
			return 0
		case NumberFloat:
			return compareFloat(float64(a.Value), b.Value)
		}
	case NumberFloat:
		switch b := b.(type) {
		case NumberFloat:
			return compareFloat(a.Value, b.Value)
		case NumberInteger:
			return compareFloat(a.Value, float64(b.Value))
		}
	}
	return compareNumberString(a.String(), b.String())
}

func compareFloat(f1, f2 float64) int {
	switch {
	case f1 > f2:
		return 1
	case f1 < f2:
		return -1
	}
	return 0
}

func compareNumberString(n1, n2 string) int {
	neg1, i1, d1 := numberComponents(n1)
	neg2, i2, d2 := numberComponents(n2)
	switch {
	case neg1 == neg2:
		if c := strings.Compare(i1, i2); c != 0 {
			return c
		}
		return strings.Compare(d1, d2)
	case neg1:
		return -1
	}
	return 1
}

// numberComponents returns (negative, integer, decimals).
func numberComponents(s string) (bool, string, string) {
	if rest, ok := strings.CutPrefix(s, "-"); ok {
		_, i, d := numberComponents(rest)
		return true, i, d
	}
	if index := strings.IndexByte(s, '.'); index >= 0 {
		return false, strings.TrimLeft(s[:index], "0"), strings.TrimRight(s[index+1:], "0")
	}
	return false, strings.TrimLeft(s, "0"), ""
}

// ParseFloat parses s like Rust's f64::from_str: decimal and exponent
// notation, an optional sign, and inf/infinity/nan (case insensitive). Out
// of range values become infinities or zero.
func ParseFloat(s string) (float64, bool) {
	if strings.ContainsAny(s, "_xX") {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return f, true
		}
		return 0, false
	}
	return f, true
}
