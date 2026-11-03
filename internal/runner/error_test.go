package runner

import (
	"testing"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/text"
)

func TestErrorTimeout(t *testing.T) {
	content := "GET http://unknown"
	lines := sourceerr.Lines(content)
	kind := RunnerErrorHTTP{Value: httpclient.HttpErrorLibcurl{Code: 6, Detail: "Could not resolve host: unknown"}}
	entry := sourceInfo(1, 1, 1, 19)
	err := NewRunnerError(sourceInfo(1, 5, 1, 19), kind, true)

	if got, want := err.Message(lines).Render(text.FormatPlain), "\n 1 | GET http://unknown\n   |     ^^^^^^^^^^^^^^ (6) Could not resolve host: unknown\n   |"; got != want {
		t.Errorf("Message() =\n%q\nwant\n%q", got, want)
	}
	want := `HTTP connection
  --> test.hurl:1:5
   |
 1 | GET http://unknown
   |     ^^^^^^^^^^^^^^ (6) Could not resolve host: unknown
   |`
	if got := sourceerr.Render(err, "test.hurl", content, &entry, sourceerr.OutputFormatTerminal{}); got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestAssertErrorStatus(t *testing.T) {
	content := "GET http://unknown\nHTTP/1.0 200\n"
	lines := sourceerr.Lines(content)
	entry := sourceInfo(1, 1, 1, 18)
	err := NewRunnerError(sourceInfo(2, 10, 2, 13), RunnerErrorAssertStatus{Actual: "404"}, true)

	if got, want := err.Message(lines).Render(text.FormatPlain), "\n 2 | HTTP/1.0 200\n   |          ^^^ actual value is <404>\n   |"; got != want {
		t.Errorf("Message() = %q, want %q", got, want)
	}
	if got, want := err.Message(lines).Render(text.FormatANSI), "\n\x1b[1;34m 2 |\x1b[0m HTTP/1.0 200\n\x1b[1;34m   |\x1b[0m\x1b[1;31m          ^^^ actual value is <404>\x1b[0m\n\x1b[1;34m   |\x1b[0m"; got != want {
		t.Errorf("Message() ANSI = %q, want %q", got, want)
	}
	want := `Assert status code
  --> test.hurl:2:10
   |
   | GET http://unknown
 2 | HTTP/1.0 200
   |          ^^^ actual value is <404>
   |`
	if got := sourceerr.Render(err, "test.hurl", content, &entry, sourceerr.OutputFormatTerminal{}); got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestInvalidXPathExpression(t *testing.T) {
	content := "GET http://example.com\nHTTP/1.0 200\n[Asserts]\nxpath \"strong(//head/title)\" == \"Hello\"\n"
	lines := sourceerr.Lines(content)
	entry := sourceInfo(1, 1, 1, 22)
	err := NewRunnerError(sourceInfo(4, 7, 4, 29), RunnerErrorInvalidXPathEval{}, true)
	if got, want := err.Message(lines).Render(text.FormatPlain), "\n 4 | xpath \"strong(//head/title)\" == \"Hello\"\n   |       ^^^^^^^^^^^^^^^^^^^^^^ XPath expression is not valid\n   |"; got != want {
		t.Errorf("Message() = %q, want %q", got, want)
	}
	want := `Invalid XPath expression
  --> test.hurl:4:7
   |
   | GET http://example.com
   | ...
 4 | xpath "strong(//head/title)" == "Hello"
   |       ^^^^^^^^^^^^^^^^^^^^^^ XPath expression is not valid
   |`
	if got := sourceerr.Render(err, "test.hurl", content, &entry, sourceerr.OutputFormatTerminal{}); got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestAssertErrorJSONPath(t *testing.T) {
	content := "GET http://api\nHTTP/1.0 200\n[Asserts]\njsonpath \"$.count\" >= 5\n"
	lines := sourceerr.Lines(content)
	entry := sourceInfo(1, 1, 1, 14)
	err := &RunnerError{
		sourceInfo: sourceInfo(4, 0, 4, 0),
		Kind:       RunnerErrorAssertFailure{Actual: "integer <2>", Expected: "greater than integer <5>"},
		Assert:     true,
	}
	if got, want := err.Message(lines).Render(text.FormatPlain), `
 4 | jsonpath "$.count" >= 5
   |   actual:   integer <2>
   |   expected: greater than integer <5>
   |`; got != want {
		t.Errorf("Message() =\n%s\nwant\n%s", got, want)
	}
	want := `Assert failure
  --> test.hurl:4:0
   |
   | GET http://api
   | ...
 4 | jsonpath "$.count" >= 5
   |   actual:   integer <2>
   |   expected: greater than integer <5>
   |`
	if got := sourceerr.Render(err, "test.hurl", content, &entry, sourceerr.OutputFormatTerminal{}); got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestAssertErrorNewline(t *testing.T) {
	content := "GET http://localhost\nHTTP/1.0 200\n```\n<p>Hello</p>\n```\n"
	lines := sourceerr.Lines(content)
	entry := sourceInfo(1, 1, 1, 20)
	// The hunk of diff("<p>Hello</p>\n", "<p>Hello</p>\n\n").
	var hunk text.StyledString
	hunk.PushWith("+\n", text.NewStyle().Green())
	kind := RunnerErrorAssertBodyDiffError{
		Hunks:          []DiffHunk{{Content: hunk, Start: 0, SourceLine: 0}},
		BodySourceInfo: sourceInfo(4, 1, 4, 1),
	}
	err := NewRunnerError(sourceInfo(4, 1, 4, 1), kind, true)
	if got, want := err.Message(lines).Render(text.FormatPlain), "\n 4 | <p>Hello</p>\n   |   +\n   |"; got != want {
		t.Errorf("Message() = %q, want %q", got, want)
	}
	want := "Assert body value\n  --> test.hurl:4:1\n   |\n   | GET http://localhost\n   | ...\n 4 | <p>Hello</p>\n   |   +\n   |"
	if got := sourceerr.Render(err, "test.hurl", content, &entry, sourceerr.OutputFormatTerminal{}); got != want {
		t.Errorf("Render() =\n%s\nwant\n%s", got, want)
	}
}

func TestErrorDescriptions(t *testing.T) {
	content := []string{"GET http://localhost"}
	si := sourceInfo(1, 5, 1, 21)
	tests := []struct {
		kind              RunnerErrorKind
		description, line string
	}{
		{RunnerErrorFilterInvalidInputType{Actual: "boolean", Expected: "string"}, "Filter error", "    ^^^^^^^^^^^^^^^^ invalid filter input type\n   |                         actual:   boolean\n   |                         expected: string"},
		{RunnerErrorFilterDateParsingError{Date: "x", Format: "%Y"}, "Filter error", "    ^^^^^^^^^^^^^^^^ value <x> could not be parsed with <%Y> format"},
		{RunnerErrorTemplateVariableNotDefined{Name: "foo"}, "Undefined variable", "    ^^^^^^^^^^^^^^^^ you must set the variable foo"},
		{RunnerErrorUnauthorizedFileAccess{Path: "/etc/passwd"}, "Unauthorized file access", "    ^^^^^^^^^^^^^^^^ unauthorized access to file /etc/passwd, check --file-root option"},
		{RunnerErrorQueryInvalidJSONPathExpression{Value: "$$"}, "Invalid JSONPath", "    ^^^^^^^^^^^^^^^^ JSONPath expression '$$' is not valid"},
		{RunnerErrorInvalidURL{URL: "x", Message: "y"}, "Invalid URL", "    ^^^^^^^^^^^^^^^^ invalid URL <x> (y)"},
		{RunnerErrorAssertFailure{Actual: "a", Expected: "b", TypeMismatch: true}, "Assert failure", "  actual:   a\n   |   expected: b\n   |   >>> types between actual and expected are not consistent"},
	}
	for _, tt := range tests {
		err := NewRunnerError(si, tt.kind, false)
		if got := err.Description(); got != tt.description {
			t.Errorf("%T Description() = %q, want %q", tt.kind, got, tt.description)
		}
		want := "\n 1 | GET http://localhost\n   | " + tt.line + "\n   |"
		if got := err.Message(content).Render(text.FormatPlain); got != want {
			t.Errorf("%T Message() =\n%q\nwant\n%q", tt.kind, got, want)
		}
	}
}
