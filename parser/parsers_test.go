package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestHurlFile(t *testing.T) {
	r := reader.New("GET http://google.fr")
	assertEqual(t, len(must(hurlFile(r)).Entries), 1)
}

func TestEntry(t *testing.T) {
	r := reader.New("GET http://google.fr")
	assertEqual(t, must(entry(r)).Request.Method, ast.Method("GET"))
	assertCursor(t, r, 20)
}

func TestSeveralEntry(t *testing.T) {
	tests := []struct {
		input          string
		index1, index2 int
	}{
		{"GET http://google.fr\nGET http://google.fr", 21, 41},
		{"GET http://google.fr # comment1\nGET http://google.fr # comment2", 32, 63},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(entry(r)).Request.Method, ast.Method("GET"))
		assertCursor(t, r, tt.index1)
		assertEqual(t, r.Cursor().Pos.Line, 2)

		assertEqual(t, must(entry(r)).Request.Method, ast.Method("GET"))
		assertCursor(t, r, tt.index2)
		assertEqual(t, r.Cursor().Pos.Line, 2)
	}
}

func TestEntryWithResponse(t *testing.T) {
	r := reader.New("GET http://google.fr\nHTTP/1.1 200")
	e := must(entry(r))
	assertEqual(t, e.Request.Method, ast.Method("GET"))
	assertEqual(t, e.Response.Status.Value, ast.StatusValue(ast.StatusSpecific{Value: 200}))
}

func TestRequest(t *testing.T) {
	r := reader.New("GET http://google.fr")
	assertEqual(t, must(request(r)), ast.Request{
		Space0:          ews(1, 1),
		Method:          "GET",
		Space1:          ws(" ", 1, 4, 1, 5),
		URL:             strTmpl("http://google.fr", si(1, 5, 1, 21)),
		LineTerminator0: lt(ews(1, 21), ews(1, 21)),
		SourceInfo:      si(1, 1, 1, 21),
	})
	assertCursor(t, r, 20)

	r = reader.New("GET  http://google.fr # comment")
	assertEqual(t, must(request(r)), ast.Request{
		Space0: ews(1, 1),
		Method: "GET",
		Space1: ws("  ", 1, 4, 1, 6),
		URL:    strTmpl("http://google.fr", si(1, 6, 1, 22)),
		LineTerminator0: ast.LineTerminator{
			Space0:  ws(" ", 1, 22, 1, 23),
			Comment: &ast.Comment{Value: " comment", SourceInfo: si(1, 24, 1, 32)},
			Newline: ews(1, 32),
		},
		SourceInfo: si(1, 1, 1, 32),
	})
	assertCursor(t, r, 31)

	r = reader.New("GET http://google.fr\nGET http://google.fr")
	assertEqual(t, must(request(r)).Method, ast.Method("GET"))
	assertCursor(t, r, 21)
	assertEqual(t, must(request(r)).Method, ast.Method("GET"))
}

func TestRequestMultilines(t *testing.T) {
	r := reader.New("GET http://google.fr\n```\nHello World!\n```")
	assertEqual(t, must(request(r)).Body, &ast.Body{
		Space0: ews(2, 1),
		Value: ast.BytesMultilineString{Value: mlText(
			ews(2, 4), ws("\n", 2, 4, 3, 1), strTmpl("Hello World!\n", si(3, 1, 4, 1)),
		)},
		LineTerminator0: lt(ews(4, 4), ews(4, 4)),
	})
}

func jsonList123() ast.JSONValue {
	return ast.JSONList{Elements: []ast.JSONListElement{
		{Value: ast.JSONNumber{Value: "1"}},
		{Value: ast.JSONNumber{Value: "2"}},
		{Value: ast.JSONNumber{Value: "3"}},
	}}
}

func TestRequestPostJSON(t *testing.T) {
	tests := []struct {
		input string
		want  ast.Bytes
	}{
		{"POST http://localhost:8000/post-json-array\n[1,2,3]", ast.BytesJSON{Value: jsonList123()}},
		{"POST http://localhost:8000/post-json-string\n\"Hello\"", ast.BytesJSON{Value: ast.JSONString{Value: tmpl('"', si(2, 2, 2, 7), str("Hello"))}}},
		{"POST http://localhost:8000/post-json-number\n100", ast.BytesJSON{Value: ast.JSONNumber{Value: "100"}}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		req := must(request(r))
		assertEqual(t, req.Method, ast.Method("POST"))
		assertEqual(t, req.Body.Value, tt.want)
	}
}

func TestRequestError(t *testing.T) {
	r := reader.New("xxx")
	_, err := request(r)
	assertError(t, err, pos(1, 1), false, nil)
}

func TestResponse(t *testing.T) {
	r := reader.New("HTTP/1.1 200")
	resp := must(response(r))
	assertEqual(t, resp.Version.Value, ast.Version11)
	assertEqual(t, resp.Status.Value, ast.StatusValue(ast.StatusSpecific{Value: 200}))
}

func TestMethod(t *testing.T) {
	r := reader.New("xxx ")
	_, err := method(r)
	assertError(t, err, pos(1, 1), false, nil)
	assertCursor(t, r, 3)

	r = reader.New("")
	_, err = method(r)
	assertError(t, err, pos(1, 1), true, nil)
	assertCursor(t, r, 0)

	r = reader.New("GET ")
	assertEqual(t, must(method(r)), ast.Method("GET"))
	assertCursor(t, r, 3)

	r = reader.New("CUSTOM")
	assertEqual(t, must(method(r)), ast.Method("CUSTOM"))
	assertCursor(t, r, 6)
}

func TestVersion(t *testing.T) {
	r := reader.New("HTTP 200")
	assertEqual(t, must(version(r)).Value, ast.VersionAny)
	assertCursor(t, r, 4)

	r = reader.New("HTTP\t200")
	assertEqual(t, must(version(r)).Value, ast.VersionAny)
	assertCursor(t, r, 4)

	r = reader.New("HTTP/1.1 200")
	assertEqual(t, must(version(r)).Value, ast.Version11)

	r = reader.New("HTTP/1. 200")
	_, err := version(r)
	assertError(t, err, pos(1, 1), false, ParseErrorVersion{})
}

func TestStatus(t *testing.T) {
	r := reader.New("*")
	assertEqual(t, must(status(r)).Value, ast.StatusValue(ast.StatusAny{}))

	r = reader.New("200")
	assertEqual(t, must(status(r)).Value, ast.StatusValue(ast.StatusSpecific{Value: 200}))

	r = reader.New("xxx")
	if _, err := status(r); err == nil {
		t.Fatal("expected error")
	}
}

func TestBodyJSON(t *testing.T) {
	tests := []struct {
		input           string
		lineTerminators int
		want            ast.JSONValue
		index           int
	}{
		{"[1,2,3] ", 0, jsonList123(), 8},
		{"{}", 0, ast.JSONObject{}, 2},
		{"# comment\n {} # comment\nxxx", 1, ast.JSONObject{}, 24},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		b := must(body(r))
		assertEqual(t, len(b.LineTerminators), tt.lineTerminators)
		assertEqual(t, b.Value, ast.Bytes(ast.BytesJSON{Value: tt.want}))
		assertCursor(t, r, tt.index)
	}

	r := reader.New("{x")
	_, err := body(r)
	assertError(t, err, pos(1, 2), false, nil)
}
