package runner

import (
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func sourceInfo(l1, c1, l2, c2 int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: l1, Column: c1}, End: reader.Pos{Line: l2, Column: c2}}
}

func templateString(s string, si ast.SourceInfo) ast.Template {
	return ast.Template{Elements: []ast.TemplateElement{ast.TemplateString{Value: s, Source: s}}, SourceInfo: si}
}

func variableExpr(name string, si ast.SourceInfo) ast.Expr {
	return ast.Expr{Kind: ast.ExprVariable{Value: ast.Variable{Name: name, SourceInfo: si}}, SourceInfo: si}
}

func TestRepr(t *testing.T) {
	tests := []struct {
		v    Value
		want string
	}{
		{NewBool(true), "boolean <true>"},
		{ValueBytes{[]byte{1, 2, 3}}, "bytes <010203>"},
		{ValueDate{time.Date(2000, 1, 1, 12, 0, 0, 123_000_000, time.UTC)}, "date <2000-01-01 12:00:00.123 UTC>"},
		{ValueList{}, "list <[]>"},
		{ValueNodeset{5}, "nodeset <Nodeset(size=5)>"},
		{ValueNull{}, "null <null>"},
		{NewInteger(1), "integer <1>"},
		{NewFloat(1.0), "float <1.0>"},
		{ValueObject{}, "object <Object()>"},
		{ValueRegex{regexp.MustCompile(`[0-9]+`)}, "regex </[0-9]+/>"},
		{NewString("Hello"), "string <Hello>"},
		{ValueUnit{}, "unit"},
	}
	for _, tt := range tests {
		if got := Repr(tt.v); got != tt.want {
			t.Errorf("Repr(%#v) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestIsScalar(t *testing.T) {
	if !IsScalar(NewInteger(1)) {
		t.Error("integer should be scalar")
	}
	if IsScalar(ValueList{}) {
		t.Error("list shouldn't be scalar")
	}
}

func TestEqual(t *testing.T) {
	if Equal(NewBool(true), NewBool(false)) {
		t.Error("true == false")
	}
	if !Equal(NewInteger(1), NewFloat(1.0)) {
		t.Error("1 != 1.0")
	}
	if Equal(NewBool(true), NewString("Hello")) {
		t.Error("true == Hello")
	}
}

func TestFormatISO(t *testing.T) {
	tests := []struct {
		t    time.Time
		want string
	}{
		{time.Date(2000, 1, 1, 12, 0, 0, 123_000_000, time.UTC), "2000-01-01T12:00:00.123000Z"},
		{time.Date(2000, 2, 1, 12, 0, 0, 123_456_000, time.UTC), "2000-02-01T12:00:00.123456Z"},
		{time.Date(2000, 2, 1, 12, 0, 0, 123_456_789, time.UTC), "2000-02-01T12:00:00.123456Z"},
	}
	for _, tt := range tests {
		if got, _ := Render(ValueDate{tt.t}); got != tt.want {
			t.Errorf("Render(%v) = %q, want %q", tt.t, got, tt.want)
		}
	}
}

func TestCompare(t *testing.T) {
	if c, err := valueCompare(NewInteger(1), NewInteger(1)); err != nil || c != 0 {
		t.Errorf("compare(1, 1) = %d, %v", c, err)
	}
	if c, err := valueCompare(NewInteger(1), NewInteger(2)); err != nil || c != -1 {
		t.Errorf("compare(1, 2) = %d, %v", c, err)
	}
	if c, err := valueCompare(NewString("xyz"), NewString("abc")); err != nil || c != 1 {
		t.Errorf("compare(xyz, abc) = %d, %v", c, err)
	}
	if _, err := valueCompare(NewInteger(1), NewBool(true)); err != EvalErrorType {
		t.Errorf("compare(1, true) err = %v", err)
	}
}

func TestStartsEndsWith(t *testing.T) {
	if ok, _ := valueStartsWith(NewString("Hello"), NewString("H")); !ok {
		t.Error("Hello should start with H")
	}
	if ok, _ := valueStartsWith(ValueBytes{[]byte{0, 1, 2}}, ValueBytes{[]byte{0, 2}}); ok {
		t.Error("[0,1,2] shouldn't start with [0,2]")
	}
	if ok, _ := valueEndsWith(NewString("Hello"), NewString("o")); !ok {
		t.Error("Hello should end with o")
	}
	if ok, _ := valueEndsWith(ValueBytes{[]byte{0, 1, 2}}, ValueBytes{[]byte{0, 2}}); ok {
		t.Error("[0,1,2] shouldn't end with [0,2]")
	}
}

func TestContains(t *testing.T) {
	haystack := ValueBytes{[]byte{1, 2, 3}}
	for _, tt := range []struct {
		needle []byte
		want   bool
	}{{[]byte{1}, true}, {[]byte{1, 2}, true}, {[]byte{1, 3}, false}} {
		if got, _ := valueContains(haystack, ValueBytes{tt.needle}); got != tt.want {
			t.Errorf("contains(%v) = %v", tt.needle, got)
		}
	}
	if ok, _ := valueContains(NewString("abc"), NewString("ab")); !ok {
		t.Error("abc should contain ab")
	}
	values := ValueList{[]Value{NewInteger(0), NewInteger(2), NewInteger(3)}}
	if ok, _ := valueContains(values, NewInteger(0)); !ok {
		t.Error("list should contain 0")
	}
	if ok, _ := valueContains(values, NewInteger(4)); ok {
		t.Error("list shouldn't contain 4")
	}
	if ok, _ := valueIncludes(values, NewInteger(0)); !ok {
		t.Error("list should include 0")
	}
	if ok, _ := valueIncludes(values, NewInteger(4)); ok {
		t.Error("list shouldn't include 4")
	}
}

func TestType(t *testing.T) {
	v1 := NewBool(true)
	if !valueIsBoolean(v1) || valueIsCollection(v1) {
		t.Error("bool")
	}
	v2 := NewInteger(1)
	if valueIsBoolean(v2) || valueIsCollection(v2) || !valueIsNumber(v2) || !valueIsInteger(v2) {
		t.Error("integer")
	}
	v3 := NewString("01cc0f54-8885-4a1d-9121-ae5d316a33c5")
	if valueIsBoolean(v3) || valueIsCollection(v3) || !valueIsString(v3) {
		t.Error("string")
	}
	if ok, _ := valueIsUUID(v3); !ok {
		t.Error("uuid")
	}
}

func TestIsUUIDForms(t *testing.T) {
	for _, s := range []string{"01cc0f5488854a1d9121ae5d316a33c5", "{01cc0f54-8885-4a1d-9121-ae5d316a33c5}", "urn:uuid:01CC0F54-8885-4A1D-9121-AE5D316A33C5"} {
		if !isUUID(s) {
			t.Errorf("isUUID(%q) = false", s)
		}
	}
	for _, s := range []string{"", "01cc0f54-8885-4a1d-9121-ae5d316a33c", "01cc0f54x8885-4a1d-9121-ae5d316a33c5", "g1cc0f5488854a1d9121ae5d316a33c5"} {
		if isUUID(s) {
			t.Errorf("isUUID(%q) = true", s)
		}
	}
}

func TestIsIP(t *testing.T) {
	tests := []struct {
		s          string
		ipv4, ipv6 bool
	}{
		{"192.168.1.1", true, false},
		{"::1", false, true},
		{"2001:db8::ff00:42:8329", false, true},
		{"192.168.01.1", false, false},
		{"fe80::1%eth0", false, false},
		{"hello", false, false},
	}
	for _, tt := range tests {
		v4, _ := valueIsIPv4(NewString(tt.s))
		v6, _ := valueIsIPv6(NewString(tt.s))
		if v4 != tt.ipv4 || v6 != tt.ipv6 {
			t.Errorf("%q: ipv4 %v ipv6 %v", tt.s, v4, v6)
		}
	}
	if _, err := valueIsIPv4(NewBool(true)); err != EvalErrorType {
		t.Error("expected type error")
	}
}

func TestISODate(t *testing.T) {
	for _, s := range []string{"1985-04-12T23:20:50.52Z", "1996-12-19T16:39:57-08:00", "1990-12-31T23:59:60Z", "1990-12-31T15:59:60-08:00", "1937-01-01T12:00:27.87+00:20"} {
		if ok, _ := valueIsISODate(NewString(s)); !ok {
			t.Errorf("isISODate(%q) = false", s)
		}
	}
	if ok, _ := valueIsISODate(NewString("1978-01-15")); ok {
		t.Error("1978-01-15 is not an ISO date")
	}
	if _, err := valueIsISODate(NewBool(true)); err != EvalErrorType {
		t.Error("expected type error")
	}
}

func TestIsMatch(t *testing.T) {
	value := NewString("hello")
	if ok, _ := valueIsMatch(value, NewString("he.*")); !ok {
		t.Error("he.*")
	}
	if ok, _ := valueIsMatch(value, ValueRegex{regexp.MustCompile("he.*")}); !ok {
		t.Error("/he.*/")
	}
	if ok, _ := valueIsMatch(value, NewString("HE.*")); ok {
		t.Error("HE.*")
	}
	if _, err := valueIsMatch(value, NewString("?HE.*")); err != EvalErrorInvalidRegex {
		t.Errorf("?HE.* err = %v", err)
	}
	if _, err := valueIsMatch(value, NewBool(true)); err != EvalErrorType {
		t.Errorf("true err = %v", err)
	}
}

func TestNumberToString(t *testing.T) {
	tests := []struct {
		n    Number
		want string
	}{
		{NumberFloat{1.0}, "1.0"},
		{NumberFloat{1.1}, "1.1"},
		{NumberBigInteger{"1.1"}, "1.1"},
		{NumberBigInteger{"1"}, "1"},
		// Rust's format_float quirks.
		{NumberFloat{-1.0}, "-1.0"},
		{NumberFloat{-1.5}, "-1.5.0"},
		{NumberFloat{1e21}, "1000000000000000000000.0"},
		{NumberFloat{0.0000001}, "0.0000001"},
	}
	for _, tt := range tests {
		if got := tt.n.String(); got != tt.want {
			t.Errorf("%#v.String() = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestCmpNumber(t *testing.T) {
	integerZero, integerMinusOne, integerOne, integerTwo := NumberInteger{0}, NumberInteger{-1}, NumberInteger{1}, NumberInteger{2}
	integerMax, integerMin := NumberInteger{9223372036854775807}, NumberInteger{-9223372036854775808}
	floatZero, floatOne, floatTwo := NumberFloat{0}, NumberFloat{1}, NumberFloat{2}
	floatOnePlusEpsilon := NumberFloat{1.000_000_000_000_000_100}
	floatOnePlusPlusEpsilon := NumberFloat{1.000_000_000_000_001}
	floatMin, floatMax := NumberFloat{-1.7976931348623157e308}, NumberFloat{1.7976931348623157e308}
	numberOne, numberTwo, numberTwoWithDecimal := NumberBigInteger{"1"}, NumberBigInteger{"2"}, NumberBigInteger{"2.0"}

	tests := []struct {
		a, b Number
		want int
	}{
		{integerMinusOne, integerZero, -1},
		{integerOne, integerOne, 0},
		{integerOne, numberOne, 0},
		{integerOne, floatOne, 0},
		{integerOne, integerZero, 1},
		{integerOne, floatZero, 1},
		{integerOne, integerTwo, -1},
		{integerOne, floatTwo, -1},
		{integerOne, numberTwo, -1},
		{integerOne, numberTwoWithDecimal, -1},
		{integerMin, floatMin, 1},
		{integerMax, floatMax, -1},
		{floatOne, floatOne, 0},
		{floatOne, floatOnePlusEpsilon, 0},
		{floatOne, floatOnePlusPlusEpsilon, -1},
		{NumberFloat{9_007_199_254_740_992.0}, NumberInteger{9_007_199_254_740_993}, 0},
	}
	for _, tt := range tests {
		if got := cmpNumber(tt.a, tt.b); got != tt.want {
			t.Errorf("cmpNumber(%v, %v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompareNumberString(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1", "1", 0},
		{"1", "1.0", 0},
		{"1.000", "1.0", 0},
		{"1", "2", -1},
		{"1.1", "2", -1},
		{"-001.1000", "-1.1", 0},
	}
	for _, tt := range tests {
		if got := compareNumberString(tt.a, tt.b); got != tt.want {
			t.Errorf("compareNumberString(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestNumberComponents(t *testing.T) {
	tests := []struct {
		s    string
		neg  bool
		i, d string
	}{
		{"1", false, "1", ""},
		{"1.0", false, "1", ""},
		{"01", false, "1", ""},
		{"1.1", false, "1", "1"},
		{"1.100", false, "1", "1"},
		{"-1.1", true, "1", "1"},
		{"-01.100", true, "1", "1"},
	}
	for _, tt := range tests {
		neg, i, d := numberComponents(tt.s)
		if neg != tt.neg || i != tt.i || d != tt.d {
			t.Errorf("numberComponents(%q) = %v, %q, %q", tt.s, neg, i, d)
		}
	}
}

func TestParseFloat(t *testing.T) {
	ok := map[string]float64{"3.1415": 3.1415, "1e3": 1000, "-2.": -2, ".5": 0.5, "+1": 1, "1e400": inf()}
	for s, want := range ok {
		if got, valid := ParseFloat(s); !valid || got != want {
			t.Errorf("ParseFloat(%q) = %v, %v", s, got, valid)
		}
	}
	for _, s := range []string{"3x.1415", "0x10", "1_000", "", " 1"} {
		if _, valid := ParseFloat(s); valid {
			t.Errorf("ParseFloat(%q) should fail", s)
		}
	}
}

func inf() float64 { f, _ := ParseFloat("inf"); return f }

func TestValueFromJSON(t *testing.T) {
	tests := []struct {
		json string
		want Value
	}{
		{"1000", NewInteger(1000)},
		{"1.0", NewFloat(1.0)},
		{"1000000000000000000000", NewBigInteger("1000000000000000000000")},
		{"1000000000000000000000.5", NewFloat(1000000000000000000000.5)},
		{`{"b":1,"a":[true,null,"x"]}`, ValueObject{[]KeyValue{
			{"a", ValueList{[]Value{NewBool(true), ValueNull{}, NewString("x")}}},
			{"b", NewInteger(1)},
		}}},
	}
	for _, tt := range tests {
		doc, err := parseJSON([]byte(tt.json))
		if err != nil {
			t.Fatalf("parseJSON(%q): %v", tt.json, err)
		}
		got := valueFromJSON(doc)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("valueFromJSON(%s) = %#v, want %#v", tt.json, got, tt.want)
		}
	}
	for _, s := range []string{"", "{", "1 2", `{"a":1}x`} {
		if _, err := parseJSON([]byte(s)); err == nil {
			t.Errorf("parseJSON(%q) should fail", s)
		}
	}
	if _, err := parseJSON([]byte(" [1] \n")); err != nil {
		t.Errorf("trailing whitespace: %v", err)
	}
}

func TestVariableSet(t *testing.T) {
	variables := NewVariableSet()
	variables.Insert("foo", NewString("xxx"))
	variables.Insert("bar", NewInteger(42))
	variables.Insert("bar", NewBool(true))
	variables.Insert("baz", NewFloat(1.0))
	variables.InsertSecret("quic", "42")

	tests := []struct {
		name string
		want Variable
	}{
		{"foo", Variable{"foo", NewString("xxx"), VisibilityPublic}},
		{"bar", Variable{"bar", NewBool(true), VisibilityPublic}},
		{"baz", Variable{"baz", NewFloat(1.0), VisibilityPublic}},
		{"quic", Variable{"quic", NewString("42"), VisibilitySecret}},
	}
	for _, tt := range tests {
		if got, ok := variables.Get(tt.name); !ok || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Get(%q) = %#v, %v", tt.name, got, ok)
		}
	}
	for _, name := range []string{"Foo", "BAZ"} {
		if _, ok := variables.Get(name); ok {
			t.Errorf("Get(%q) should be absent", name)
		}
	}
	if got := variables.Secrets(); !reflect.DeepEqual(got, []string{"42"}) {
		t.Errorf("Secrets() = %v", got)
	}
}

func TestIterVariableSet(t *testing.T) {
	data := map[string]Value{"foo": NewString("yyy"), "bar": NewBool(false), "baz": NewFloat(12.0)}
	variables := NewVariableSet()
	for name, value := range data {
		variables.Insert(name, value)
	}
	if len(variables.All()) != len(data) {
		t.Fatalf("All() has %d variables", len(variables.All()))
	}
	for name, variable := range variables.All() {
		if !Equal(data[name], variable.Value) {
			t.Errorf("%s = %v", name, variable.Value)
		}
	}
}

func TestSecretVariableCanBeReassignedButRemainsSecret(t *testing.T) {
	var variables VariableSet
	variables.InsertSecret("foo", "42")
	variables.Insert("foo", NewString("xxx"))
	if v, _ := variables.Get("foo"); !Equal(v.Value, NewString("xxx")) {
		t.Errorf("foo = %v", v.Value)
	}
	if got := variables.Secrets(); !reflect.DeepEqual(got, []string{"42"}) {
		t.Errorf("Secrets() = %v", got)
	}
}

func TestGetSecrets(t *testing.T) {
	variables := NewVariableSet()
	variables.InsertSecret("foo", "42")
	variables.Insert("bar", NewString("toto"))
	variables.Insert("baz", NewBool(true))
	variables.InsertSecret("a", "1234")
	if got := variables.Secrets(); !reflect.DeepEqual(got, []string{"1234", "42"}) {
		t.Errorf("Secrets() = %v", got)
	}
}
