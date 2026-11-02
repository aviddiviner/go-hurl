package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestDurationUnit(t *testing.T) {
	for _, input := range []string{"", "\n"} {
		r := reader.New(input)
		if u := must(durationUnit(r)); u != nil {
			t.Fatalf("durationUnit(%q) = %v, want nil", input, *u)
		}
	}
	tests := []struct {
		input string
		want  ast.DurationUnit
	}{
		{"s\n", ast.DurationUnitSecond},
		{"ms\n", ast.DurationUnitMilliSecond},
		{"m\n", ast.DurationUnitMinute},
		{"h\n", ast.DurationUnitHour},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(durationUnit(r)), &tt.want)
	}
}

func TestDurationUnitError(t *testing.T) {
	r := reader.New("mms\n")
	_, err := durationUnit(r)
	assertError(t, err, pos(1, 1), false, ParseErrorInvalidDurationUnit{Value: "mms"})
}

func durationOf(value uint64, source string, unit *ast.DurationUnit) ast.Duration {
	return ast.Duration{Value: ast.U64{Value: value, Source: source}, Unit: unit}
}

func unitPtr(u ast.DurationUnit) *ast.DurationUnit { return &u }

func TestDuration(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Duration
	}{
		{"10", durationOf(10, "10", nil)},
		{"10s", durationOf(10, "10", unitPtr(ast.DurationUnitSecond))},
		{"10000ms", durationOf(10000, "10000", unitPtr(ast.DurationUnitMilliSecond))},
		{"1m", durationOf(1, "1", unitPtr(ast.DurationUnitMinute))},
		{"1h", durationOf(1, "1", unitPtr(ast.DurationUnitHour))},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(duration(r)), tt.want)
	}
}

func TestDurationError(t *testing.T) {
	r := reader.New("10mms\n")
	_, err := duration(r)
	assertError(t, err, pos(1, 3), false, ParseErrorInvalidDurationUnit{Value: "mms"})
}
