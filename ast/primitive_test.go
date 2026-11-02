package ast

import (
	"math"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func si(l1, c1, l2, c2 int) SourceInfo {
	return SourceInfo{Start: reader.Pos{Line: l1, Column: c1}, End: reader.Pos{Line: l2, Column: c2}}
}

func whitespace() Whitespace { return Whitespace{Value: "", SourceInfo: si(0, 0, 0, 0)} }

func variablePlaceholder(name string) Placeholder {
	return Placeholder{
		Space0: whitespace(),
		Expr: Expr{
			Kind:       ExprVariable{Value: Variable{Name: name, SourceInfo: si(0, 0, 0, 0)}},
			SourceInfo: si(0, 0, 0, 0),
		},
		Space1: whitespace(),
	}
}

func TestFloat(t *testing.T) {
	tests := []struct {
		value  float64
		source string
		str    string
	}{
		{1.0, "1.0", "1"},
		{1.01, "1.01", "1.01"},
		{1.01, "1.010", "1.01"},
		{-1.333, "-1.333", "-1.333"},
	}
	for _, tt := range tests {
		f := Float{Value: tt.value, Source: tt.source}
		if got := f.ToSource(); got != tt.source {
			t.Errorf("Float(%v).ToSource() = %q, want %q", tt.value, got, tt.source)
		}
		if got := f.String(); got != tt.str {
			t.Errorf("Float(%v).String() = %q, want %q", tt.value, got, tt.str)
		}
	}
}

// Expected values from Rust's f64 Display (rustc 1.94).
func TestFormatFloat(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{1e21, "1000000000000000000000"},
		{1e-7, "0.0000001"},
		{0.30000000000000004, "0.30000000000000004"},
		{math.Copysign(0, -1), "-0"},
		{math.Inf(1), "inf"},
		{math.Inf(-1), "-inf"},
		{math.NaN(), "NaN"},
		{123456789.125, "123456789.125"},
	}
	for _, tt := range tests {
		if got := FormatFloat(tt.value); got != tt.want {
			t.Errorf("FormatFloat(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestTemplate(t *testing.T) {
	tmpl := Template{
		Elements: []TemplateElement{
			TemplateString{Value: "Hello ", Source: "Hello "},
			TemplatePlaceholder{Value: variablePlaceholder("name")},
			TemplateString{Value: "!", Source: "!"},
		},
		SourceInfo: si(0, 0, 0, 0),
	}
	if got := tmpl.String(); got != "Hello {{name}}!" {
		t.Errorf("got %q", got)
	}
}

func TestTemplateToSource(t *testing.T) {
	str := func(s string) Template {
		return Template{Delimiter: '"', Elements: []TemplateElement{TemplateString{Value: s, Source: s}}, SourceInfo: si(0, 0, 0, 0)}
	}

	tests := []struct {
		want string
		got  interface{ ToSource() string }
	}{
		{"{{x}}", JSONPlaceholder{Value: variablePlaceholder("x")}},
		{"1", JSONNumber{Value: "1"}},
		{`"hello"`, JSONString{Value: str("hello")}},
		{"true", JSONBoolean{Value: true}},
		{"[]", JSONList{Space0: "", Elements: nil}},
		{"[1, 2, 3]", JSONList{Space0: "", Elements: []JSONListElement{
			{Space0: "", Value: JSONNumber{Value: "1"}, Space1: ""},
			{Space0: " ", Value: JSONNumber{Value: "2"}, Space1: ""},
			{Space0: " ", Value: JSONNumber{Value: "3"}, Space1: ""},
		}}},
		{"{}", JSONObject{Space0: "", Elements: nil}},
		{`{ "id": 123 }`, JSONObject{Space0: "", Elements: []JSONObjectElement{{
			Space0: " ",
			Name:   str("id"),
			Space1: "",
			Space2: " ",
			Value:  JSONNumber{Value: "123"},
			Space3: " ",
		}}}},
		{"null", JSONNull{}},
		{"{{name}}", TemplatePlaceholder{Value: variablePlaceholder("name")}},
		{"{{name}}", Template{Elements: []TemplateElement{TemplatePlaceholder{Value: variablePlaceholder("name")}}, SourceInfo: si(1, 1, 1, 1)}},
	}
	for _, tt := range tests {
		if got := tt.got.ToSource(); got != tt.want {
			t.Errorf("%#v.ToSource() = %q, want %q", tt.got, got, tt.want)
		}
	}
}

func TestPlaceholderToSourceKeepsWhitespace(t *testing.T) {
	p := variablePlaceholder("name")
	p.Space0.Value = " "
	p.Space1.Value = "  "
	if got := p.ToSource(); got != "{{ name  }}" {
		t.Errorf("got %q", got)
	}
	// JSON placeholders drop the inner whitespace, as in Hurl.
	if got := (JSONPlaceholder{Value: p}).ToSource(); got != "{{name}}" {
		t.Errorf("got %q", got)
	}
}

func TestCookiePath(t *testing.T) {
	path := CookiePath{
		Name: Template{
			Elements:   []TemplateElement{TemplateString{Value: "LSID", Source: "unused"}},
			SourceInfo: si(0, 0, 0, 0),
		},
		Attribute: &CookieAttribute{
			Space0: whitespace(),
			Name:   CookieAttributeName{Kind: CookieAttributeMaxAge, Value: "Max-Age"},
			Space1: whitespace(),
		},
	}
	if got := path.String(); got != "LSID[Max-Age]" {
		t.Errorf("got %q", got)
	}
}

func TestCount(t *testing.T) {
	for _, tt := range []struct {
		n    int32
		want Count
		str  string
	}{
		{-1, CountInfinite{}, "-1"},
		{0, CountFinite{Value: 0}, "0"},
		{10, CountFinite{Value: 10}, "10"},
	} {
		got, err := CountFromInt32(tt.n)
		if err != nil || got != tt.want || got.String() != tt.str {
			t.Errorf("CountFromInt32(%d) = %v, %v", tt.n, got, err)
		}
	}
	if _, err := CountFromInt32(-2); err == nil || err.Error() != "-2 is not in -1..=2147483647" {
		t.Errorf("got %v", err)
	}
}

func TestOptionKindString(t *testing.T) {
	unit := DurationUnitSecond
	tests := []struct {
		kind OptionKind
		want string
	}{
		{OptionRetry{Value: CountLiteral{Value: CountInfinite{}}}, "retry: -1"},
		{OptionDelay{Value: DurationLiteral{Value: Duration{Value: U64{Value: 3, Source: "3"}, Unit: &unit}}}, "delay: 3s"},
		{OptionInsecure{Value: BooleanLiteral{Value: true}}, "insecure: true"},
		{OptionVerbosity{Value: VerbosityDebug}, "verbosity: debug"},
		{OptionVariable{Value: VariableDefinition{Name: "a", Value: VariableNull{}}}, "variable: a=null"},
		{OptionFollowLocationTrusted{Value: BooleanPlaceholder{Value: variablePlaceholder("x")}}, "location-trusted: x"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}
