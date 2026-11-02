package sourceerr

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/text"
	"github.com/aviddiviner/go-hurl/reader"
)

func si(l1, c1, l2, c2 int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: l1, Column: c1}, End: reader.Pos{Line: l2, Column: c2}}
}

func TestAddCarets(t *testing.T) {
	// `Hello World`
	// ^^^^^^^^^^^^^ actual value is <Hello World!>
	got := AddCarets("actual value is <Hello World!>", si(1, 1, 1, 14), []string{"`Hello World`"})
	if want := " ^^^^^^^^^^^^^ actual value is <Hello World!>"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// end of file missing ```
	content := []string{
		"POST https://fake.com",
		"```",
		`{ "test": true}`,
		"",
		"HTTP 200",
	}
	got = AddCarets("expecting '```'", si(6, 1, 6, 1), content)
	if want := " ^ expecting '```'"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAddCaretsMultiline(t *testing.T) {
	got := AddCarets("line1\n\nline3\n", si(1, 3, 1, 5), []string{"abcdef"})
	if want := "   ^^ line1\n\n      line3"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestAddSourceLine(t *testing.T) {
	content := []string{"Invalid method", "get http://localhost"}
	var s text.StyledString
	AddSourceLine(&s, content, 2)
	if got := s.Render(text.FormatPlain); got != " get http://localhost\n" {
		t.Errorf("got %q", got)
	}

	s = text.StyledString{}
	AddSourceLine(&s, content, 6)
	if got := s.Render(text.FormatPlain); got != " \n" {
		t.Errorf("got %q", got)
	}
}

func TestGetCarets(t *testing.T) {
	tests := []struct {
		line          string
		column, width int
		want          string
	}{
		// `Hello World`
		// ^^^^^^^^^^^^^ actual value is <Hello World!>
		{"`Hello World`", 1, 13, " ^^^^^^^^^^^^^ "},
		// Content-Length: 200
		//                 ^^^ actual value is <12>
		{"Content-Length: 200", 17, 3, "                 ^^^ "},
		// With a tab instead of a space
		// Content-Length:    200
		//                    ^^^ actual value is <12>
		{"Content-Length:\t200", 17, 3, "                    ^^^ "},
	}
	for _, tt := range tests {
		if got := getCarets(tt.line, tt.column, tt.width); got != tt.want {
			t.Errorf("getCarets(%q, %d, %d) = %q, want %q", tt.line, tt.column, tt.width, got, tt.want)
		}
	}
}

// testError is a DisplaySourceError with a configurable fixme and an
// optional Message override.
type testError struct {
	sourceInfo  ast.SourceInfo
	description string
	fixme       func(content []string) text.StyledString
}

func (e testError) SourceInfo() ast.SourceInfo               { return e.sourceInfo }
func (e testError) Description() string                      { return e.description }
func (e testError) Fixme(content []string) text.StyledString { return e.fixme(content) }

type diffError struct{ testError }

func (e diffError) Message(content []string) text.StyledString {
	return AddLineInfoPrefix(e.Fixme(content), nil, 4)
}

func TestDiffError(t *testing.T) {
	content := "GET http://localhost:8000/failed/multiline/json\nHTTP 200\n```\n{\n  \"name\": \"John\",\n  \"age\": 27\n}\n```\n"
	e := diffError{testError{
		sourceInfo:  si(4, 1, 4, 0),
		description: "Assert body value",
		fixme: func([]string) text.StyledString {
			var diff text.StyledString
			diff.Push(" {\n   \"name\": \"John\",\n")
			diff.PushWith("-  \"age\": 27", text.NewStyle().Red())
			diff.Push("\n")
			diff.PushWith("+  \"age\": 28", text.NewStyle().Green())
			diff.Push("\n }\n")
			return diff
		},
	}}

	lines := Lines(content)
	if got, want := Message(e, lines).Render(text.FormatPlain), `
 4 | {
   |   "name": "John",
   |-  "age": 27
   |+  "age": 28
   | }
   |`; got != want {
		t.Errorf("plain message:\n%s\nwant:\n%s", got, want)
	}
	if got, want := Message(e, lines).Render(text.FormatANSI),
		"\n\x1b[1;34m 4 |\x1b[0m {\n\x1b[1;34m   |\x1b[0m   \"name\": \"John\",\n\x1b[1;34m   |\x1b[0m\x1b[31m-  \"age\": 27\x1b[0m\n\x1b[1;34m   |\x1b[0m\x1b[32m+  \"age\": 28\x1b[0m\n\x1b[1;34m   |\x1b[0m }\n\x1b[1;34m   |\x1b[0m"; got != want {
		t.Errorf("ansi message:\n%q\nwant:\n%q", got, want)
	}
	if got, want := Render(e, "test.hurl", content, nil, OutputFormatTerminal{Color: false}), `Assert body value
  --> test.hurl:4:1
   |
 4 | {
   |   "name": "John",
   |-  "age": 27
   |+  "age": 28
   | }
   |`; got != want {
		t.Errorf("render:\n%s\nwant:\n%s", got, want)
	}
}

// caretError mimics the parser and runner errors whose fixme is a red bold
// message under carets.
func caretError(sourceInfo ast.SourceInfo, description, message string) testError {
	return testError{
		sourceInfo:  sourceInfo,
		description: description,
		fixme: func(content []string) text.StyledString {
			var s text.StyledString
			for i, line := range strings.Split(AddCarets(message, sourceInfo, content), "\n") {
				if i > 0 {
					s.Push("\n")
				}
				s.PushWith(line, text.NewStyle().Red().Bold())
			}
			return s
		},
	}
}

// From hurl_core parser/error.rs.
func TestParsingError(t *testing.T) {
	e := caretError(si(1, 5, 1, 5), "Parsing URL", "expecting http://, https:// or {{")
	got := Render(e, "test.hurl", "GET abc", nil, OutputFormatTerminal{Color: false})
	want := `Parsing URL
  --> test.hurl:1:5
   |
 1 | GET abc
   |     ^ expecting http://, https:// or {{
   |`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// From hurl runner/error.rs.
func TestRunnerErrors(t *testing.T) {
	entry := func(end int) *ast.SourceInfo { s := si(1, 1, 1, end); return &s }
	tests := []struct {
		name    string
		err     testError
		content string
		entry   *ast.SourceInfo
		message string
		render  string
	}{
		{
			name:    "http error",
			err:     caretError(si(1, 5, 1, 19), "HTTP connection", "(6) Could not resolve host: unknown"),
			content: "GET http://unknown",
			entry:   entry(19),
			render: `HTTP connection
  --> test.hurl:1:5
   |
 1 | GET http://unknown
   |     ^^^^^^^^^^^^^^ (6) Could not resolve host: unknown
   |`,
		},
		{
			name:    "assert status",
			err:     caretError(si(2, 10, 2, 13), "Assert status code", "actual value is <404>"),
			content: "GET http://unknown\nHTTP/1.0 200\n",
			entry:   entry(18),
			message: "\n 2 | HTTP/1.0 200\n   |          ^^^ actual value is <404>\n   |",
			render: `Assert status code
  --> test.hurl:2:10
   |
   | GET http://unknown
 2 | HTTP/1.0 200
   |          ^^^ actual value is <404>
   |`,
		},
		{
			name:    "invalid xpath",
			err:     caretError(si(4, 7, 4, 29), "Invalid XPath expression", "XPath expression is not valid"),
			content: "GET http://example.com\nHTTP/1.0 200\n[Asserts]\nxpath \"strong(//head/title)\" == \"Hello\"\n",
			entry:   entry(22),
			message: "\n 4 | xpath \"strong(//head/title)\" == \"Hello\"\n   |       ^^^^^^^^^^^^^^^^^^^^^^ XPath expression is not valid\n   |",
			render: `Invalid XPath expression
  --> test.hurl:4:7
   |
   | GET http://example.com
   | ...
 4 | xpath "strong(//head/title)" == "Hello"
   |       ^^^^^^^^^^^^^^^^^^^^^^ XPath expression is not valid
   |`,
		},
		{
			name: "assert jsonpath",
			err: testError{
				sourceInfo:  si(4, 0, 4, 0),
				description: "Assert failure",
				fixme: func([]string) text.StyledString {
					var s text.StyledString
					s.PushWith("   actual:   integer <2>", text.NewStyle().Red().Bold())
					s.Push("\n")
					s.PushWith("   expected: greater than integer <5>", text.NewStyle().Red().Bold())
					return s
				},
			},
			content: "GET http://api\nHTTP/1.0 200\n[Asserts]\njsonpath \"$.count\" >= 5\n",
			entry:   entry(14),
			message: `
 4 | jsonpath "$.count" >= 5
   |   actual:   integer <2>
   |   expected: greater than integer <5>
   |`,
			render: `Assert failure
  --> test.hurl:4:0
   |
   | GET http://api
   | ...
 4 | jsonpath "$.count" >= 5
   |   actual:   integer <2>
   |   expected: greater than integer <5>
   |`,
		},
	}
	for _, tt := range tests {
		lines := Lines(tt.content)
		if tt.message != "" {
			if got := Message(tt.err, lines).Render(text.FormatPlain); got != tt.message {
				t.Errorf("%s: message:\n%q\nwant:\n%q", tt.name, got, tt.message)
			}
		}
		if got := Render(tt.err, "test.hurl", tt.content, tt.entry, OutputFormatTerminal{Color: false}); got != tt.render {
			t.Errorf("%s: render:\n%s\nwant:\n%s", tt.name, got, tt.render)
		}
	}
}

// From hurl runner/error.rs (test_assert_error_status).
func TestAssertStatusANSI(t *testing.T) {
	e := caretError(si(2, 10, 2, 13), "Assert status code", "actual value is <404>")
	got := Message(e, Lines("GET http://unknown\nHTTP/1.0 200\n")).Render(text.FormatANSI)
	want := "\n\x1b[1;34m 2 |\x1b[0m HTTP/1.0 200\n\x1b[1;34m   |\x1b[0m\x1b[1;31m          ^^^ actual value is <404>\x1b[0m\n\x1b[1;34m   |\x1b[0m"
	if got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestRenderCRLF(t *testing.T) {
	e := caretError(si(1, 5, 1, 8), "Parsing\r\nURL", "oops")
	content := "GET abc\r\nHTTP 200\r\n"
	body := "\n  --> f:1:5\n   |\n 1 | GET abc\n   |     ^^^ oops\n   |"
	if got, want := Render(e, "f", content, nil, OutputFormatTerminal{}), "Parsing\nURL"+body; got != want {
		t.Errorf("terminal: got %q", got)
	}
	if got, want := Render(e, "f", content, nil, OutputFormatPlain{}), "Parsing\r\nURL"+body; got != want {
		t.Errorf("plain: got %q", got)
	}
}

func TestGutterWidth(t *testing.T) {
	content := strings.Repeat("x\n", 120)
	e := caretError(si(100, 1, 100, 2), "Error", "here")
	got := Render(e, "f", content, nil, OutputFormatPlain{})
	want := "Error\n   --> f:100:1\n    |\n100 | x\n    | ^ here\n    |"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLines(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"\n", []string{""}},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"a\n\nb", []string{"a", "", "b"}},
		// Same example as Rust's str::lines documentation.
		{"foo\r\nbar\n\nbaz\r", []string{"foo", "bar", "", "baz\r"}},
	}
	for _, tt := range tests {
		got := Lines(tt.in)
		if fmt.Sprintf("%q", got) != fmt.Sprintf("%q", tt.want) {
			t.Errorf("Lines(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
