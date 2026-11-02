package ast

type Section struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	LineTerminator0 LineTerminator
	Value           SectionValue
	SourceInfo      SourceInfo
}

// Identifier returns the Hurl identifier of this section (e.g. "Asserts").
func (s Section) Identifier() string {
	switch v := s.Value.(type) {
	case SectionAsserts:
		return "Asserts"
	case SectionQueryParams:
		if v.Short {
			return "Query"
		}
		return "QueryStringParams"
	case SectionBasicAuth:
		return "BasicAuth"
	case SectionFormParams:
		if v.Short {
			return "Form"
		}
		return "FormParams"
	case SectionCookies:
		return "Cookies"
	case SectionCaptures:
		return "Captures"
	case SectionMultipartFormData:
		if v.Short {
			return "Multipart"
		}
		return "MultipartFormData"
	case SectionOptions:
		return "Options"
	}
	return ""
}

// SectionValue is one of the Section* variant types. Short reports whether
// the short section name was used ([Query], [Form], [Multipart]).
type SectionValue interface{ sectionValue() }

type SectionQueryParams struct {
	Value []KeyValue
	Short bool
}
type SectionBasicAuth struct{ Value *KeyValue }
type SectionFormParams struct {
	Value []KeyValue
	Short bool
}
type SectionMultipartFormData struct {
	Value []MultipartParam
	Short bool
}
type SectionCookies struct{ Value []Cookie }
type SectionCaptures struct{ Value []Capture }
type SectionAsserts struct{ Value []Assert }
type SectionOptions struct{ Value []EntryOption }

func (SectionQueryParams) sectionValue()       {}
func (SectionBasicAuth) sectionValue()         {}
func (SectionFormParams) sectionValue()        {}
func (SectionMultipartFormData) sectionValue() {}
func (SectionCookies) sectionValue()           {}
func (SectionCaptures) sectionValue()          {}
func (SectionAsserts) sectionValue()           {}
func (SectionOptions) sectionValue()           {}

type Cookie struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Name            Template
	Space1          Whitespace
	Space2          Whitespace
	Value           Template
	LineTerminator0 LineTerminator
}

// MultipartParam is either a MultipartParamParam or a
// MultipartParamFilenameParam.
type MultipartParam interface{ multipartParam() }

type MultipartParamParam struct{ Value KeyValue }
type MultipartParamFilenameParam struct{ Value FilenameParam }

func (MultipartParamParam) multipartParam()         {}
func (MultipartParamFilenameParam) multipartParam() {}

type FilenameParam struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Key             Template
	Space1          Whitespace
	Space2          Whitespace
	Value           FilenameValue
	LineTerminator0 LineTerminator
}

type FilenameValue struct {
	Space0      Whitespace
	Filename    Template
	Space1      Whitespace
	Space2      Whitespace
	ContentType *Template
}

type Capture struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Name            Template
	Space1          Whitespace
	Space2          Whitespace
	Query           Query
	Filters         []SpacedFilter
	Space3          Whitespace
	Redacted        bool
	LineTerminator0 LineTerminator
}

type Assert struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Query           Query
	Filters         []SpacedFilter
	Space1          Whitespace
	Predicate       Predicate
	LineTerminator0 LineTerminator
}

type Query struct {
	SourceInfo SourceInfo
	Value      QueryValue
}

// QueryValue is one of the Query* variant types.
type QueryValue interface {
	queryValue()
	// Identifier returns the Hurl identifier of this query.
	Identifier() string
}

type QueryStatus struct{}
type QueryVersion struct{}
type QueryURL struct{}
type QueryHeader struct {
	Space0 Whitespace
	Name   Template
}
type QueryCookie struct {
	Space0 Whitespace
	Expr   CookiePath
}
type QueryBody struct{}
type QueryXPath struct {
	Space0 Whitespace
	Expr   Template
}
type QueryJSONPath struct {
	Space0 Whitespace
	Expr   Template
}
type QueryRegex struct {
	Space0 Whitespace
	Value  RegexValue
}
type QueryVariable struct {
	Space0 Whitespace
	Name   Template
}
type QueryDuration struct{}
type QueryBytes struct{}
type QueryRawBytes struct{}
type QuerySHA256 struct{}
type QueryMD5 struct{}
type QueryCertificate struct {
	Space0        Whitespace
	AttributeName CertificateAttributeName
}
type QueryIP struct{}
type QueryRedirects struct{}

func (QueryStatus) queryValue()      {}
func (QueryVersion) queryValue()     {}
func (QueryURL) queryValue()         {}
func (QueryHeader) queryValue()      {}
func (QueryCookie) queryValue()      {}
func (QueryBody) queryValue()        {}
func (QueryXPath) queryValue()       {}
func (QueryJSONPath) queryValue()    {}
func (QueryRegex) queryValue()       {}
func (QueryVariable) queryValue()    {}
func (QueryDuration) queryValue()    {}
func (QueryBytes) queryValue()       {}
func (QueryRawBytes) queryValue()    {}
func (QuerySHA256) queryValue()      {}
func (QueryMD5) queryValue()         {}
func (QueryCertificate) queryValue() {}
func (QueryIP) queryValue()          {}
func (QueryRedirects) queryValue()   {}

func (QueryStatus) Identifier() string      { return "status" }
func (QueryVersion) Identifier() string     { return "version" }
func (QueryURL) Identifier() string         { return "url" }
func (QueryHeader) Identifier() string      { return "header" }
func (QueryCookie) Identifier() string      { return "cookie" }
func (QueryBody) Identifier() string        { return "body" }
func (QueryXPath) Identifier() string       { return "xpath" }
func (QueryJSONPath) Identifier() string    { return "jsonpath" }
func (QueryRegex) Identifier() string       { return "regex" }
func (QueryVariable) Identifier() string    { return "variable" }
func (QueryDuration) Identifier() string    { return "duration" }
func (QueryBytes) Identifier() string       { return "bytes" }
func (QueryRawBytes) Identifier() string    { return "rawbytes" }
func (QuerySHA256) Identifier() string      { return "sha256" }
func (QueryMD5) Identifier() string         { return "md5" }
func (QueryCertificate) Identifier() string { return "certificate" }
func (QueryIP) Identifier() string          { return "ip" }
func (QueryRedirects) Identifier() string   { return "redirects" }

// RegexValue is either a RegexTemplate (a quoted string) or a RegexRegex
// (a /literal/).
type RegexValue interface{ regexValue() }

type RegexTemplate struct{ Value Template }
type RegexRegex struct{ Value Regex }

func (RegexTemplate) regexValue() {}
func (RegexRegex) regexValue()    {}

type CookiePath struct {
	Name      Template
	Attribute *CookieAttribute
}

func (c CookiePath) String() string {
	s := c.Name.String()
	if c.Attribute != nil {
		s += "[" + c.Attribute.identifier() + "]"
	}
	return s
}

func (c CookiePath) ToSource() string {
	s := `"` + c.Name.ToSource()
	if c.Attribute != nil {
		s += "[" + c.Attribute.Name.Value + "]"
	}
	return s + `"`
}

type CookieAttribute struct {
	Space0 Whitespace
	Name   CookieAttributeName
	Space1 Whitespace
}

func (c CookieAttribute) identifier() string {
	switch c.Name.Kind {
	case CookieAttributeMaxAge:
		return "Max-Age"
	case CookieAttributeValue:
		return "Value"
	case CookieAttributeExpires:
		return "Expires"
	case CookieAttributeDomain:
		return "Domain"
	case CookieAttributePath:
		return "Path"
	case CookieAttributeSecure:
		return "Secure"
	case CookieAttributeHTTPOnly:
		return "HttpOnly"
	case CookieAttributeSameSite:
		return "SameSite"
	}
	return ""
}

// CookieAttributeName is a cookie attribute: its Kind and its spelling in
// the source (Value), e.g. {CookieAttributeMaxAge, "max-age"}.
type CookieAttributeName struct {
	Kind  CookieAttributeKind
	Value string
}

type CookieAttributeKind int

const (
	CookieAttributeValue CookieAttributeKind = iota
	CookieAttributeExpires
	CookieAttributeMaxAge
	CookieAttributeDomain
	CookieAttributePath
	CookieAttributeSecure
	CookieAttributeHTTPOnly
	CookieAttributeSameSite
)

type CertificateAttributeName int

const (
	CertificateAttributeSubject CertificateAttributeName = iota
	CertificateAttributeIssuer
	CertificateAttributeStartDate
	CertificateAttributeExpireDate
	CertificateAttributeSerialNumber
	CertificateAttributeSubjectAltName
	CertificateAttributeValue
)

// Identifier returns the Hurl identifier of this certificate attribute.
func (c CertificateAttributeName) Identifier() string {
	switch c {
	case CertificateAttributeSubject:
		return "Subject"
	case CertificateAttributeIssuer:
		return "Issuer"
	case CertificateAttributeStartDate:
		return "Start-Date"
	case CertificateAttributeExpireDate:
		return "Expire-Date"
	case CertificateAttributeSerialNumber:
		return "Serial-Number"
	case CertificateAttributeSubjectAltName:
		return "Subject-Alt-Name"
	case CertificateAttributeValue:
		return "Value"
	}
	return ""
}

func (c CertificateAttributeName) ToSource() string { return `"` + c.Identifier() + `"` }

type Predicate struct {
	Not           bool
	Space0        Whitespace
	PredicateFunc PredicateFunc
}

type Not struct {
	Value  bool
	Space0 Whitespace
}

type PredicateFunc struct {
	SourceInfo SourceInfo
	Value      PredicateFuncValue
}

// PredicateValue is one of the Predicate* value variant types
// (PredicateBase64 ... PredicateString).
type PredicateValue interface{ predicateValue() }

type PredicateBase64 struct{ Value Base64 }
type PredicateBool struct{ Value bool }
type PredicateFile struct{ Value File }
type PredicateHex struct{ Value Hex }
type PredicateMultilineString struct{ Value MultilineString }
type PredicateNull struct{}
type PredicateNumber struct{ Value Number }
type PredicatePlaceholder struct{ Value Placeholder }
type PredicateRegex struct{ Value Regex }
type PredicateString struct{ Value Template }

func (PredicateBase64) predicateValue()          {}
func (PredicateBool) predicateValue()            {}
func (PredicateFile) predicateValue()            {}
func (PredicateHex) predicateValue()             {}
func (PredicateMultilineString) predicateValue() {}
func (PredicateNull) predicateValue()            {}
func (PredicateNumber) predicateValue()          {}
func (PredicatePlaceholder) predicateValue()     {}
func (PredicateRegex) predicateValue()           {}
func (PredicateString) predicateValue()          {}

// PredicateFuncValue is one of the PredicateFunc* variant types.
type PredicateFuncValue interface {
	predicateFuncValue()
	// Identifier returns the Hurl identifier of this predicate.
	Identifier() string
	ToSource() string
}

type PredicateFuncEqual struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncNotEqual struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncGreaterThan struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncGreaterThanOrEqual struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncLessThan struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncLessThanOrEqual struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncStartWith struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncEndWith struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncContain struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncInclude struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncMatch struct {
	Space0 Whitespace
	Value  PredicateValue
}
type PredicateFuncExist struct{}
type PredicateFuncIsBoolean struct{}
type PredicateFuncIsCollection struct{}
type PredicateFuncIsDate struct{}
type PredicateFuncIsEmpty struct{}
type PredicateFuncIsFloat struct{}
type PredicateFuncIsInteger struct{}
type PredicateFuncIsIPv4 struct{}
type PredicateFuncIsIPv6 struct{}
type PredicateFuncIsISODate struct{}
type PredicateFuncIsList struct{}
type PredicateFuncIsNumber struct{}
type PredicateFuncIsObject struct{}
type PredicateFuncIsString struct{}
type PredicateFuncIsUUID struct{}

func (PredicateFuncEqual) predicateFuncValue()              {}
func (PredicateFuncNotEqual) predicateFuncValue()           {}
func (PredicateFuncGreaterThan) predicateFuncValue()        {}
func (PredicateFuncGreaterThanOrEqual) predicateFuncValue() {}
func (PredicateFuncLessThan) predicateFuncValue()           {}
func (PredicateFuncLessThanOrEqual) predicateFuncValue()    {}
func (PredicateFuncStartWith) predicateFuncValue()          {}
func (PredicateFuncEndWith) predicateFuncValue()            {}
func (PredicateFuncContain) predicateFuncValue()            {}
func (PredicateFuncInclude) predicateFuncValue()            {}
func (PredicateFuncMatch) predicateFuncValue()              {}
func (PredicateFuncExist) predicateFuncValue()              {}
func (PredicateFuncIsBoolean) predicateFuncValue()          {}
func (PredicateFuncIsCollection) predicateFuncValue()       {}
func (PredicateFuncIsDate) predicateFuncValue()             {}
func (PredicateFuncIsEmpty) predicateFuncValue()            {}
func (PredicateFuncIsFloat) predicateFuncValue()            {}
func (PredicateFuncIsInteger) predicateFuncValue()          {}
func (PredicateFuncIsIPv4) predicateFuncValue()             {}
func (PredicateFuncIsIPv6) predicateFuncValue()             {}
func (PredicateFuncIsISODate) predicateFuncValue()          {}
func (PredicateFuncIsList) predicateFuncValue()             {}
func (PredicateFuncIsNumber) predicateFuncValue()           {}
func (PredicateFuncIsObject) predicateFuncValue()           {}
func (PredicateFuncIsString) predicateFuncValue()           {}
func (PredicateFuncIsUUID) predicateFuncValue()             {}

func (PredicateFuncEqual) Identifier() string              { return "==" }
func (PredicateFuncNotEqual) Identifier() string           { return "!=" }
func (PredicateFuncGreaterThan) Identifier() string        { return ">" }
func (PredicateFuncGreaterThanOrEqual) Identifier() string { return ">=" }
func (PredicateFuncLessThan) Identifier() string           { return "<" }
func (PredicateFuncLessThanOrEqual) Identifier() string    { return "<=" }
func (PredicateFuncStartWith) Identifier() string          { return "startsWith" }
func (PredicateFuncEndWith) Identifier() string            { return "endsWith" }
func (PredicateFuncContain) Identifier() string            { return "contains" }
func (PredicateFuncInclude) Identifier() string            { return "includes" }
func (PredicateFuncMatch) Identifier() string              { return "matches" }
func (PredicateFuncExist) Identifier() string              { return "exists" }
func (PredicateFuncIsBoolean) Identifier() string          { return "isBoolean" }
func (PredicateFuncIsCollection) Identifier() string       { return "isCollection" }
func (PredicateFuncIsDate) Identifier() string             { return "isDate" }
func (PredicateFuncIsEmpty) Identifier() string            { return "isEmpty" }
func (PredicateFuncIsFloat) Identifier() string            { return "isFloat" }
func (PredicateFuncIsInteger) Identifier() string          { return "isInteger" }
func (PredicateFuncIsIPv4) Identifier() string             { return "isIpv4" }
func (PredicateFuncIsIPv6) Identifier() string             { return "isIpv6" }
func (PredicateFuncIsISODate) Identifier() string          { return "isIsoDate" }
func (PredicateFuncIsList) Identifier() string             { return "isList" }
func (PredicateFuncIsNumber) Identifier() string           { return "isNumber" }
func (PredicateFuncIsObject) Identifier() string           { return "isObject" }
func (PredicateFuncIsString) Identifier() string           { return "isString" }
func (PredicateFuncIsUUID) Identifier() string             { return "isUuid" }

func (p PredicateFuncEqual) ToSource() string              { return p.Identifier() }
func (p PredicateFuncNotEqual) ToSource() string           { return p.Identifier() }
func (p PredicateFuncGreaterThan) ToSource() string        { return p.Identifier() }
func (p PredicateFuncGreaterThanOrEqual) ToSource() string { return p.Identifier() }
func (p PredicateFuncLessThan) ToSource() string           { return p.Identifier() }
func (p PredicateFuncLessThanOrEqual) ToSource() string    { return p.Identifier() }
func (p PredicateFuncStartWith) ToSource() string          { return p.Identifier() }
func (p PredicateFuncEndWith) ToSource() string            { return p.Identifier() }
func (p PredicateFuncContain) ToSource() string            { return p.Identifier() }
func (p PredicateFuncInclude) ToSource() string            { return p.Identifier() }
func (p PredicateFuncMatch) ToSource() string              { return p.Identifier() }
func (p PredicateFuncExist) ToSource() string              { return p.Identifier() }
func (p PredicateFuncIsBoolean) ToSource() string          { return p.Identifier() }
func (p PredicateFuncIsCollection) ToSource() string       { return p.Identifier() }
func (p PredicateFuncIsDate) ToSource() string             { return p.Identifier() }
func (p PredicateFuncIsEmpty) ToSource() string            { return p.Identifier() }
func (p PredicateFuncIsFloat) ToSource() string            { return p.Identifier() }
func (p PredicateFuncIsInteger) ToSource() string          { return p.Identifier() }
func (p PredicateFuncIsIPv4) ToSource() string             { return p.Identifier() }
func (p PredicateFuncIsIPv6) ToSource() string             { return p.Identifier() }
func (p PredicateFuncIsISODate) ToSource() string          { return p.Identifier() }
func (p PredicateFuncIsList) ToSource() string             { return p.Identifier() }
func (p PredicateFuncIsNumber) ToSource() string           { return p.Identifier() }
func (p PredicateFuncIsObject) ToSource() string           { return p.Identifier() }
func (p PredicateFuncIsString) ToSource() string           { return p.Identifier() }
func (p PredicateFuncIsUUID) ToSource() string             { return p.Identifier() }
