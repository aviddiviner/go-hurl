package ast

type EntryOption struct {
	LineTerminators []LineTerminator
	Space0          Whitespace
	Space1          Whitespace
	Space2          Whitespace
	Kind            OptionKind
	LineTerminator0 LineTerminator
}

// OptionKind is one of the Option* variant types. String renders
// "identifier: value".
type OptionKind interface {
	optionKind()
	// Identifier returns the Hurl identifier of this option.
	Identifier() string
	String() string
}

// BooleanOption is either a BooleanLiteral or a BooleanPlaceholder.
type BooleanOption interface {
	booleanOption()
	String() string
}

type BooleanLiteral struct{ Value bool }
type BooleanPlaceholder struct{ Value Placeholder }

func (BooleanLiteral) booleanOption()     {}
func (BooleanPlaceholder) booleanOption() {}

func (o BooleanLiteral) String() string {
	if o.Value {
		return "true"
	}
	return "false"
}
func (o BooleanPlaceholder) String() string { return o.Value.String() }

// NaturalOption is either a NaturalLiteral or a NaturalPlaceholder.
type NaturalOption interface {
	naturalOption()
	String() string
}

type NaturalLiteral struct{ Value U64 }
type NaturalPlaceholder struct{ Value Placeholder }

func (NaturalLiteral) naturalOption()     {}
func (NaturalPlaceholder) naturalOption() {}

func (o NaturalLiteral) String() string     { return o.Value.String() }
func (o NaturalPlaceholder) String() string { return o.Value.String() }

// CountOption is either a CountLiteral or a CountPlaceholder.
type CountOption interface {
	countOption()
	String() string
}

type CountLiteral struct{ Value Count }
type CountPlaceholder struct{ Value Placeholder }

func (CountLiteral) countOption()     {}
func (CountPlaceholder) countOption() {}

func (o CountLiteral) String() string     { return o.Value.String() }
func (o CountPlaceholder) String() string { return o.Value.String() }

// DurationOption is either a DurationLiteral or a DurationPlaceholder.
type DurationOption interface {
	durationOption()
	String() string
}

type DurationLiteral struct{ Value Duration }
type DurationPlaceholder struct{ Value Placeholder }

func (DurationLiteral) durationOption()     {}
func (DurationPlaceholder) durationOption() {}

func (o DurationLiteral) String() string     { return o.Value.String() }
func (o DurationPlaceholder) String() string { return o.Value.String() }

type Duration struct {
	Value U64
	Unit  *DurationUnit
}

func (d Duration) String() string {
	unit := ""
	if d.Unit != nil {
		unit = d.Unit.String()
	}
	return d.Value.String() + unit
}

type VariableDefinition struct {
	SourceInfo SourceInfo
	Name       string
	Space0     Whitespace
	Space1     Whitespace
	Value      VariableValue
}

func (v VariableDefinition) String() string { return v.Name + "=" + v.Value.String() }

// VariableValue is one of VariableNull, VariableBool, VariableNumber or
// VariableString.
type VariableValue interface {
	variableValue()
	String() string
	ToSource() string
}

type VariableNull struct{}
type VariableBool struct{ Value bool }
type VariableNumber struct{ Value Number }
type VariableString struct{ Value Template }

func (VariableNull) variableValue()   {}
func (VariableBool) variableValue()   {}
func (VariableNumber) variableValue() {}
func (VariableString) variableValue() {}

func (VariableNull) String() string { return "null" }
func (v VariableBool) String() string {
	if v.Value {
		return "true"
	}
	return "false"
}
func (v VariableNumber) String() string { return v.Value.String() }
func (v VariableString) String() string { return v.Value.String() }

func (VariableNull) ToSource() string     { return "null" }
func (v VariableBool) ToSource() string   { return v.String() }
func (v VariableNumber) ToSource() string { return v.Value.ToSource() }
func (v VariableString) ToSource() string { return v.Value.ToSource() }

type VerbosityOption int

const (
	VerbosityBrief VerbosityOption = iota
	VerbosityVerbose
	VerbosityDebug
)

func (v VerbosityOption) String() string { return v.Identifier() }

func (v VerbosityOption) Identifier() string {
	switch v {
	case VerbosityBrief:
		return "brief"
	case VerbosityVerbose:
		return "verbose"
	case VerbosityDebug:
		return "debug"
	}
	return ""
}

type OptionAWSSigV4 struct{ Value Template }
type OptionCACertificate struct{ Value Template }
type OptionClientCert struct{ Value Template }
type OptionClientKey struct{ Value Template }
type OptionCompressed struct{ Value BooleanOption }
type OptionConnectTo struct{ Value Template }
type OptionConnectTimeout struct{ Value DurationOption }
type OptionDelay struct{ Value DurationOption }
type OptionDigest struct{ Value BooleanOption }
type OptionFailWithBody struct{ Value BooleanOption }
type OptionFollowLocation struct{ Value BooleanOption }
type OptionFollowLocationTrusted struct{ Value BooleanOption }
type OptionHeader struct{ Value Template }
type OptionHTTP10 struct{ Value BooleanOption }
type OptionHTTP11 struct{ Value BooleanOption }
type OptionHTTP2 struct{ Value BooleanOption }
type OptionHTTP2PriorKnowledge struct{ Value BooleanOption }
type OptionHTTP3 struct{ Value BooleanOption }
type OptionInsecure struct{ Value BooleanOption }
type OptionIPv4 struct{ Value BooleanOption }
type OptionIPv6 struct{ Value BooleanOption }
type OptionLimitRate struct{ Value NaturalOption }
type OptionMaxRedirect struct{ Value CountOption }
type OptionMaxTime struct{ Value DurationOption }
type OptionNegotiate struct{ Value BooleanOption }
type OptionNetRc struct{ Value BooleanOption }
type OptionNetRcFile struct{ Value Template }
type OptionNetRcOptional struct{ Value BooleanOption }
type OptionNoHeader struct{ Value Template }
type OptionNoJSONPathCoercion struct{ Value BooleanOption }
type OptionNtlm struct{ Value BooleanOption }
type OptionOutput struct{ Value Template }
type OptionPathAsIs struct{ Value BooleanOption }
type OptionPinnedPublicKey struct{ Value Template }
type OptionProxy struct{ Value Template }
type OptionRepeat struct{ Value CountOption }
type OptionResolve struct{ Value Template }
type OptionRetry struct{ Value CountOption }
type OptionRetryInterval struct{ Value DurationOption }
type OptionSkip struct{ Value BooleanOption }
type OptionTLSMax struct{ Value Template }
type OptionTLSMin struct{ Value Template }
type OptionUnixSocket struct{ Value Template }
type OptionUser struct{ Value Template }
type OptionVariable struct{ Value VariableDefinition }
type OptionVerbose struct{ Value BooleanOption }
type OptionVerbosity struct{ Value VerbosityOption }
type OptionVeryVerbose struct{ Value BooleanOption }

func (OptionAWSSigV4) optionKind()              {}
func (OptionCACertificate) optionKind()         {}
func (OptionClientCert) optionKind()            {}
func (OptionClientKey) optionKind()             {}
func (OptionCompressed) optionKind()            {}
func (OptionConnectTo) optionKind()             {}
func (OptionConnectTimeout) optionKind()        {}
func (OptionDelay) optionKind()                 {}
func (OptionDigest) optionKind()                {}
func (OptionFailWithBody) optionKind()          {}
func (OptionFollowLocation) optionKind()        {}
func (OptionFollowLocationTrusted) optionKind() {}
func (OptionHeader) optionKind()                {}
func (OptionHTTP10) optionKind()                {}
func (OptionHTTP11) optionKind()                {}
func (OptionHTTP2) optionKind()                 {}
func (OptionHTTP2PriorKnowledge) optionKind()   {}
func (OptionHTTP3) optionKind()                 {}
func (OptionInsecure) optionKind()              {}
func (OptionIPv4) optionKind()                  {}
func (OptionIPv6) optionKind()                  {}
func (OptionLimitRate) optionKind()             {}
func (OptionMaxRedirect) optionKind()           {}
func (OptionMaxTime) optionKind()               {}
func (OptionNegotiate) optionKind()             {}
func (OptionNetRc) optionKind()                 {}
func (OptionNetRcFile) optionKind()             {}
func (OptionNetRcOptional) optionKind()         {}
func (OptionNoHeader) optionKind()              {}
func (OptionNoJSONPathCoercion) optionKind()    {}
func (OptionNtlm) optionKind()                  {}
func (OptionOutput) optionKind()                {}
func (OptionPathAsIs) optionKind()              {}
func (OptionPinnedPublicKey) optionKind()       {}
func (OptionProxy) optionKind()                 {}
func (OptionRepeat) optionKind()                {}
func (OptionResolve) optionKind()               {}
func (OptionRetry) optionKind()                 {}
func (OptionRetryInterval) optionKind()         {}
func (OptionSkip) optionKind()                  {}
func (OptionTLSMax) optionKind()                {}
func (OptionTLSMin) optionKind()                {}
func (OptionUnixSocket) optionKind()            {}
func (OptionUser) optionKind()                  {}
func (OptionVariable) optionKind()              {}
func (OptionVerbose) optionKind()               {}
func (OptionVerbosity) optionKind()             {}
func (OptionVeryVerbose) optionKind()           {}

func (OptionAWSSigV4) Identifier() string              { return "aws-sigv4" }
func (OptionCACertificate) Identifier() string         { return "cacert" }
func (OptionClientCert) Identifier() string            { return "cert" }
func (OptionClientKey) Identifier() string             { return "key" }
func (OptionCompressed) Identifier() string            { return "compressed" }
func (OptionConnectTo) Identifier() string             { return "connect-to" }
func (OptionConnectTimeout) Identifier() string        { return "connect-timeout" }
func (OptionDelay) Identifier() string                 { return "delay" }
func (OptionDigest) Identifier() string                { return "digest" }
func (OptionFailWithBody) Identifier() string          { return "fail-with-body" }
func (OptionFollowLocation) Identifier() string        { return "location" }
func (OptionFollowLocationTrusted) Identifier() string { return "location-trusted" }
func (OptionHeader) Identifier() string                { return "header" }
func (OptionHTTP10) Identifier() string                { return "http1.0" }
func (OptionHTTP11) Identifier() string                { return "http1.1" }
func (OptionHTTP2) Identifier() string                 { return "http2" }
func (OptionHTTP2PriorKnowledge) Identifier() string   { return "http2-prior-knowledge" }
func (OptionHTTP3) Identifier() string                 { return "http3" }
func (OptionInsecure) Identifier() string              { return "insecure" }
func (OptionIPv4) Identifier() string                  { return "ipv4" }
func (OptionIPv6) Identifier() string                  { return "ipv6" }
func (OptionLimitRate) Identifier() string             { return "limit-rate" }
func (OptionMaxRedirect) Identifier() string           { return "max-redirs" }
func (OptionMaxTime) Identifier() string               { return "max-time" }
func (OptionNegotiate) Identifier() string             { return "negotiate" }
func (OptionNetRc) Identifier() string                 { return "netrc" }
func (OptionNetRcFile) Identifier() string             { return "netrc-file" }
func (OptionNetRcOptional) Identifier() string         { return "netrc-optional" }
func (OptionNoHeader) Identifier() string              { return "no-header" }
func (OptionNoJSONPathCoercion) Identifier() string    { return "no-jsonpath-coercion" }
func (OptionNtlm) Identifier() string                  { return "ntlm" }
func (OptionOutput) Identifier() string                { return "output" }
func (OptionPathAsIs) Identifier() string              { return "path-as-is" }
func (OptionPinnedPublicKey) Identifier() string       { return "pinnedpubkey" }
func (OptionProxy) Identifier() string                 { return "proxy" }
func (OptionRepeat) Identifier() string                { return "repeat" }
func (OptionResolve) Identifier() string               { return "resolve" }
func (OptionRetry) Identifier() string                 { return "retry" }
func (OptionRetryInterval) Identifier() string         { return "retry-interval" }
func (OptionSkip) Identifier() string                  { return "skip" }
func (OptionTLSMax) Identifier() string                { return "tls-max" }
func (OptionTLSMin) Identifier() string                { return "tls-min" }
func (OptionUnixSocket) Identifier() string            { return "unix-socket" }
func (OptionUser) Identifier() string                  { return "user" }
func (OptionVariable) Identifier() string              { return "variable" }
func (OptionVerbose) Identifier() string               { return "verbose" }
func (OptionVerbosity) Identifier() string             { return "verbosity" }
func (OptionVeryVerbose) Identifier() string           { return "very-verbose" }

func (o OptionAWSSigV4) String() string              { return o.Identifier() + ": " + o.Value.String() }
func (o OptionCACertificate) String() string         { return o.Identifier() + ": " + o.Value.String() }
func (o OptionClientCert) String() string            { return o.Identifier() + ": " + o.Value.String() }
func (o OptionClientKey) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionCompressed) String() string            { return o.Identifier() + ": " + o.Value.String() }
func (o OptionConnectTo) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionConnectTimeout) String() string        { return o.Identifier() + ": " + o.Value.String() }
func (o OptionDelay) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionDigest) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionFailWithBody) String() string          { return o.Identifier() + ": " + o.Value.String() }
func (o OptionFollowLocation) String() string        { return o.Identifier() + ": " + o.Value.String() }
func (o OptionFollowLocationTrusted) String() string { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHeader) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHTTP10) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHTTP11) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHTTP2) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHTTP2PriorKnowledge) String() string   { return o.Identifier() + ": " + o.Value.String() }
func (o OptionHTTP3) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionInsecure) String() string              { return o.Identifier() + ": " + o.Value.String() }
func (o OptionIPv4) String() string                  { return o.Identifier() + ": " + o.Value.String() }
func (o OptionIPv6) String() string                  { return o.Identifier() + ": " + o.Value.String() }
func (o OptionLimitRate) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionMaxRedirect) String() string           { return o.Identifier() + ": " + o.Value.String() }
func (o OptionMaxTime) String() string               { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNegotiate) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNetRc) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNetRcFile) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNetRcOptional) String() string         { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNoHeader) String() string              { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNoJSONPathCoercion) String() string    { return o.Identifier() + ": " + o.Value.String() }
func (o OptionNtlm) String() string                  { return o.Identifier() + ": " + o.Value.String() }
func (o OptionOutput) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionPathAsIs) String() string              { return o.Identifier() + ": " + o.Value.String() }
func (o OptionPinnedPublicKey) String() string       { return o.Identifier() + ": " + o.Value.String() }
func (o OptionProxy) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionRepeat) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionResolve) String() string               { return o.Identifier() + ": " + o.Value.String() }
func (o OptionRetry) String() string                 { return o.Identifier() + ": " + o.Value.String() }
func (o OptionRetryInterval) String() string         { return o.Identifier() + ": " + o.Value.String() }
func (o OptionSkip) String() string                  { return o.Identifier() + ": " + o.Value.String() }
func (o OptionTLSMax) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionTLSMin) String() string                { return o.Identifier() + ": " + o.Value.String() }
func (o OptionUnixSocket) String() string            { return o.Identifier() + ": " + o.Value.String() }
func (o OptionUser) String() string                  { return o.Identifier() + ": " + o.Value.String() }
func (o OptionVariable) String() string              { return o.Identifier() + ": " + o.Value.String() }
func (o OptionVerbose) String() string               { return o.Identifier() + ": " + o.Value.String() }
func (o OptionVerbosity) String() string             { return o.Identifier() + ": " + o.Value.String() }
func (o OptionVeryVerbose) String() string           { return o.Identifier() + ": " + o.Value.String() }
