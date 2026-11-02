package parser

import (
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestJSONParseError(t *testing.T) {
	r := reader.New("{ \"a\":\n}")
	_, err := parseAtDepth(r, 0)
	assertError(t, err, pos(1, 7), false, ParseErrorJSON{Value: JSONErrorEmptyElement{}})

	r = reader.New("[0,1,]")
	_, err = parseAtDepth(r, 0)
	assertError(t, err, pos(1, 5), false, ParseErrorJSON{Value: JSONErrorTrailingComma{}})
}

func TestJSONMaxNestingDepth(t *testing.T) {
	for _, delims := range [][2]string{{"[", "]"}, {`{"a":`, "}"}} {
		opening, closing := delims[0], delims[1]
		valid := strings.Repeat(opening, maxNestingDepth) + "null" + strings.Repeat(closing, maxNestingDepth)
		must(parseAtDepth(reader.New(valid), 0))

		tooDeep := strings.Repeat(opening, maxNestingDepth+1) + "null" + strings.Repeat(closing, maxNestingDepth+1)
		_, err := parseAtDepth(reader.New(tooDeep), 0)
		if err == nil {
			t.Fatal("expected error")
		}
		assertEqual(t, err.Kind, ParseErrorJSON{Value: JSONErrorMaxDepthExceeded{MaxDepth: maxNestingDepth}})
	}
}

func TestJSONNullValue(t *testing.T) {
	r := reader.New("null")
	assertEqual(t, must(nullValue(r)), ast.JSONValue(ast.JSONNull{}))
	assertCursor(t, r, 4)

	r = reader.New("true")
	_, err := nullValue(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "null"})
}

func TestJSONInteger(t *testing.T) {
	tests := []struct {
		input, want string
		index       int
	}{
		{"0", "0", 1},
		{"123", "123", 3},
		{"0123", "0", 1},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(jsonInteger(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestJSONFraction(t *testing.T) {
	r := reader.New(".5")
	assertEqual(t, must(jsonFraction(r)), ".5")
	assertCursor(t, r, 2)
}

func TestJSONExponent(t *testing.T) {
	r := reader.New("e2")
	assertEqual(t, jsonExponent(r), "e2")
	assertCursor(t, r, 2)
}

func TestJSONBooleanValue(t *testing.T) {
	r := reader.New("true")
	assertEqual(t, must(booleanValue(r)), ast.JSONValue(ast.JSONBoolean{Value: true}))
	assertCursor(t, r, 4)

	r = reader.New("1")
	_, err := booleanValue(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "true|false"})
}

// jsonHelloWorldValue is "Hello\u0020{{name}}!".
func jsonHelloWorldValue() ast.JSONValue {
	return ast.JSONString{Value: tmpl('"', si(1, 2, 1, 22), strSrc("Hello ", `Hello\u0020`), phElem("name", 1, 15), str("!"))}
}

func TestJSONStringValue(t *testing.T) {
	tests := []struct {
		input string
		want  ast.JSONValue
		index int
	}{
		{`""`, ast.JSONString{Value: tmpl('"', si(1, 2, 1, 2))}, 2},
		{`"Hello\u0020{{name}}!"`, jsonHelloWorldValue(), 22},
		{`"{}"`, ast.JSONString{Value: tmpl('"', si(1, 2, 1, 4), str("{}"))}, 4},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(stringValue(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestJSONStringValueError(t *testing.T) {
	r := reader.New("1")
	_, err := stringValue(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: `"`})

	r = reader.New(`"1`)
	_, err = stringValue(r)
	assertError(t, err, pos(1, 3), false, ParseErrorExpecting{Value: `"`})

	r = reader.New(`"{{x"`)
	_, err = stringValue(r)
	assertError(t, err, pos(1, 5), false, ParseErrorExpecting{Value: "}}"})
}

func TestJSONAnyChar(t *testing.T) {
	tests := []struct {
		input  string
		c      rune
		source string
		index  int
	}{
		{"a", 'a', "a", 1},
		{" ", ' ', " ", 1},
		{`\u0020 `, ' ', `\u0020`, 6},
		{`\t`, '\t', `\t`, 2},
		{"#", '#', "#", 1},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(jsonAnyChar(r)), EncodedChar{tt.c, tt.source, pos(1, 1)})
		assertCursor(t, r, tt.index)
	}
}

func TestJSONAnyCharError(t *testing.T) {
	for _, input := range []string{"", "\t"} {
		r := reader.New(input)
		_, err := jsonAnyChar(r)
		assertError(t, err, pos(1, 1), true, nil)
	}
}

func TestJSONEscapeChar(t *testing.T) {
	r := reader.New(`\n`)
	assertEqual(t, must(jsonEscapeChar(r)), '\n')
	assertCursor(t, r, 2)

	r = reader.New(`\u000a`)
	assertEqual(t, must(jsonEscapeChar(r)), '\n')
	assertCursor(t, r, 6)

	r = reader.New("x")
	_, err := jsonEscapeChar(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: `\`})
	assertCursor(t, r, 0)
}

func TestJSONUnicode(t *testing.T) {
	tests := []struct {
		input string
		want  rune
		index int
	}{
		{"000a", '\n', 4},
		{"c350", '썐', 4},
		{`d83c\udf78`, '🍸', 10},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(jsonUnicode(r)), tt.want)
		assertCursor(t, r, tt.index)
	}

	r := reader.New("d800")
	_, err := jsonUnicode(r)
	assertError(t, err, pos(1, 5), false, ParseErrorExpecting{Value: `\u`})

	r = reader.New(`d800\ud800`)
	_, err = jsonUnicode(r)
	assertError(t, err, pos(1, 7), false, ParseErrorUnicode{})
}

func TestJSONHexValue(t *testing.T) {
	r := reader.New("0020x")
	assertEqual(t, must(jsonHexValue(r)), 32)

	r = reader.New("d800")
	assertEqual(t, must(jsonHexValue(r)), 55296)

	r = reader.New("x")
	_, err := jsonHexValue(r)
	assertError(t, err, pos(1, 1), false, ParseErrorHexDigit{})
}

func TestJSONNumberValue(t *testing.T) {
	tests := []struct {
		input, want string
		index       int
	}{
		{"100", "100", 3},
		{"1.333", "1.333", 5},
		{"-1", "-1", 2},
		{"00", "0", 1},
		{"1e0", "1e0", 3},
		{"1e005", "1e005", 5},
		{"1e-005", "1e-005", 6},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(numberValue(r)), ast.JSONValue(ast.JSONNumber{Value: tt.want}))
		assertCursor(t, r, tt.index)
	}
}

func TestJSONNumberValueError(t *testing.T) {
	r := reader.New("true")
	_, err := numberValue(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "number"})

	r = reader.New("1.x")
	_, err = numberValue(r)
	assertError(t, err, pos(1, 3), false, ParseErrorExpecting{Value: "digits"})
}

func TestJSONExpressionValue(t *testing.T) {
	r := reader.New("{{n}}")
	assertEqual(t, must(expressionValue(r)), ast.JSONValue(ast.JSONPlaceholder{Value: ph("n", 1, 3)}))
	assertCursor(t, r, 5)
}

func TestJSONListValue(t *testing.T) {
	tests := []struct {
		input string
		want  ast.JSONValue
		index int
	}{
		{"[]", ast.JSONList{}, 2},
		{"[ ]", ast.JSONList{Space0: " "}, 3},
		{"[true, false]", ast.JSONList{Elements: []ast.JSONListElement{
			{Value: ast.JSONBoolean{Value: true}},
			{Space0: " ", Value: ast.JSONBoolean{Value: false}},
		}}, 13},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(listValue(r, 0)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestJSONListError(t *testing.T) {
	r := reader.New("true")
	_, err := listValue(r, 0)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "["})

	r = reader.New("[1, 2,]")
	_, err = listValue(r, 0)
	assertError(t, err, pos(1, 6), false, ParseErrorJSON{Value: JSONErrorTrailingComma{}})
}

func TestJSONListElement(t *testing.T) {
	r := reader.New("true")
	assertEqual(t, must(listElement(r, 1)), ast.JSONListElement{Value: ast.JSONBoolean{Value: true}})
	assertCursor(t, r, 4)
}

func TestJSONObjectValue(t *testing.T) {
	tests := []struct {
		input string
		want  ast.JSONValue
		index int
	}{
		{"{}", ast.JSONObject{}, 2},
		{"{ }", ast.JSONObject{Space0: " "}, 3},
		{"{\n  \"a\": true\n}", ast.JSONObject{Space0: "\n  ", Elements: []ast.JSONObjectElement{{
			Name:   tmpl('"', si(2, 4, 2, 5), str("a")),
			Space2: " ",
			Value:  ast.JSONBoolean{Value: true},
			Space3: "\n",
		}}}, 15},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(objectValue(r, 0)), tt.want)
		assertCursor(t, r, tt.index)
	}

	r := reader.New("true")
	_, err := objectValue(r, 0)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "{"})
}

func TestJSONObjectError(t *testing.T) {
	r := reader.New("{ \"a\":\n}")
	_, err := objectValue(r, 0)
	assertError(t, err, pos(1, 7), false, ParseErrorJSON{Value: JSONErrorEmptyElement{}})
}

func TestJSONObjectElement(t *testing.T) {
	r := reader.New(`"a": true`)
	assertEqual(t, must(objectElement(r, 1)), ast.JSONObjectElement{
		Name:   tmpl('"', si(1, 2, 1, 3), str("a")),
		Space2: " ",
		Value:  ast.JSONBoolean{Value: true},
	})
	assertCursor(t, r, 9)
}

func TestJSONObjectElementError(t *testing.T) {
	r := reader.New(":")
	_, err := objectElement(r, 1)
	assertError(t, err, pos(1, 1), false, ParseErrorExpecting{Value: `"`})

	r = reader.New("\"name\":\n")
	_, err = objectElement(r, 1)
	assertError(t, err, pos(1, 8), false, ParseErrorJSON{Value: JSONErrorEmptyElement{}})
}

func TestJSONWhitespace(t *testing.T) {
	tests := []struct {
		input, want string
		index       int
	}{
		{"", "", 0},
		{" x", " ", 1},
		{"\n  x", "\n  ", 3},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, jsonWhitespace(r), tt.want)
		assertCursor(t, r, tt.index)
	}
}
