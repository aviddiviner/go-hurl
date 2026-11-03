package runner

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/parser"
	"github.com/aviddiviner/go-hurl/reader"
)

// evalJSONValue evaluates a JSON value to a string given a set of variables.
// If keepWhitespace is true, whitespace is preserved from the JSON value,
// otherwise it is trimmed.
func evalJSONValue(jsonValue ast.JSONValue, variables *VariableSet, keepWhitespace bool) (string, *RunnerError) {
	switch j := jsonValue.(type) {
	case ast.JSONNull:
		return "null", nil
	case ast.JSONNumber:
		return j.Value, nil
	case ast.JSONString:
		s, err := evalJSONTemplate(j.Value, variables)
		if err != nil {
			return "", err
		}
		return `"` + s + `"`, nil
	case ast.JSONBoolean:
		return strconv.FormatBool(j.Value), nil
	case ast.JSONList:
		elems := make([]string, len(j.Elements))
		for i, e := range j.Elements {
			s, err := evalJSONValue(e.Value, variables, keepWhitespace)
			if err != nil {
				return "", err
			}
			if keepWhitespace {
				s = e.Space0 + s + e.Space1
			}
			elems[i] = s
		}
		if keepWhitespace {
			return "[" + j.Space0 + strings.Join(elems, ",") + "]", nil
		}
		return "[" + strings.Join(elems, ",") + "]", nil
	case ast.JSONObject:
		elems := make([]string, len(j.Elements))
		for i, e := range j.Elements {
			s, err := evalJSONObjectElement(e, variables, keepWhitespace)
			if err != nil {
				return "", err
			}
			elems[i] = s
		}
		if keepWhitespace {
			return "{" + j.Space0 + strings.Join(elems, ",") + "}", nil
		}
		return "{" + strings.Join(elems, ",") + "}", nil
	case ast.JSONPlaceholder:
		expr := j.Value.Expr
		s, err := renderExpr(expr, variables)
		if err != nil {
			return "", err
		}
		// The rendered value can only be null, a boolean or a number.
		for _, parse := range []func(*reader.Reader) (ast.JSONValue, *parser.ParseError){
			parser.ParseJSONNumber, parser.ParseJSONBoolean, parser.ParseJSONNull,
		} {
			if _, perr := parse(reader.New(s)); perr == nil {
				return s, nil
			}
		}
		return "", NewRunnerError(expr.SourceInfo, RunnerErrorInvalidJSON{Value: s}, false)
	}
	panic("unknown JSON value")
}

func evalJSONObjectElement(e ast.JSONObjectElement, variables *VariableSet, keepWhitespace bool) (string, *RunnerError) {
	name, err := evalTemplate(e.Name, variables)
	if err != nil {
		return "", err
	}
	value, err := evalJSONValue(e.Value, variables, keepWhitespace)
	if err != nil {
		return "", err
	}
	if keepWhitespace {
		return e.Space0 + `"` + name + `"` + e.Space1 + ":" + e.Space2 + value + e.Space3, nil
	}
	// As in Hurl, the unevaluated name is used here.
	return `"` + e.Name.String() + `":` + value, nil
}

// evalJSONTemplate evaluates a JSON string template: literal parts are kept
// as written in the source, placeholders are JSON-escaped. For instance, the
// template "Hello {{quote}}" with quote=" gives "Hello \"".
func evalJSONTemplate(template ast.Template, variables *VariableSet) (string, *RunnerError) {
	var b strings.Builder
	for _, e := range template.Elements {
		switch e := e.(type) {
		case ast.TemplateString:
			b.WriteString(e.Source)
		case ast.TemplatePlaceholder:
			s, err := renderExpr(e.Value.Expr, variables)
			if err != nil {
				return "", err
			}
			b.WriteString(encodeJSONString(s))
		}
	}
	return b.String(), nil
}

func encodeJSONString(s string) string {
	var b strings.Builder
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(c)
		}
	}
	return b.String()
}

// jsonQuote serializes s as a JSON string, like serde_json (no HTML
// escaping, lowercase \u00xx for other control characters).
func jsonQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, c := range s {
		switch c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 {
				fmt.Fprintf(&b, `\u%04x`, c)
			} else {
				b.WriteRune(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// parseJSON parses a JSON document like serde_json::from_str: trailing
// whitespace is allowed, anything else after the value is an error.
func parseJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing characters")
	}
	return v, nil
}

// valueFromJSON is Value::from_json (defined in Hurl's query.rs): numbers
// with a fraction or an exponent are floats (if finite), then integers that
// fit an int64, and anything else is a big integer. Object members are in
// sorted key order, like serde_json without preserve_order.
func valueFromJSON(v any) Value {
	switch v := v.(type) {
	case nil:
		return ValueNull{}
	case bool:
		return ValueBool{v}
	case json.Number:
		return ValueNumber{numberFromJSON(string(v))}
	case string:
		return ValueString{v}
	case []any:
		values := make([]Value, len(v))
		for i, e := range v {
			values[i] = valueFromJSON(e)
		}
		return ValueList{values}
	case map[string]any:
		elements := make([]KeyValue, 0, len(v))
		for _, k := range slices.Sorted(maps.Keys(v)) {
			elements = append(elements, KeyValue{k, valueFromJSON(v[k])})
		}
		return ValueObject{elements}
	}
	panic(fmt.Sprintf("unexpected JSON value %T", v))
}

// numberFromJSON follows serde_json's arbitrary_precision Number: is_f64,
// then is_i64, else the literal itself.
func numberFromJSON(n string) Number {
	if strings.ContainsAny(n, ".eE") {
		f, err := strconv.ParseFloat(n, 64)
		if err == nil || (errors.Is(err, strconv.ErrRange) && !math.IsInf(f, 0)) {
			return NumberFloat{f}
		}
	}
	if i, err := strconv.ParseInt(n, 10, 64); err == nil {
		return NumberInteger{i}
	}
	return NumberBigInteger{n}
}
