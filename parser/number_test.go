package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestNatural(t *testing.T) {
	r := reader.New("0")
	assertEqual(t, must(natural(r)), ast.U64{Value: 0, Source: "0"})
	assertCursor(t, r, 1)

	r = reader.New("10x")
	assertEqual(t, must(natural(r)), ast.U64{Value: 10, Source: "10"})
	assertCursor(t, r, 2)
}

func TestNaturalError(t *testing.T) {
	tests := []struct {
		input       string
		pos         reader.Pos
		recoverable bool
	}{
		{"", pos(1, 1), true},
		{"01", pos(1, 2), false},
		{"x", pos(1, 1), true},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		_, err := natural(r)
		assertError(t, err, tt.pos, tt.recoverable, ParseErrorExpecting{Value: "natural"})
	}
}

func TestInteger(t *testing.T) {
	r := reader.New("0")
	assertEqual(t, must(integer(r)), ast.I64{Value: 0, Source: "0"})
	assertCursor(t, r, 1)

	r = reader.New("-1")
	assertEqual(t, must(integer(r)), ast.I64{Value: -1, Source: "-1"})
	assertCursor(t, r, 2)

	tests := []struct {
		input string
		want  ast.Number
		index int
	}{
		{"0", ast.NumberInteger{Value: ast.I64{Value: 0, Source: "0"}}, 1},
		{"10x", ast.NumberInteger{Value: ast.I64{Value: 10, Source: "10"}}, 2},
		{"-10x", ast.NumberInteger{Value: ast.I64{Value: -10, Source: "-10"}}, 3},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(number(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestFloat(t *testing.T) {
	tests := []struct {
		input string
		value float64
	}{
		{"1.0", 1.0},
		{"-1.0", -1.0},
		{"1.1", 1.1},
		{"1.100", 1.1},
		{"1.01", 1.01},
		{"1.010", 1.01},
		{"-0.3333333333333333333", -0.3333333333333333},
		{"1000000000000000000000.5", 1000000000000000000000.0},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(number(r)), ast.NumberFloat{Value: ast.Float{Value: tt.value, Source: tt.input}})
		assertCursor(t, r, len(tt.input))
	}
}

func TestNumberError(t *testing.T) {
	tests := []struct {
		input       string
		pos         reader.Pos
		recoverable bool
		expecting   string
	}{
		{"", pos(1, 1), true, "number"},
		{"-", pos(1, 2), true, "number"},
		{"x", pos(1, 1), true, "number"},
		{"1.", pos(1, 3), false, "decimal digits"},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		_, err := number(r)
		assertError(t, err, tt.pos, tt.recoverable, ParseErrorExpecting{Value: tt.expecting})
	}
}
