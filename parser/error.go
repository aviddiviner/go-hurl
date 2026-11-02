package parser

import (
	"fmt"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/text"
	"github.com/aviddiviner/go-hurl/reader"
)

// ParseError is a parser error. Recoverable errors let a combinator try the
// next alternative; non-recoverable errors stop the parse.
type ParseError struct {
	Pos         reader.Pos
	Recoverable bool
	Kind        ParseErrorKind
}

func newError(pos reader.Pos, recoverable bool, kind ParseErrorKind) *ParseError {
	return &ParseError{Pos: pos, Recoverable: recoverable, Kind: kind}
}

func (e *ParseError) toRecoverable() *ParseError {
	e2 := *e
	e2.Recoverable = true
	return &e2
}

func (e *ParseError) toNonRecoverable() *ParseError {
	e2 := *e
	e2.Recoverable = false
	return &e2
}

// ParseErrorKind is one of the ParseError* variant types.
type ParseErrorKind interface{ parseErrorKind() }

type ParseErrorDuplicateSection struct{}
type ParseErrorEscapeChar struct{}
type ParseErrorExpecting struct{ Value string }
type ParseErrorFileContentType struct{}
type ParseErrorFilename struct{}
type ParseErrorGraphQLVariables struct{}
type ParseErrorHexDigit struct{}
type ParseErrorInvalidCookieAttribute struct{}
type ParseErrorInvalidDurationUnit struct{ Value string }
type ParseErrorInvalidOption struct{ Value string }
type ParseErrorJSON struct{ Value JSONErrorVariant }
type ParseErrorJSONPathExpr struct{}
type ParseErrorMethod struct{ Name string }
type ParseErrorMultiline struct{}
type ParseErrorMultilineLanguageHint struct{ Value string }
type ParseErrorOddNumberOfHexDigits struct{}
type ParseErrorPredicate struct{}
type ParseErrorPredicateValue struct{}
type ParseErrorRegexExpr struct{ Message string }
type ParseErrorRequestSection struct{}
type ParseErrorRequestSectionName struct{ Name string }
type ParseErrorResponseSection struct{}
type ParseErrorResponseSectionName struct{ Name string }
type ParseErrorSpace struct{}
type ParseErrorStatus struct{}
type ParseErrorTemplateVariable struct{}
type ParseErrorUnicode struct{}
type ParseErrorURLIllegalCharacter struct{ Value rune }
type ParseErrorURLInvalidStart struct{}
type ParseErrorVariable struct{ Value string }
type ParseErrorVersion struct{}
type ParseErrorXPathExpr struct{}
type ParseErrorXML struct{}

func (ParseErrorDuplicateSection) parseErrorKind()       {}
func (ParseErrorEscapeChar) parseErrorKind()             {}
func (ParseErrorExpecting) parseErrorKind()              {}
func (ParseErrorFileContentType) parseErrorKind()        {}
func (ParseErrorFilename) parseErrorKind()               {}
func (ParseErrorGraphQLVariables) parseErrorKind()       {}
func (ParseErrorHexDigit) parseErrorKind()               {}
func (ParseErrorInvalidCookieAttribute) parseErrorKind() {}
func (ParseErrorInvalidDurationUnit) parseErrorKind()    {}
func (ParseErrorInvalidOption) parseErrorKind()          {}
func (ParseErrorJSON) parseErrorKind()                   {}
func (ParseErrorJSONPathExpr) parseErrorKind()           {}
func (ParseErrorMethod) parseErrorKind()                 {}
func (ParseErrorMultiline) parseErrorKind()              {}
func (ParseErrorMultilineLanguageHint) parseErrorKind()  {}
func (ParseErrorOddNumberOfHexDigits) parseErrorKind()   {}
func (ParseErrorPredicate) parseErrorKind()              {}
func (ParseErrorPredicateValue) parseErrorKind()         {}
func (ParseErrorRegexExpr) parseErrorKind()              {}
func (ParseErrorRequestSection) parseErrorKind()         {}
func (ParseErrorRequestSectionName) parseErrorKind()     {}
func (ParseErrorResponseSection) parseErrorKind()        {}
func (ParseErrorResponseSectionName) parseErrorKind()    {}
func (ParseErrorSpace) parseErrorKind()                  {}
func (ParseErrorStatus) parseErrorKind()                 {}
func (ParseErrorTemplateVariable) parseErrorKind()       {}
func (ParseErrorUnicode) parseErrorKind()                {}
func (ParseErrorURLIllegalCharacter) parseErrorKind()    {}
func (ParseErrorURLInvalidStart) parseErrorKind()        {}
func (ParseErrorVariable) parseErrorKind()               {}
func (ParseErrorVersion) parseErrorKind()                {}
func (ParseErrorXPathExpr) parseErrorKind()              {}
func (ParseErrorXML) parseErrorKind()                    {}

// JSONErrorVariant is one of the JSONError* variant types.
type JSONErrorVariant interface{ jsonErrorVariant() }

type JSONErrorMaxDepthExceeded struct{ MaxDepth int }
type JSONErrorTrailingComma struct{}
type JSONErrorExpectingElement struct{}
type JSONErrorEmptyElement struct{}

func (JSONErrorMaxDepthExceeded) jsonErrorVariant() {}
func (JSONErrorTrailingComma) jsonErrorVariant()    {}
func (JSONErrorExpectingElement) jsonErrorVariant() {}
func (JSONErrorEmptyElement) jsonErrorVariant()     {}

var _ sourceerr.DisplaySourceError = (*ParseError)(nil)

func (e *ParseError) SourceInfo() ast.SourceInfo {
	return ast.SourceInfo{Start: e.Pos, End: e.Pos}
}

func (e *ParseError) Description() string {
	switch e.Kind.(type) {
	case ParseErrorDuplicateSection:
		return "Parsing section"
	case ParseErrorEscapeChar:
		return "Parsing escape character"
	case ParseErrorExpecting:
		return "Parsing literal"
	case ParseErrorFileContentType:
		return "Parsing file content type"
	case ParseErrorFilename:
		return "Parsing filename"
	case ParseErrorGraphQLVariables:
		return "Parsing GraphQL variables"
	case ParseErrorHexDigit:
		return "Parsing hexadecimal number"
	case ParseErrorInvalidCookieAttribute:
		return "Parsing cookie attribute"
	case ParseErrorInvalidOption:
		return "Parsing option"
	case ParseErrorInvalidDurationUnit:
		return "Parsing duration"
	case ParseErrorJSON:
		return "Parsing JSON"
	case ParseErrorJSONPathExpr:
		return "Parsing JSONPath expression"
	case ParseErrorMethod:
		return "Parsing method"
	case ParseErrorMultiline, ParseErrorMultilineLanguageHint:
		return "Parsing multiline"
	case ParseErrorOddNumberOfHexDigits:
		return "Parsing hex bytearray"
	case ParseErrorPredicate:
		return "Parsing predicate"
	case ParseErrorPredicateValue:
		return "Parsing predicate value"
	case ParseErrorRegexExpr:
		return "Parsing regex"
	case ParseErrorRequestSection:
		return "Parsing section"
	case ParseErrorRequestSectionName:
		return "Parsing request section name"
	case ParseErrorResponseSection:
		return "Parsing section"
	case ParseErrorResponseSectionName:
		return "Parsing response section name"
	case ParseErrorSpace:
		return "Parsing space"
	case ParseErrorStatus:
		return "Parsing status code"
	case ParseErrorTemplateVariable:
		return "Parsing template variable"
	case ParseErrorUnicode:
		return "Parsing unicode literal"
	case ParseErrorURLIllegalCharacter, ParseErrorURLInvalidStart:
		return "Parsing URL"
	case ParseErrorVariable:
		return "Parsing variable"
	case ParseErrorVersion:
		return "Parsing version"
	case ParseErrorXPathExpr:
		return "Parsing XPath expression"
	case ParseErrorXML:
		return "Parsing XML"
	}
	return ""
}

var validOptionNames = []string{
	"aws-sigv4",
	"cacert",
	"cert",
	"compressed",
	"connect-timeout",
	"connect-to",
	"delay",
	"digest",
	"fail-with-body",
	"header",
	"http1.0",
	"http1.1",
	"http2",
	"http2-prior-knowledge",
	"http3",
	"insecure",
	"ipv4",
	"ipv6",
	"key",
	"limit-rate",
	"location",
	"location-trusted",
	"max-redirs",
	"max-time",
	"negotiate",
	"netrc",
	"netrc-file",
	"netrc-optional",
	"no-header",
	"no-jsonpath-coercion",
	"ntlm",
	"output",
	"path-as-is",
	"pinnedpubkey",
	"proxy",
	"repeat",
	"resolve",
	"retry",
	"retry-interval",
	"skip",
	"tls-max",
	"tls-min",
	"unix-socket",
	"user",
	"variable",
	"verbose",
	"verbosity",
	"very-verbose",
}

func (e *ParseError) message() string {
	switch k := e.Kind.(type) {
	case ParseErrorDuplicateSection:
		return "the section is already defined"
	case ParseErrorEscapeChar:
		return "the escaping sequence is not valid"
	case ParseErrorExpecting:
		return fmt.Sprintf("expecting '%s'", k.Value)
	case ParseErrorFileContentType:
		return "expecting a content type"
	case ParseErrorFilename:
		return "expecting a filename"
	case ParseErrorGraphQLVariables:
		return "GraphQL variables is not a valid JSON object"
	case ParseErrorHexDigit:
		return "expecting a valid hexadecimal number"
	case ParseErrorInvalidCookieAttribute:
		return "the cookie attribute is not valid"
	case ParseErrorInvalidDurationUnit:
		validValues := []string{"ms", "s"}
		def := "Valid values are " + strings.Join(validValues, ", ")
		return "the duration unit is not valid. " + didYouMean(validValues, k.Value, def)
	case ParseErrorInvalidOption:
		def := "Valid values are " + strings.Join(validOptionNames, ", ")
		return "the option name is not valid. " + didYouMean(validOptionNames, k.Value, def)
	case ParseErrorJSON:
		switch v := k.Value.(type) {
		case JSONErrorMaxDepthExceeded:
			return fmt.Sprintf("maximum nesting depth of %d exceeded", v.MaxDepth)
		case JSONErrorTrailingComma:
			return "trailing comma is not allowed"
		case JSONErrorEmptyElement:
			return "expecting an element; found empty element instead"
		case JSONErrorExpectingElement:
			return "expecting a boolean, number, string, array, object or null"
		}
	case ParseErrorJSONPathExpr:
		return "expecting a JSONPath expression"
	case ParseErrorMethod:
		validValues := []string{"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH"}
		def := "Valid values are " + strings.Join(validValues, ", ")
		return fmt.Sprintf("the HTTP method <%s> is not valid. %s", k.Name, didYouMean(validValues, k.Name, def))
	case ParseErrorMultiline:
		return "the multiline is not valid"
	case ParseErrorMultilineLanguageHint:
		return "Invalid language hint " + k.Value
	case ParseErrorOddNumberOfHexDigits:
		return "expecting an even number of hex digits"
	case ParseErrorPredicate:
		return "expecting a predicate"
	case ParseErrorPredicateValue:
		return "invalid predicate value"
	case ParseErrorRegexExpr:
		return "invalid Regex expression: " + k.Message
	case ParseErrorRequestSection:
		return "this is not a valid section for a request"
	case ParseErrorRequestSectionName:
		validValues := []string{"Query", "Form", "Multipart", "Cookies", "Options"}
		def := "Valid values are " + strings.Join(validValues, ", ")
		return "the section is not valid. " + didYouMean(validValues, k.Name, def)
	case ParseErrorResponseSection:
		return "this is not a valid section for a response"
	case ParseErrorResponseSectionName:
		validValues := []string{"Captures", "Asserts"}
		return "the section is not valid. " + didYouMean(validValues, k.Name, "Valid values are Captures or Asserts")
	case ParseErrorSpace:
		return "expecting a space"
	case ParseErrorStatus:
		return "HTTP status code is not valid"
	case ParseErrorTemplateVariable:
		return "expecting a variable"
	case ParseErrorUnicode:
		return "Invalid unicode literal"
	case ParseErrorURLIllegalCharacter:
		return fmt.Sprintf("illegal character <%c>", k.Value)
	case ParseErrorURLInvalidStart:
		return "expecting http://, https:// or {{"
	case ParseErrorVariable:
		return k.Value
	case ParseErrorVersion:
		return "HTTP version must be HTTP, HTTP/1.0, HTTP/1.1, HTTP/2 or HTTP/3"
	case ParseErrorXPathExpr:
		return "expecting a XPath expression"
	case ParseErrorXML:
		return "invalid XML"
	}
	return ""
}

func (e *ParseError) Fixme(content []string) text.StyledString {
	message := sourceerr.AddCarets(e.message(), e.SourceInfo(), content)
	var s text.StyledString
	s.PushWith(message, text.NewStyle().Red().Bold())
	return s
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", e.Pos.Line, e.Pos.Column, e.Description(), e.message())
}

func didYouMean(validValues []string, actual, def string) string {
	if s, ok := suggestion(validValues, actual); ok {
		return "Did you mean " + s + "?"
	}
	return def
}

func suggestion(validValues []string, actual string) (string, bool) {
	const suggestionMaxLength = 256
	if len(actual) > suggestionMaxLength {
		return "", false
	}
	for _, value := range validValues {
		if levenshteinDistance(strings.ToLower(value), strings.ToLower(actual)) < 2 {
			return value, true
		}
	}
	return "", false
}

func levenshteinDistance(s1, s2 string) int {
	v1 := []rune(s1)
	v2 := []rune(s2)
	column := make([]int, len(v1)+1)
	for i := range column {
		column[i] = i
	}
	for x := 1; x <= len(v2); x++ {
		column[0] = x
		lastdiag := x - 1
		for y := 1; y <= len(v1); y++ {
			olddiag := column[y]
			delta := 0
			if v1[y-1] != v2[x-1] {
				delta = 1
			}
			column[y] = min(column[y]+1, column[y-1]+1, lastdiag+delta)
			lastdiag = olddiag
		}
	}
	return column[len(v1)]
}
