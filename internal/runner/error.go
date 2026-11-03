package runner

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/text"
)

// RunnerError is a runtime error, usually raised by running an entry. Its
// kind is one of the RunnerError* variant types.
type RunnerError struct {
	sourceInfo ast.SourceInfo
	Kind       RunnerErrorKind
	Assert     bool
}

// NewRunnerError returns a runner error.
func NewRunnerError(sourceInfo ast.SourceInfo, kind RunnerErrorKind, assert bool) *RunnerError {
	return &RunnerError{sourceInfo: sourceInfo, Kind: kind, Assert: assert}
}

// RunnerErrorKind is the variant of a RunnerError.
type RunnerErrorKind interface{ runnerErrorKind() }

// DiffHunk is a hunk of a body diff (from runner/diff.rs, owned by the
// execution layer): its styled content, its 0-based start line and its
// 0-based line in the source.
type DiffHunk struct {
	Content    text.StyledString
	Start      int
	SourceLine int
}

type RunnerErrorAssertBodyDiffError struct {
	BodySourceInfo ast.SourceInfo
	Hunks          []DiffHunk
}
type RunnerErrorAssertBodyValueError struct{ Actual, Expected string }
type RunnerErrorAssertFailure struct {
	Actual, Expected string
	TypeMismatch     bool
}
type RunnerErrorAssertHeaderValueError struct{ Actual string }
type RunnerErrorAssertStatus struct{ Actual string }
type RunnerErrorAssertVersion struct{ Actual string }

// RunnerErrorBinaryOutput is raised when the user tries to output binary
// data to standard output.
type RunnerErrorBinaryOutput struct{}
type RunnerErrorExpressionInvalidType struct{ Value, Expecting string }

// RunnerErrorFileReadAccess is an I/O read error on Path.
type RunnerErrorFileReadAccess struct{ Path string }

// RunnerErrorFileWriteAccess is an I/O write error on Path.
type RunnerErrorFileWriteAccess struct{ Path, Error string }
type RunnerErrorFilterDecode struct{ Value string }
type RunnerErrorFilterDateParsingError struct{ Date, Format string }
type RunnerErrorFilterInvalidEncoding struct{ Value string }

// RunnerErrorFilterInvalidInputValue is an invalid filter input, with a
// reason.
type RunnerErrorFilterInvalidInputValue struct{ Value string }

// RunnerErrorFilterInvalidInputType is a filter input of the wrong type.
type RunnerErrorFilterInvalidInputType struct{ Actual, Expected string }
type RunnerErrorFilterInvalidFormatSpecifier struct{ Value string }
type RunnerErrorFilterMissingInput struct{}
type RunnerErrorHTTP struct{ Value httpclient.HttpError }
type RunnerErrorInvalidJSON struct{ Value string }
type RunnerErrorInvalidOptionValue struct{ Name, Value, Message string }
type RunnerErrorInvalidRegex struct{}
type RunnerErrorInvalidURL struct{ URL, Message string }

// RunnerErrorInvalidXPathEval is raised when an XPath expression fails to
// evaluate.
type RunnerErrorInvalidXPathEval struct{}

// RunnerErrorNoFilterResult is raised when a filter in a chain returns no
// value.
type RunnerErrorNoFilterResult struct{}

// RunnerErrorNoQueryResult is raised when a query returns no value.
type RunnerErrorNoQueryResult struct{}
type RunnerErrorPossibleLoggedSecret struct{}
type RunnerErrorQueryHeaderNotFound struct{}
type RunnerErrorQueryInvalidJSONPathExpression struct{ Value string }
type RunnerErrorQueryInvalidXML struct{}
type RunnerErrorQueryInvalidJSON struct{}
type RunnerErrorTemplateVariableNotDefined struct{ Name string }

// RunnerErrorUnauthorizedFileAccess is an access outside --file-root.
type RunnerErrorUnauthorizedFileAccess struct{ Path string }

// RunnerErrorUnsupportedSecretType is raised for non-string secrets.
type RunnerErrorUnsupportedSecretType struct{ Value string }
type RunnerErrorUnrenderableExpression struct{ Value string }

func (RunnerErrorAssertBodyDiffError) runnerErrorKind()            {}
func (RunnerErrorAssertBodyValueError) runnerErrorKind()           {}
func (RunnerErrorAssertFailure) runnerErrorKind()                  {}
func (RunnerErrorAssertHeaderValueError) runnerErrorKind()         {}
func (RunnerErrorAssertStatus) runnerErrorKind()                   {}
func (RunnerErrorAssertVersion) runnerErrorKind()                  {}
func (RunnerErrorBinaryOutput) runnerErrorKind()                   {}
func (RunnerErrorExpressionInvalidType) runnerErrorKind()          {}
func (RunnerErrorFileReadAccess) runnerErrorKind()                 {}
func (RunnerErrorFileWriteAccess) runnerErrorKind()                {}
func (RunnerErrorFilterDecode) runnerErrorKind()                   {}
func (RunnerErrorFilterDateParsingError) runnerErrorKind()         {}
func (RunnerErrorFilterInvalidEncoding) runnerErrorKind()          {}
func (RunnerErrorFilterInvalidInputValue) runnerErrorKind()        {}
func (RunnerErrorFilterInvalidInputType) runnerErrorKind()         {}
func (RunnerErrorFilterInvalidFormatSpecifier) runnerErrorKind()   {}
func (RunnerErrorFilterMissingInput) runnerErrorKind()             {}
func (RunnerErrorHTTP) runnerErrorKind()                           {}
func (RunnerErrorInvalidJSON) runnerErrorKind()                    {}
func (RunnerErrorInvalidOptionValue) runnerErrorKind()             {}
func (RunnerErrorInvalidRegex) runnerErrorKind()                   {}
func (RunnerErrorInvalidURL) runnerErrorKind()                     {}
func (RunnerErrorInvalidXPathEval) runnerErrorKind()               {}
func (RunnerErrorNoFilterResult) runnerErrorKind()                 {}
func (RunnerErrorNoQueryResult) runnerErrorKind()                  {}
func (RunnerErrorPossibleLoggedSecret) runnerErrorKind()           {}
func (RunnerErrorQueryHeaderNotFound) runnerErrorKind()            {}
func (RunnerErrorQueryInvalidJSONPathExpression) runnerErrorKind() {}
func (RunnerErrorQueryInvalidXML) runnerErrorKind()                {}
func (RunnerErrorQueryInvalidJSON) runnerErrorKind()               {}
func (RunnerErrorTemplateVariableNotDefined) runnerErrorKind()     {}
func (RunnerErrorUnauthorizedFileAccess) runnerErrorKind()         {}
func (RunnerErrorUnsupportedSecretType) runnerErrorKind()          {}
func (RunnerErrorUnrenderableExpression) runnerErrorKind()         {}

// SourceInfo returns the position of the error.
func (e *RunnerError) SourceInfo() ast.SourceInfo { return e.sourceInfo }

// SetSourceInfo changes the position of the error.
func (e *RunnerError) SetSourceInfo(sourceInfo ast.SourceInfo) { e.sourceInfo = sourceInfo }

func (e *RunnerError) Error() string { return e.Description() }

// Description returns the short description of the error, used as title.
func (e *RunnerError) Description() string {
	switch k := e.Kind.(type) {
	case RunnerErrorAssertBodyDiffError, RunnerErrorAssertBodyValueError:
		return "Assert body value"
	case RunnerErrorAssertFailure:
		return "Assert failure"
	case RunnerErrorAssertHeaderValueError:
		return "Assert header value"
	case RunnerErrorAssertStatus:
		return "Assert status code"
	case RunnerErrorAssertVersion:
		return "Assert HTTP version"
	case RunnerErrorBinaryOutput:
		return "Binary output"
	case RunnerErrorExpressionInvalidType:
		return "Invalid expression type"
	case RunnerErrorFileReadAccess:
		return "File read access"
	case RunnerErrorFileWriteAccess:
		return "File write access"
	case RunnerErrorFilterDateParsingError, RunnerErrorFilterDecode, RunnerErrorFilterInvalidEncoding,
		RunnerErrorFilterInvalidInputValue, RunnerErrorFilterInvalidInputType,
		RunnerErrorFilterInvalidFormatSpecifier, RunnerErrorFilterMissingInput, RunnerErrorNoFilterResult:
		return "Filter error"
	case RunnerErrorHTTP:
		return k.Value.Description()
	case RunnerErrorInvalidJSON, RunnerErrorQueryInvalidJSON:
		return "Invalid JSON"
	case RunnerErrorInvalidOptionValue:
		return "Invalid option value"
	case RunnerErrorInvalidRegex:
		return "Invalid regex"
	case RunnerErrorInvalidURL:
		return "Invalid URL"
	case RunnerErrorInvalidXPathEval:
		return "Invalid XPath expression"
	case RunnerErrorNoQueryResult:
		return "No query result"
	case RunnerErrorPossibleLoggedSecret:
		return "Invalid redacted secret"
	case RunnerErrorQueryHeaderNotFound:
		return "Header not found"
	case RunnerErrorQueryInvalidJSONPathExpression:
		return "Invalid JSONPath"
	case RunnerErrorQueryInvalidXML:
		return "Invalid XML"
	case RunnerErrorTemplateVariableNotDefined:
		return "Undefined variable"
	case RunnerErrorUnauthorizedFileAccess:
		return "Unauthorized file access"
	case RunnerErrorUnrenderableExpression:
		return "Unrenderable expression"
	case RunnerErrorUnsupportedSecretType:
		return "Invalid secret type"
	}
	return ""
}

// Fixme returns the message displayed under the source line.
func (e *RunnerError) Fixme(content []string) text.StyledString {
	if k, ok := e.Kind.(RunnerErrorAssertBodyDiffError); ok {
		// As in Hurl, this variant isn't reached: Message doesn't call it.
		var message text.StyledString
		if len(k.Hunks) > 0 {
			message.Append(k.Hunks[0].Content.Clone())
		}
		return message
	}
	if k, ok := e.Kind.(RunnerErrorAssertFailure); ok {
		additional := ""
		if k.TypeMismatch {
			additional = "\n   >>> types between actual and expected are not consistent"
		}
		return colorRedMultilineString(fmt.Sprintf("   actual:   %s\n   expected: %s%s", k.Actual, k.Expected, additional))
	}
	message := sourceerr.AddCarets(e.fixmeMessage(), e.sourceInfo, content)
	return colorRedMultilineString(message)
}

func (e *RunnerError) fixmeMessage() string {
	switch k := e.Kind.(type) {
	case RunnerErrorAssertBodyValueError:
		return "actual value is <" + k.Actual + ">"
	case RunnerErrorAssertHeaderValueError:
		return "actual value is <" + k.Actual + ">"
	case RunnerErrorAssertStatus:
		return "actual value is <" + k.Actual + ">"
	case RunnerErrorBinaryOutput:
		return `binary output can mess up your terminal. Use "--output -" to tell Hurl to output it to your terminal anyway, or consider "--output" to save to a file`
	case RunnerErrorAssertVersion:
		return "actual value is <" + k.Actual + ">"
	case RunnerErrorExpressionInvalidType:
		return "expecting " + k.Expecting + ", actual value is " + k.Value
	case RunnerErrorFileReadAccess:
		return "file " + k.Path + " can not be read"
	case RunnerErrorFileWriteAccess:
		return k.Path + " can not be written (" + k.Error + ")"
	case RunnerErrorFilterDateParsingError:
		return "value <" + k.Date + "> could not be parsed with <" + k.Format + "> format"
	case RunnerErrorFilterDecode:
		return "value can not be decoded with <" + k.Value + "> encoding"
	case RunnerErrorFilterInvalidEncoding:
		return "<" + k.Value + "> encoding is not supported"
	case RunnerErrorFilterInvalidInputValue:
		return "invalid filter input: " + k.Value
	case RunnerErrorFilterInvalidInputType:
		return "invalid filter input type\n   actual:   " + k.Actual + "\n   expected: " + k.Expected
	case RunnerErrorFilterInvalidFormatSpecifier:
		return "date format <" + k.Value + "> is not supported"
	case RunnerErrorFilterMissingInput:
		return "missing value to apply filter"
	case RunnerErrorHTTP:
		return k.Value.Message()
	case RunnerErrorInvalidJSON:
		return "actual value is <" + k.Value + ">"
	case RunnerErrorInvalidOptionValue:
		return "invalid " + k.Name + " option value <" + k.Value + ">: " + k.Message
	case RunnerErrorInvalidRegex:
		return "regex expression is not valid"
	case RunnerErrorInvalidURL:
		return "invalid URL <" + k.URL + "> (" + k.Message + ")"
	case RunnerErrorInvalidXPathEval:
		return "XPath expression is not valid"
	case RunnerErrorNoFilterResult:
		return "a filter didn't return any result"
	case RunnerErrorNoQueryResult:
		return "query didn't return any result"
	case RunnerErrorPossibleLoggedSecret:
		return "redacted secret not authorized in verbose"
	case RunnerErrorQueryHeaderNotFound:
		return "this header has not been found in the response"
	case RunnerErrorQueryInvalidJSON:
		return "HTTP response is not a valid JSON"
	case RunnerErrorQueryInvalidJSONPathExpression:
		return "JSONPath expression '" + k.Value + "' is not valid"
	case RunnerErrorQueryInvalidXML:
		return "HTTP response is not a valid XML"
	case RunnerErrorTemplateVariableNotDefined:
		return "you must set the variable " + k.Name
	case RunnerErrorUnauthorizedFileAccess:
		return "unauthorized access to file " + k.Path + ", check --file-root option"
	case RunnerErrorUnrenderableExpression:
		return "expression with value " + k.Value + " can not be rendered"
	case RunnerErrorUnsupportedSecretType:
		return "secret must be string, actual value is <" + k.Value + ">"
	}
	return ""
}

// Message returns the source line(s) and the fixme, with a line number
// gutter. Body diff errors show their first hunk instead.
func (e *RunnerError) Message(content []string) text.StyledString {
	k, ok := e.Kind.(RunnerErrorAssertBodyDiffError)
	if !ok {
		return sourceerr.DefaultMessage(e, content)
	}
	locMaxWidth := max(len(strconv.Itoa(len(content))), 2)
	var t text.StyledString
	// Only the first hunk is processed for the time being, as in Hurl.
	if len(k.Hunks) > 0 {
		t.Push("\n")
		t.Append(hunkString(k.Hunks[0], k.BodySourceInfo.Start.Line, content, locMaxWidth))
	}
	return t
}

// colorRedMultilineString colors each line separately.
func colorRedMultilineString(s string) text.StyledString {
	var t text.StyledString
	for i, line := range strings.Split(s, "\n") {
		if i > 0 {
			t.Push("\n")
		}
		t.PushWith(line, text.NewStyle().Red().Bold())
	}
	return t
}

func hunkString(hunk DiffHunk, sourceLine int, content []string, locMaxWidth int) text.StyledString {
	var s text.StyledString
	var prefix text.StyledString
	prefix.PushWith(strings.Repeat(" ", locMaxWidth)+" |", text.NewStyle().Blue().Bold())
	errorLine := sourceLine + hunk.SourceLine
	source := content[errorLine-1]
	s.PushWith(fmt.Sprintf("%*d | %s\n", locMaxWidth, errorLine, source), text.NewStyle().Blue().Bold())
	for i, line := range hunk.Content.Split('\n') {
		if i > 0 {
			s.Push("\n")
		}
		s.Append(prefix.Clone())
		if !line.IsEmpty() {
			s.Push("   ")
			s.Append(line)
		}
	}
	return s
}
