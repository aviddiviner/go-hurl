package ast

import (
	"slices"
	"strconv"
)

// HurlFile is the root node of a Hurl AST.
type HurlFile struct {
	Entries         []Entry
	LineTerminators []LineTerminator
}

// Entry is a request specification and an optional response specification.
type Entry struct {
	Request  Request
	Response *Response
}

// SourceInfo returns the source information for this entry.
func (e Entry) SourceInfo() SourceInfo { return e.Request.Space0.SourceInfo }

type Request struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Method          Method
	Space1          Whitespace
	URL             Template
	LineTerminator0 LineTerminator
	Headers         []KeyValue
	Sections        []Section
	Body            *Body
	SourceInfo      SourceInfo
}

// QueryStringParams returns the query string params of this request.
func (r *Request) QueryStringParams() []KeyValue {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionQueryParams); ok {
			return v.Value
		}
	}
	return nil
}

// FormParams returns the form params of this request.
func (r *Request) FormParams() []KeyValue {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionFormParams); ok {
			return v.Value
		}
	}
	return nil
}

// MultipartFormData returns the multipart form data of this request.
func (r *Request) MultipartFormData() []MultipartParam {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionMultipartFormData); ok {
			return v.Value
		}
	}
	return nil
}

// Cookies returns the cookies of this request.
func (r *Request) Cookies() []Cookie {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionCookies); ok {
			return v.Value
		}
	}
	return nil
}

// BasicAuth returns the basic authentication of this request, or nil.
func (r *Request) BasicAuth() *KeyValue {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionBasicAuth); ok {
			return v.Value
		}
	}
	return nil
}

// Options returns the options specific to this request.
func (r *Request) Options() []EntryOption {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionOptions); ok {
			return v.Value
		}
	}
	return nil
}

type Response struct {
	LineTerminators []LineTerminator
	Version         Version
	Space0          Whitespace
	Status          Status
	Space1          Whitespace
	LineTerminator0 LineTerminator
	Headers         []KeyValue
	Sections        []Section
	Body            *Body
	SourceInfo      SourceInfo
}

// Captures returns the captures of this response.
func (r *Response) Captures() []Capture {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionCaptures); ok {
			return v.Value
		}
	}
	return nil
}

// Asserts returns the asserts of this response.
func (r *Response) Asserts() []Assert {
	for _, s := range r.Sections {
		if v, ok := s.Value.(SectionAsserts); ok {
			return v.Value
		}
	}
	return nil
}

type Method string

func (m Method) String() string   { return string(m) }
func (m Method) ToSource() string { return string(m) }

type Version struct {
	Value      VersionValue
	SourceInfo SourceInfo
}

type VersionValue int

const (
	Version1 VersionValue = iota
	Version11
	Version2
	Version3
	VersionAny
)

func (v VersionValue) String() string {
	switch v {
	case Version1:
		return "HTTP/1.0"
	case Version11:
		return "HTTP/1.1"
	case Version2:
		return "HTTP/2"
	case Version3:
		return "HTTP/3"
	case VersionAny:
		return "HTTP"
	}
	return ""
}

func (v VersionValue) ToSource() string { return v.String() }

type Status struct {
	Value      StatusValue
	SourceInfo SourceInfo
}

// StatusValue is either StatusAny (*) or StatusSpecific.
type StatusValue interface {
	statusValue()
	String() string
	ToSource() string
}

type StatusAny struct{}
type StatusSpecific struct{ Value uint64 }

func (StatusAny) statusValue()      {}
func (StatusSpecific) statusValue() {}

func (StatusAny) String() string        { return "*" }
func (s StatusSpecific) String() string { return strconv.FormatUint(s.Value, 10) }

func (s StatusAny) ToSource() string      { return s.String() }
func (s StatusSpecific) ToSource() string { return s.String() }

type Body struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Value           Bytes
	LineTerminator0 LineTerminator
}

// IsVariableReserved reports whether name is reserved (it would conflict with
// an existing function).
func IsVariableReserved(name string) bool {
	return slices.Contains([]string{"getEnv", "newDate", "newUuid"}, name)
}

type Filter struct {
	SourceInfo SourceInfo
	Value      FilterValue
}

// SpacedFilter is a filter and the whitespace preceding it, as found in
// captures and asserts.
type SpacedFilter struct {
	Space0 Whitespace
	Filter Filter
}

// FilterValue is one of the Filter* variant types.
type FilterValue interface {
	filterValue()
	// Identifier returns the Hurl identifier of this filter.
	Identifier() string
}

type FilterBase64Decode struct{}
type FilterBase64Encode struct{}
type FilterBase64URLSafeDecode struct{}
type FilterBase64URLSafeEncode struct{}
type FilterCharsetDecode struct {
	Space0   Whitespace
	Encoding Template
}
type FilterCount struct{}
type FilterDaysAfterNow struct{}
type FilterDaysBeforeNow struct{}
type FilterDecode struct {
	Space0   Whitespace
	Encoding Template
}
type FilterFirst struct{}
type FilterFormat struct {
	Space0 Whitespace
	Fmt    Template
}
type FilterDateFormat struct {
	Space0 Whitespace
	Fmt    Template
}
type FilterHTMLEscape struct{}
type FilterHTMLUnescape struct{}
type FilterJSONPath struct {
	Space0 Whitespace
	Expr   Template
}
type FilterLast struct{}
type FilterLocation struct{}
type FilterNth struct {
	Space0 Whitespace
	N      IntegerValue
}
type FilterRegex struct {
	Space0 Whitespace
	Value  RegexValue
}
type FilterReplace struct {
	Space0   Whitespace
	OldValue Template
	Space1   Whitespace
	NewValue Template
}
type FilterReplaceRegex struct {
	Space0   Whitespace
	Pattern  RegexValue
	Space1   Whitespace
	NewValue Template
}
type FilterSplit struct {
	Space0 Whitespace
	Sep    Template
}
type FilterToDate struct {
	Space0 Whitespace
	Fmt    Template
}
type FilterToFloat struct{}
type FilterToHex struct{}
type FilterToInt struct{}
type FilterToString struct{}
type FilterURLDecode struct{}
type FilterURLEncode struct{}
type FilterURLQueryParam struct {
	Space0 Whitespace
	Param  Template
}
type FilterUTF8Decode struct{}
type FilterUTF8Encode struct{}
type FilterXPath struct {
	Space0 Whitespace
	Expr   Template
}

func (FilterBase64Decode) filterValue()        {}
func (FilterBase64Encode) filterValue()        {}
func (FilterBase64URLSafeDecode) filterValue() {}
func (FilterBase64URLSafeEncode) filterValue() {}
func (FilterCharsetDecode) filterValue()       {}
func (FilterCount) filterValue()               {}
func (FilterDaysAfterNow) filterValue()        {}
func (FilterDaysBeforeNow) filterValue()       {}
func (FilterDecode) filterValue()              {}
func (FilterFirst) filterValue()               {}
func (FilterFormat) filterValue()              {}
func (FilterDateFormat) filterValue()          {}
func (FilterHTMLEscape) filterValue()          {}
func (FilterHTMLUnescape) filterValue()        {}
func (FilterJSONPath) filterValue()            {}
func (FilterLast) filterValue()                {}
func (FilterLocation) filterValue()            {}
func (FilterNth) filterValue()                 {}
func (FilterRegex) filterValue()               {}
func (FilterReplace) filterValue()             {}
func (FilterReplaceRegex) filterValue()        {}
func (FilterSplit) filterValue()               {}
func (FilterToDate) filterValue()              {}
func (FilterToFloat) filterValue()             {}
func (FilterToHex) filterValue()               {}
func (FilterToInt) filterValue()               {}
func (FilterToString) filterValue()            {}
func (FilterURLDecode) filterValue()           {}
func (FilterURLEncode) filterValue()           {}
func (FilterURLQueryParam) filterValue()       {}
func (FilterUTF8Decode) filterValue()          {}
func (FilterUTF8Encode) filterValue()          {}
func (FilterXPath) filterValue()               {}

func (FilterBase64Decode) Identifier() string        { return "base64Decode" }
func (FilterBase64Encode) Identifier() string        { return "base64Encode" }
func (FilterBase64URLSafeDecode) Identifier() string { return "base64UrlSafeDecode" }
func (FilterBase64URLSafeEncode) Identifier() string { return "base64UrlSafeEncode" }
func (FilterCharsetDecode) Identifier() string       { return "charsetDecode" }
func (FilterCount) Identifier() string               { return "count" }
func (FilterDaysAfterNow) Identifier() string        { return "daysAfterNow" }
func (FilterDaysBeforeNow) Identifier() string       { return "daysBeforeNow" }
func (FilterDecode) Identifier() string              { return "decode" }
func (FilterFirst) Identifier() string               { return "first" }
func (FilterFormat) Identifier() string              { return "format" }
func (FilterDateFormat) Identifier() string          { return "dateFormat" }
func (FilterHTMLEscape) Identifier() string          { return "htmlEscape" }
func (FilterHTMLUnescape) Identifier() string        { return "htmlUnescape" }
func (FilterJSONPath) Identifier() string            { return "jsonpath" }
func (FilterLast) Identifier() string                { return "last" }
func (FilterLocation) Identifier() string            { return "location" }
func (FilterNth) Identifier() string                 { return "nth" }
func (FilterRegex) Identifier() string               { return "regex" }
func (FilterReplace) Identifier() string             { return "replace" }
func (FilterReplaceRegex) Identifier() string        { return "replaceRegex" }
func (FilterSplit) Identifier() string               { return "split" }
func (FilterToDate) Identifier() string              { return "toDate" }
func (FilterToFloat) Identifier() string             { return "toFloat" }
func (FilterToHex) Identifier() string               { return "toHex" }
func (FilterToInt) Identifier() string               { return "toInt" }
func (FilterToString) Identifier() string            { return "toString" }
func (FilterURLDecode) Identifier() string           { return "urlDecode" }
func (FilterURLEncode) Identifier() string           { return "urlEncode" }
func (FilterURLQueryParam) Identifier() string       { return "urlQueryParam" }
func (FilterUTF8Decode) Identifier() string          { return "utf8Decode" }
func (FilterUTF8Encode) Identifier() string          { return "utf8Encode" }
func (FilterXPath) Identifier() string               { return "xpath" }

// IntegerValue is either an IntegerLiteral or an IntegerPlaceholder.
type IntegerValue interface {
	integerValue()
	String() string
}

type IntegerLiteral struct{ Value I64 }
type IntegerPlaceholder struct{ Value Placeholder }

func (IntegerLiteral) integerValue()     {}
func (IntegerPlaceholder) integerValue() {}

func (v IntegerLiteral) String() string     { return v.Value.String() }
func (v IntegerPlaceholder) String() string { return v.Value.String() }
