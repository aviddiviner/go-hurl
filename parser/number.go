package parser

import (
	"errors"
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func natural(r *reader.Reader) (ast.U64, *ParseError) {
	start := r.Cursor()
	s, err := parseNaturalStr(r, "natural")
	if err != nil {
		return ast.U64{}, err
	}
	value, perr := strconv.ParseUint(s, 10, 64)
	if perr != nil {
		return ast.U64{}, newError(start.Pos, false, ParseErrorExpecting{Value: "natural"})
	}
	return ast.U64{Value: value, Source: r.ReadFrom(start.Index)}, nil
}

func integer(r *reader.Reader) (ast.I64, *ParseError) {
	start := r.Cursor()
	sign := int64(1)
	if tryLiteral("-", r) == nil {
		sign = -1
	}
	s, err := parseNaturalStr(r, "integer")
	if err != nil {
		return ast.I64{}, err
	}
	value, perr := strconv.ParseInt(s, 10, 64)
	if perr != nil {
		return ast.I64{}, newError(start.Pos, false, ParseErrorExpecting{Value: "integer"})
	}
	return ast.I64{Value: sign * value, Source: r.ReadFrom(start.Index)}, nil
}

// parseNaturalStr is common code between the natural and integer parsers.
func parseNaturalStr(r *reader.Reader, expecting string) (string, *ParseError) {
	save := r.Cursor()
	firstDigit, ok := r.Read()
	if !ok || !isASCIIDigit(firstDigit) {
		return "", newError(save.Pos, true, ParseErrorExpecting{Value: expecting})
	}
	pos := r.Cursor().Pos
	s := r.ReadWhile(isASCIIDigit)
	// If the first digit is zero, there must not be any more digits.
	if firstDigit == '0' && s != "" {
		return "", newError(pos, false, ParseErrorExpecting{Value: expecting})
	}
	return string(firstDigit) + s, nil
}

func number(r *reader.Reader) (ast.Number, *ParseError) {
	start := r.Cursor()
	sign := ""
	if tryLiteral("-", r) == nil {
		sign = "-"
	}
	integerDigits := r.ReadWhile(isASCIIDigit)
	if integerDigits == "" {
		return nil, newError(r.Cursor().Pos, true, ParseErrorExpecting{Value: "number"})
	} else if len(integerDigits) > 1 && integerDigits[0] == '0' {
		return nil, newError(r.Cursor().Pos, false, ParseErrorExpecting{Value: "natural"})
	}

	if tryLiteral(".", r) == nil {
		save := r.Cursor()
		decimalDigits := r.ReadWhile(isASCIIDigit)
		if decimalDigits == "" {
			return nil, newError(save.Pos, false, ParseErrorExpecting{Value: "decimal digits"})
		}
		value, err := strconv.ParseFloat(sign+integerDigits+"."+decimalDigits, 64)
		// Like Rust's f64 parsing, out of range values are ±inf (or 0).
		if err != nil && !errors.Is(err, strconv.ErrRange) {
			return nil, newError(start.Pos, false, ParseErrorExpecting{Value: "float"})
		}
		return ast.NumberFloat{Value: ast.Float{Value: value, Source: r.ReadFrom(start.Index)}}, nil
	}

	value, err := strconv.ParseInt(sign+integerDigits, 10, 64)
	if err != nil {
		return ast.NumberBigInteger{Value: integerDigits}, nil
	}
	return ast.NumberInteger{Value: ast.I64{Value: value, Source: r.ReadFrom(start.Index)}}, nil
}
