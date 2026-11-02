package jsonpath

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Queries and segments

// filterQuery is a query inside a filter: a relativeQuery (@) or an absolute *Query ($).
type filterQuery interface {
	evalFilter(current, root any) NodeList
}

type relativeQuery struct {
	segments []segment
}

type segment interface {
	eval(current, root any) NodeList
}

type childSegment struct {
	selectors []selector
}

type descendantSegment struct {
	selectors []selector
}

// Selectors

type selector interface {
	eval(current, root any) NodeList
}

type nameSelector struct {
	value string
}

type wildcardSelector struct{}

type indexSelector struct {
	value int64
}

type arraySliceSelector struct {
	start *int64
	end   *int64
	step  int64
}

type filterSelector struct {
	expr logicalExpr
}

// Singular queries

type singularQuery interface {
	valueType
	regexArgument
}

type absoluteSingularQuery struct {
	segments []singularQuerySegment
}

type relativeSingularQuery struct {
	segments []singularQuerySegment
}

// singularQuerySegment is a nameSelector or an indexSelector.
type singularQuerySegment interface {
	evalOne(v any) (any, bool)
}

// Logical expressions

type logicalExpr interface {
	eval(current, root any) bool
}

type comparisonExpr struct {
	left     valueType
	right    valueType
	operator comparisonOp
}

type comparisonOp int

const (
	opEqual comparisonOp = iota
	opNotEqual
	opLessOrEqual
	opLess
	opGreaterOrEqual
	opGreater
)

type testExpr struct {
	not  bool
	kind testExprKind
}

type testExprKind interface {
	test(current, root any) bool
}

type testFilterQuery struct {
	query filterQuery
}

type testFunction struct {
	function logicalTypeFunction
}

type andExpr struct {
	operands []logicalExpr
}

type orExpr struct {
	operands []logicalExpr
}

type notExpr struct {
	expr logicalExpr
}

// Values: comparables and ValueType function arguments.

// valueType evaluates to a JSON value, or to nothing (false).
// It is a literal, a singularQuery or a valueTypeFunction.
type valueType interface {
	eval(current, root any) (any, bool)
}

// literal holds nil, a bool, a string or a number.
type literal struct {
	value any
}

type number interface {
	jsonNumber() json.Number
}

type (
	integerNumber    int64
	bigIntegerNumber string
	floatNumber      float64
)

func (n integerNumber) jsonNumber() json.Number    { return json.Number(strconv.FormatInt(int64(n), 10)) }
func (n bigIntegerNumber) jsonNumber() json.Number { return json.Number(n) }

// jsonNumber formats the float like serde_json with arbitrary_precision (ryu).
func (n floatNumber) jsonNumber() json.Number {
	f := float64(n)
	sign := ""
	if math.Signbit(f) {
		sign = "-"
	}
	if f == 0 {
		return json.Number(sign + "0.0")
	}
	mantissa, exp, _ := strings.Cut(strconv.FormatFloat(math.Abs(f), 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	e, _ := strconv.Atoi(exp)
	length := len(digits)
	kk := e + 1 // 10^(kk-1) <= |f| < 10^kk
	k := kk - length
	var s string
	switch {
	case 0 <= k && kk <= 16:
		s = digits + strings.Repeat("0", k) + ".0"
	case 0 < kk && kk <= 16:
		s = digits[:kk] + "." + digits[kk:]
	case -5 < kk && kk <= 0:
		s = "0." + strings.Repeat("0", -kk) + digits
	case length == 1:
		s = digits + "e" + strconv.Itoa(kk-1)
	default:
		s = digits[:1] + "." + digits[1:] + "e" + strconv.Itoa(kk-1)
	}
	return json.Number(sign + s)
}

// Functions

type valueTypeFunction interface {
	valueType
	regexArgument
}

type lengthFunction struct {
	arg valueType
}

type countFunction struct {
	arg filterQuery
}

type valueFunction struct {
	arg filterQuery
}

type logicalTypeFunction interface {
	eval(current, root any) bool
}

type matchFunction struct {
	arg     valueType
	pattern regexArgument
}

type searchFunction struct {
	arg     valueType
	pattern regexArgument
}

// regexArgument is a regexLiteral, a singularQuery or a valueTypeFunction.
type regexArgument interface {
	evalRegex(current, root any) (*regexp.Regexp, bool)
}

type regexLiteral struct {
	re *regexp.Regexp
}
