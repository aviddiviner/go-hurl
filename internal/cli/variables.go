package cli

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/runner"
)

// typeKind is how variables are typed.
type typeKind int

const (
	// typeInferred infers the variable type from its value.
	typeInferred typeKind = iota
	// typeString forces variables to be strings.
	typeString
)

// parseVariable parses "name=value" as a name and a value.
func parseVariable(s string, kind typeKind) (string, runner.Value, error) {
	name, value, ok := strings.Cut(s, "=")
	if !ok {
		return "", nil, optionsErrorf("Missing value for variable %s!", s)
	}
	if ast.IsVariableReserved(name) {
		return "", nil, optionsErrorf("Variable %s conflicts with the %s function, use a different name.", name, name)
	}
	v, err := parseVariableValue(value, kind)
	if err != nil {
		return "", nil, err
	}
	return name, v, nil
}

// parseVariableValue parses a variable value. With typeInferred, the type
// is inferred from s (true is a boolean, 1 an integer...).
func parseVariableValue(s string, kind typeKind) (runner.Value, error) {
	if kind == typeString {
		return runner.NewString(s), nil
	}
	switch s {
	case "true":
		return runner.NewBool(true), nil
	case "false":
		return runner.NewBool(false), nil
	case "null":
		return runner.ValueNull{}, nil
	}
	if n, err := rustParseInt(s, math.MinInt64, math.MaxInt64); err == nil {
		return runner.NewInteger(n), nil
	}
	if allNumeric(s) {
		return runner.ValueNumber{Value: runner.NumberBigInteger{Value: s}}, nil
	}
	if f, ok := rustParseFloat(s); ok {
		return runner.NewFloat(f), nil
	}
	if rest, ok := strings.CutPrefix(s, `"`); ok {
		if rest, ok := strings.CutSuffix(rest, `"`); ok {
			return runner.NewString(rest), nil
		}
		return nil, optionsErrorf("Value should end with a double quote")
	}
	return runner.NewString(s), nil
}

// allNumeric is Rust's s.chars().all(char::is_numeric) (true for "").
func allNumeric(s string) bool {
	for _, c := range s {
		if !unicode.IsNumber(c) {
			return false
		}
	}
	return true
}

// rustParseFloat parses s like Rust's str::parse::<f64>: decimal notation,
// inf, infinity and nan (case insensitive), but no hexadecimal floats or
// underscores, which Go accepts.
func rustParseFloat(s string) (float64, bool) {
	if strings.ContainsAny(s, "xX_pP") {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false
	}
	return f, true
}

// addSecret adds a secret, which can't be reassigned.
func addSecret(secrets map[string]string, name string, value runner.Value) error {
	if _, ok := secrets[name]; ok {
		return optionsErrorf("secret '%s' can't be reassigned", name)
	}
	s, ok := value.(runner.ValueString)
	if !ok {
		panic("Secrets must be string")
	}
	secrets[name] = s.Value
	return nil
}
