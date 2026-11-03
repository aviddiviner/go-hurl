package runner

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/internal/timefmt"
)

// Value is a Hurl value, used by queries, captures, asserts and predicates.
// It is one of the Value* variant types.
type Value interface {
	value()
	// Kind returns the type of this value.
	Kind() ValueKind
	String() string
}

type ValueBool struct{ Value bool }
type ValueBytes struct{ Value []byte }

// ValueDate is a date, always in UTC.
type ValueDate struct{ Value time.Time }
type ValueHTTPResponse struct{ Value HTTPResponse }
type ValueList struct{ Value []Value }

// ValueNodeset is the size of an XPath node-set.
type ValueNodeset struct{ Value int }
type ValueNull struct{}
type ValueNumber struct{ Value Number }

// ValueObject is an object (from JSONPath queries), with its members in
// order.
type ValueObject struct{ Value []KeyValue }
type ValueRegex struct{ Value *regexp.Regexp }
type ValueString struct{ Value string }
type ValueUnit struct{}

// KeyValue is a member of a ValueObject.
type KeyValue struct {
	Key   string
	Value Value
}

func (ValueBool) value()         {}
func (ValueBytes) value()        {}
func (ValueDate) value()         {}
func (ValueHTTPResponse) value() {}
func (ValueList) value()         {}
func (ValueNodeset) value()      {}
func (ValueNull) value()         {}
func (ValueNumber) value()       {}
func (ValueObject) value()       {}
func (ValueRegex) value()        {}
func (ValueString) value()       {}
func (ValueUnit) value()         {}

func NewBool(v bool) Value         { return ValueBool{v} }
func NewString(s string) Value     { return ValueString{s} }
func NewInteger(i int64) Value     { return ValueNumber{NumberInteger{i}} }
func NewFloat(f float64) Value     { return ValueNumber{NumberFloat{f}} }
func NewBigInteger(s string) Value { return ValueNumber{NumberBigInteger{s}} }

// ValueKind is the type of a Value.
type ValueKind int

const (
	ValueKindBool ValueKind = iota
	ValueKindBytes
	ValueKindDate
	ValueKindFloat
	ValueKindHTTPResponse
	ValueKindInteger
	ValueKindList
	ValueKindNodeset
	ValueKindNull
	ValueKindObject
	ValueKindRegex
	ValueKindSecret
	ValueKindString
	ValueKindUnit
)

func (k ValueKind) String() string {
	switch k {
	case ValueKindBool:
		return "boolean"
	case ValueKindBytes:
		return "bytes"
	case ValueKindDate:
		return "date"
	case ValueKindFloat:
		return "float"
	case ValueKindHTTPResponse:
		return "http response"
	case ValueKindInteger:
		return "integer"
	case ValueKindList:
		return "list"
	case ValueKindNodeset:
		return "nodeset"
	case ValueKindNull:
		return "null"
	case ValueKindObject:
		return "object"
	case ValueKindRegex:
		return "regex"
	case ValueKindSecret:
		return "secret"
	case ValueKindString:
		return "string"
	case ValueKindUnit:
		return "unit"
	}
	return ""
}

func (ValueBool) Kind() ValueKind         { return ValueKindBool }
func (ValueBytes) Kind() ValueKind        { return ValueKindBytes }
func (ValueDate) Kind() ValueKind         { return ValueKindDate }
func (ValueHTTPResponse) Kind() ValueKind { return ValueKindHTTPResponse }
func (ValueList) Kind() ValueKind         { return ValueKindList }
func (ValueNodeset) Kind() ValueKind      { return ValueKindNodeset }
func (ValueNull) Kind() ValueKind         { return ValueKindNull }
func (ValueObject) Kind() ValueKind       { return ValueKindObject }
func (ValueRegex) Kind() ValueKind        { return ValueKindRegex }
func (ValueString) Kind() ValueKind       { return ValueKindString }
func (ValueUnit) Kind() ValueKind         { return ValueKindUnit }

func (v ValueNumber) Kind() ValueKind {
	if _, ok := v.Value.(NumberFloat); ok {
		return ValueKindFloat
	}
	return ValueKindInteger
}

// EvalError is an error of the Value operations used by predicates.
type EvalError int

const (
	EvalErrorType EvalError = iota
	EvalErrorInvalidRegex
)

func (e EvalError) Error() string {
	if e == EvalErrorInvalidRegex {
		return "invalid regex"
	}
	return "type error"
}

// Equal is the equality of values used by the == predicate. Any combination
// of value types can be compared; numbers compare across Integer, Float and
// BigInteger.
func Equal(a, b Value) bool {
	switch a := a.(type) {
	case ValueBool:
		b, ok := b.(ValueBool)
		return ok && a.Value == b.Value
	case ValueBytes:
		b, ok := b.(ValueBytes)
		return ok && slices.Equal(a.Value, b.Value)
	case ValueDate:
		b, ok := b.(ValueDate)
		return ok && a.Value.Equal(b.Value)
	case ValueNumber:
		b, ok := b.(ValueNumber)
		return ok && cmpNumber(a.Value, b.Value) == 0
	case ValueList:
		b, ok := b.(ValueList)
		return ok && slices.EqualFunc(a.Value, b.Value, Equal)
	case ValueNodeset:
		b, ok := b.(ValueNodeset)
		return ok && a.Value == b.Value
	case ValueNull:
		_, ok := b.(ValueNull)
		return ok
	case ValueObject:
		b, ok := b.(ValueObject)
		return ok && slices.EqualFunc(a.Value, b.Value, func(x, y KeyValue) bool {
			return x.Key == y.Key && Equal(x.Value, y.Value)
		})
	case ValueString:
		b, ok := b.(ValueString)
		return ok && a.Value == b.Value
	case ValueUnit:
		_, ok := b.(ValueUnit)
		return ok
	}
	return false
}

func (v ValueBool) String() string         { return strconv.FormatBool(v.Value) }
func (v ValueBytes) String() string        { return hexEncode(v.Value) }
func (v ValueDate) String() string         { return timefmt.Display(v.Value) }
func (v ValueHTTPResponse) String() string { return v.Value.String() }
func (v ValueNumber) String() string       { return v.Value.String() }
func (v ValueNodeset) String() string      { return "Nodeset(size=" + strconv.Itoa(v.Value) + ")" }
func (ValueNull) String() string           { return "null" }
func (ValueObject) String() string         { return "Object()" }
func (v ValueRegex) String() string {
	return "/" + strings.ReplaceAll(v.Value.String(), "/", "\\/") + "/"
}
func (v ValueString) String() string { return v.Value }
func (ValueUnit) String() string     { return "Unit" }

func (v ValueList) String() string {
	values := make([]string, len(v.Value))
	for i, e := range v.Value {
		values[i] = e.String()
	}
	return "[" + strings.Join(values, ",") + "]"
}

const formatISO = "%Y-%m-%dT%H:%M:%S.%6fZ"

// Repr returns a printable representation of v including its type, e.g.
// "integer <1>".
func Repr(v Value) string {
	switch k := v.Kind(); k {
	case ValueKindUnit, ValueKindSecret:
		return k.String()
	default:
		return k.String() + " <" + v.String() + ">"
	}
}

// IsScalar reports whether v is neither a node-set nor a list.
func IsScalar(v Value) bool {
	switch v.(type) {
	case ValueNodeset, ValueList:
		return false
	}
	return true
}

// Render renders v as a string in templates. It returns false for values
// that can't be rendered (bytes, lists, objects...).
func Render(v Value) (string, bool) {
	switch v := v.(type) {
	case ValueBool:
		return v.String(), true
	case ValueDate:
		s, _ := timefmt.Format(v.Value, formatISO)
		return s, true
	case ValueNull:
		return "null", true
	case ValueNumber:
		return v.String(), true
	case ValueString:
		return v.Value, true
	}
	return "", false
}
