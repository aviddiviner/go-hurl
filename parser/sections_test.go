package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestSectionName(t *testing.T) {
	r := reader.New("[SectionA]")
	assertEqual(t, must(sectionName(r)), "SectionA")

	r = reader.New("[]")
	_, err := sectionName(r)
	if err == nil || !err.Recoverable {
		t.Fatalf("expected recoverable error, got %v", err)
	}
}

func headerQueryOf(name string, line, column int) ast.Query {
	end := column + 7 + len(name) + 2
	return ast.Query{
		SourceInfo: si(line, column, line, end),
		Value: ast.QueryHeader{
			Space0: ws(" ", line, column+6, line, column+7),
			Name:   tmpl('"', si(line, column+7, line, end), str(name)),
		},
	}
}

func TestAssertsSection(t *testing.T) {
	r := reader.New("[Asserts]\nheader \"Location\" == \"https://google.fr\"\n")
	assertEqual(t, must(responseSection(r)), ast.Section{
		Space0:          ews(1, 1),
		LineTerminator0: lt(ews(1, 10), ws("\n", 1, 10, 2, 1)),
		Value: ast.SectionAsserts{Value: []ast.Assert{{
			Space0: ews(2, 1),
			Query:  headerQueryOf("Location", 2, 1),
			Space1: ws(" ", 2, 18, 2, 19),
			Predicate: ast.Predicate{
				Space0: ews(2, 19),
				PredicateFunc: ast.PredicateFunc{
					SourceInfo: si(2, 19, 2, 41),
					Value: ast.PredicateFuncEqual{
						Space0: ws(" ", 2, 21, 2, 22),
						Value:  ast.PredicateString{Value: tmpl('"', si(2, 22, 2, 41), str("https://google.fr"))},
					},
				},
			},
			LineTerminator0: lt(ews(2, 41), ws("\n", 2, 41, 3, 1)),
		}}},
		SourceInfo: si(1, 1, 1, 10),
	})
}

func TestAssertsSectionError(t *testing.T) {
	r := reader.New("x[Assertsx]\nheader Location == \"https://google.fr\"\n")
	_, err := responseSection(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "["})

	r = reader.New("[Assertsx]\nheader Location == \"https://google.fr\"\n")
	_, err = responseSection(r)
	assertError(t, err, pos(1, 2), false, ParseErrorResponseSectionName{Name: "Assertsx"})
}

func TestCookie(t *testing.T) {
	r := reader.New("Foo: Bar")
	c := must(cookie(r))
	assertEqual(t, c.Name.String(), "Foo")
	assertEqual(t, c.Value, strTmpl("Bar", si(1, 6, 1, 9)))
}

func TestCookieError(t *testing.T) {
	r := reader.New("Foo: {{Bar")
	_, err := cookie(r)
	assertError(t, err, pos(1, 11), false, ParseErrorExpecting{Value: "}}"})
}

func TestFileValue(t *testing.T) {
	r := reader.New("file,hello.txt;")
	assertEqual(t, must(fileValue(r)), ast.FilenameValue{
		Space0:   ews(1, 6),
		Filename: strTmpl("hello.txt", si(1, 6, 1, 15)),
		Space1:   ews(1, 15),
		Space2:   ews(1, 16),
	})

	r = reader.New("file,hello.txt; text/html")
	contentType := strTmpl("text/html", si(1, 17, 1, 26))
	assertEqual(t, must(fileValue(r)), ast.FilenameValue{
		Space0:      ews(1, 6),
		Filename:    strTmpl("hello.txt", si(1, 6, 1, 15)),
		Space1:      ews(1, 15),
		Space2:      ws(" ", 1, 16, 1, 17),
		ContentType: &contentType,
	})
}

func TestFileContentType(t *testing.T) {
	tests := []struct {
		input, contentType string
		index              int
	}{
		{"file,hello.txt; text/html", "text/html", 25},
		{"file,------; text/plain; charset=us-ascii", "text/plain; charset=us-ascii", 41},
		{"file,******; text/html # comment", "text/html", 22},
		{"file,{{some_file}}; application/vnd.openxmlformats-officedocument.wordprocessingml.document # comment", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", 91},
		{"file,{{some_file}}; {{some_content_type}} # comment", "{{some_content_type}}", 41},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		v := must(fileValue(r))
		if v.ContentType == nil {
			t.Fatalf("%q: no content type", tt.input)
		}
		assertEqual(t, v.ContentType.ToSource(), tt.contentType)
		assertCursor(t, r, tt.index)
	}
}

func TestCapture(t *testing.T) {
	r := reader.New(`url: header "Location"`)
	assertEqual(t, must(capture(r)), ast.Capture{
		Space0:          ews(1, 1),
		Name:            strTmpl("url", si(1, 1, 1, 4)),
		Space1:          ews(1, 4),
		Space2:          ws(" ", 1, 5, 1, 6),
		Query:           headerQueryOf("Location", 1, 6),
		Space3:          ews(1, 23),
		LineTerminator0: lt(ews(1, 23), ews(1, 23)),
	})

	r = reader.New(`url: header "Token"    redact`)
	assertEqual(t, must(capture(r)), ast.Capture{
		Space0:          ews(1, 1),
		Name:            strTmpl("url", si(1, 1, 1, 4)),
		Space1:          ews(1, 4),
		Space2:          ws(" ", 1, 5, 1, 6),
		Query:           headerQueryOf("Token", 1, 6),
		Space3:          ws("    ", 1, 20, 1, 24),
		Redacted:        true,
		LineTerminator0: lt(ews(1, 30), ews(1, 30)),
	})
}

func TestCaptureWithFilter(t *testing.T) {
	r := reader.New(`token: header "Location" regex "token=(.*)"`)
	assertEqual(t, must(capture(r)).Query, headerQueryOf("Location", 1, 8))
	assertCursor(t, r, 43)
}

func TestCaptureWithFilterError(t *testing.T) {
	r := reader.New(`token: header "Location" regex `)
	_, err := capture(r)
	assertError(t, err, pos(1, 32), false, ParseErrorExpecting{Value: `" or /`})

	r = reader.New(`token: header "Location" xxx`)
	_, err = capture(r)
	assertError(t, err, pos(1, 26), false, ParseErrorExpecting{Value: "line_terminator"})
}

func TestCaptureWithComment(t *testing.T) {
	r := reader.New(`name: jsonpath "$.name"          # name`)
	c := must(capture(r))
	assertEqual(t, len(c.Filters), 0)
	assertEqual(t, c.Space3.Value, "")
	assertEqual(t, c.LineTerminator0.Space0.Value, "          ")
}

func TestAssert(t *testing.T) {
	r := reader.New(`header "Location" == "https://google.fr"`)
	assertEqual(t, must(assert(r)).Query, headerQueryOf("Location", 1, 1))
}

func TestAssertJSONPath(t *testing.T) {
	r := reader.New(`jsonpath "$.errors" == 5`)
	assertEqual(t, must(assert(r)).Predicate, ast.Predicate{
		Space0: ews(1, 21),
		PredicateFunc: ast.PredicateFunc{
			SourceInfo: si(1, 21, 1, 25),
			Value: ast.PredicateFuncEqual{
				Space0: ws(" ", 1, 23, 1, 24),
				Value:  ast.PredicateNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 5, Source: "5"}}},
			},
		},
	})
}

func TestBasicAuthSection(t *testing.T) {
	r := reader.New("[BasicAuth]\nuser:password\n\nHTTP 200\n")
	assertEqual(t, must(requestSection(r)), ast.Section{
		Space0:          ews(1, 1),
		LineTerminator0: lt(ews(1, 12), ws("\n", 1, 12, 2, 1)),
		Value: ast.SectionBasicAuth{Value: &ast.KeyValue{
			Space0:          ews(2, 1),
			Key:             strTmpl("user", si(2, 1, 2, 5)),
			Space1:          ews(2, 5),
			Space2:          ews(2, 6),
			Value:           strTmpl("password", si(2, 6, 2, 14)),
			LineTerminator0: lt(ews(2, 14), ws("\n", 2, 14, 3, 1)),
		}},
		SourceInfo: si(1, 1, 1, 12),
	})
	assertEqual(t, r.Cursor().Pos, pos(3, 1))

	r = reader.New("[BasicAuth]\nHTTP 200\n")
	assertEqual(t, must(requestSection(r)), ast.Section{
		Space0:          ews(1, 1),
		LineTerminator0: lt(ews(1, 12), ws("\n", 1, 12, 2, 1)),
		Value:           ast.SectionBasicAuth{},
		SourceInfo:      si(1, 1, 1, 12),
	})
	assertEqual(t, r.Cursor().Pos, pos(2, 1))
}
