package runner

import (
	"math"
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
)

// evalToFloat converts a number or a string to a float.
func evalToFloat(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	switch v := value.(type) {
	case ValueNumber:
		switch n := v.Value.(type) {
		case NumberFloat:
			return v, nil
		case NumberInteger:
			return NewFloat(float64(n.Value)), nil
		case NumberBigInteger:
			return nil, invalidInputValue(Repr(value)+" is too big to be cast as a float", si, assert)
		}
	case ValueString:
		if f, ok := ParseFloat(v.Value); ok {
			return NewFloat(f), nil
		}
		return nil, invalidInputValue(Repr(value), si, assert)
	}
	return nil, invalidInputType(value, "float, integer or string", si, assert)
}

// evalToInt converts a number or a string to an integer (floats are
// truncated).
func evalToInt(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	switch v := value.(type) {
	case ValueNumber:
		switch n := v.Value.(type) {
		case NumberInteger:
			return v, nil
		case NumberFloat:
			return NewInteger(floatToInt(n.Value)), nil
		}
	case ValueString:
		if i, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
			return NewInteger(i), nil
		}
		return nil, invalidInputValue(Repr(value), si, assert)
	}
	return nil, invalidInputType(value, "float, integer or string", si, assert)
}

// floatToInt is Rust's saturating `f as i64` (NaN is 0).
func floatToInt(f float64) int64 {
	switch {
	case math.IsNaN(f):
		return 0
	case f >= math.MaxInt64:
		return math.MaxInt64
	case f <= math.MinInt64:
		return math.MinInt64
	}
	return int64(f)
}

// evalToHex converts bytes to a lowercase hexadecimal string.
func evalToHex(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	b, ok := value.(ValueBytes)
	if !ok {
		return nil, invalidInputType(value, "bytes", si, assert)
	}
	return ValueString{hexEncode(b.Value)}, nil
}

// evalToString converts a renderable value to a string.
func evalToString(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	if s, ok := Render(value); ok {
		return ValueString{s}, nil
	}
	return nil, invalidInputValue(Repr(value)+" can not be converted to a string", si, assert)
}

// evalLocation returns the redirection URL of an HTTP response.
func evalLocation(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	r, ok := value.(ValueHTTPResponse)
	if !ok {
		return nil, invalidInputType(value, "http response", si, assert)
	}
	if loc := r.Value.Location(); loc != nil {
		return ValueString{loc.Raw()}, nil
	}
	return nil, nil
}
