package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestBytesJSON(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Bytes
		index int
	}{
		{"[1,2,3] ", ast.BytesJSON{Value: ast.JSONList{Elements: []ast.JSONListElement{
			{Value: ast.JSONNumber{Value: "1"}},
			{Value: ast.JSONNumber{Value: "2"}},
			{Value: ast.JSONNumber{Value: "3"}},
		}}}, 7},
		{"{ } ", ast.BytesJSON{Value: ast.JSONObject{Space0: " "}}, 3},
		{"true", ast.BytesJSON{Value: ast.JSONBoolean{Value: true}}, 4},
		{`"" x`, ast.BytesJSON{Value: ast.JSONString{Value: tmpl('"', si(1, 2, 1, 2))}}, 2},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(bytes(r)), tt.want)
		assertCursor(t, r, tt.index)
	}
}

func TestBytesXML(t *testing.T) {
	r := reader.New("<a/>")
	assertEqual(t, must(bytes(r)), ast.Bytes(ast.BytesXML{Value: "<a/>"}))
}

func TestBytesJSONError(t *testing.T) {
	r := reader.New("{ x ")
	_, err := bytes(r)
	assertError(t, err, pos(1, 3), false, ParseErrorExpecting{Value: `"`})
}

func TestBytesMultilinesError(t *testing.T) {
	r := reader.New("```\nxxx ")
	_, err := bytes(r)
	assertError(t, err, pos(2, 5), false, ParseErrorExpecting{Value: "```"})
}

func TestBytesEOF(t *testing.T) {
	r := reader.New("")
	_, err := bytes(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "file"})
}

func TestJSONBytes(t *testing.T) {
	r := reader.New("100")
	assertEqual(t, must(jsonBytes(r)), ast.Bytes(ast.BytesJSON{Value: ast.JSONNumber{Value: "100"}}))
}

func TestBytesString(t *testing.T) {
	r := reader.New("`foo`  ")
	assertEqual(t, must(bytes(r)), ast.Bytes(ast.BytesOnelineString{Value: tmpl('`', si(1, 1, 1, 6), str("foo"))}))
	assertCursor(t, r, 5)
}
