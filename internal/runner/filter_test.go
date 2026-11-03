package runner

import (
	"reflect"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

func newFilter(v ast.FilterValue) ast.Filter {
	return ast.Filter{SourceInfo: sourceInfo(1, 1, 1, 1), Value: v}
}

func applyFilter(f ast.Filter, v Value) (Value, *RunnerError) {
	return evalFilter(f, v, NewVariableSet(), false, DefaultFilterOptions())
}

type filterCase struct {
	name   string
	filter ast.Filter
	input  Value
	want   Value
	// wantKind is the expected error kind, if any.
	wantKind RunnerErrorKind
}

func runFilterCases(t *testing.T, cases []filterCase) {
	t.Helper()
	for _, tt := range cases {
		got, err := applyFilter(tt.filter, tt.input)
		if tt.wantKind != nil {
			if err == nil || !reflect.DeepEqual(err.Kind, tt.wantKind) {
				t.Errorf("%s: err = %#v, want kind %#v (value %v)", tt.name, err, tt.wantKind, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error %#v", tt.name, err)
			continue
		}
		if tt.want == nil {
			if got != nil {
				t.Errorf("%s: got %#v, want no value", tt.name, got)
			}
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: got %#v, want %#v", tt.name, got, tt.want)
		}
	}
}

func inputType(actual, expected string) RunnerErrorKind {
	return RunnerErrorFilterInvalidInputType{Actual: actual, Expected: expected}
}

func inputValue(reason string) RunnerErrorKind {
	return RunnerErrorFilterInvalidInputValue{Value: reason}
}

func ints(values ...int64) Value {
	l := make([]Value, len(values))
	for i, v := range values {
		l[i] = NewInteger(v)
	}
	return ValueList{l}
}

func TestEvalFilters(t *testing.T) {
	filters := []ast.Filter{{SourceInfo: sourceInfo(1, 1, 1, 6), Value: ast.FilterCount{}}}
	got, err := evalFilters(filters, ints(1, 2, 2), NewVariableSet(), false, DefaultFilterOptions())
	if err != nil || !Equal(got, NewInteger(3)) {
		t.Errorf("got %v, %v", got, err)
	}
	// A filter without input value.
	filters = []ast.Filter{
		{SourceInfo: sourceInfo(1, 1, 1, 6), Value: ast.FilterRegex{Value: ast.RegexTemplate{Value: templateString("x(.)", sourceInfo(1, 7, 1, 10))}}},
		{SourceInfo: sourceInfo(1, 12, 1, 17), Value: ast.FilterCount{}},
	}
	_, err = evalFilters(filters, NewString("abc"), NewVariableSet(), true, DefaultFilterOptions())
	if err == nil || err.Kind != (RunnerErrorFilterMissingInput{}) || err.SourceInfo() != sourceInfo(1, 12, 1, 17) || !err.Assert {
		t.Errorf("err = %#v", err)
	}
}

func TestFilterBase64(t *testing.T) {
	bytes := ValueBytes{[]byte{0x3c, 0x3c, 0x3f, 0x3f, 0x3f, 0x3e, 0x3e}}
	runFilterCases(t, []filterCase{
		{"decode ok", newFilter(ast.FilterBase64Decode{}), NewString("PDw/Pz8+Pg=="), bytes, nil},
		{"decode invalid characters", newFilter(ast.FilterBase64Decode{}), NewString("!@#"), nil, inputValue("string is not base64")},
		{"decode invalid input", newFilter(ast.FilterBase64Decode{}), ValueBytes{[]byte{0xc4, 0xe3, 0xba}}, nil, inputType("bytes", "base64 string")},
		{"encode ok", newFilter(ast.FilterBase64Encode{}), bytes, NewString("PDw/Pz8+Pg=="), nil},
		{"encode invalid input", newFilter(ast.FilterBase64Encode{}), NewString("你好世界"), nil, inputType("string", "bytes")},
		{"url safe decode ok", newFilter(ast.FilterBase64URLSafeDecode{}), NewString("PDw_Pz8-Pg"), bytes, nil},
		{"url safe decode invalid characters", newFilter(ast.FilterBase64URLSafeDecode{}), NewString("!@#"), nil, inputValue("string is not base64")},
		{"url safe decode padding", newFilter(ast.FilterBase64URLSafeDecode{}), NewString("0YjQtdC70LvRiw=="), nil, inputValue("base64 string contains padding")},
		{"url safe decode invalid input", newFilter(ast.FilterBase64URLSafeDecode{}), ValueBytes{[]byte{0xc4, 0xe3, 0xba}}, nil, inputType("bytes", "string")},
		{"url safe encode ok", newFilter(ast.FilterBase64URLSafeEncode{}), bytes, NewString("PDw_Pz8-Pg"), nil},
		{"url safe encode invalid input", newFilter(ast.FilterBase64URLSafeEncode{}), NewString("你好世界"), nil, inputType("string", "bytes")},
	})
}

func newDecodeFilter(encoding string) ast.Filter {
	return newFilter(ast.FilterDecode{
		Space0:   ast.Whitespace{SourceInfo: sourceInfo(7, 1, 8, 1)},
		Encoding: templateString(encoding, sourceInfo(8, 1, 8+len(encoding), 1)),
	})
}

func TestFilterDecode(t *testing.T) {
	runFilterCases(t, []filterCase{
		{"utf-8", newDecodeFilter("utf-8"), ValueBytes{[]byte{0xe4, 0xbd, 0xa0, 0xe5, 0xa5, 0xbd, 0xe4, 0xb8, 0x96, 0xe7, 0x95, 0x8c}}, NewString("你好世界"), nil},
		{"gb2312", newDecodeFilter("gb2312"), ValueBytes{[]byte{0xc4, 0xe3, 0xba, 0xc3, 0xca, 0xc0, 0xbd, 0xe7}}, NewString("你好世界"), nil},
		{"unknown encoding", newDecodeFilter("xxx"), ValueBytes{}, nil, RunnerErrorFilterInvalidEncoding{Value: "xxx"}},
		{"bad bytes", newDecodeFilter("gb2312"), ValueBytes{[]byte{0xc4, 0x00}}, nil, RunnerErrorFilterDecode{Value: "gb2312"}},
		{"bad input type", newDecodeFilter("utf-8"), NewString("café"), nil, inputType("string", "bytes")},
	})
	f := ast.Filter{SourceInfo: sourceInfo(1, 1, 1, 1), Value: ast.FilterCharsetDecode{Encoding: templateString("utf-8", sourceInfo(1, 1, 1, 1))}}
	if got, err := applyFilter(f, ValueBytes{[]byte("café")}); err != nil || !Equal(got, NewString("café")) {
		t.Errorf("charsetDecode: got %v, %v", got, err)
	}
}

func TestFilterCount(t *testing.T) {
	filter := ast.Filter{SourceInfo: sourceInfo(1, 1, 1, 6), Value: ast.FilterCount{}}
	runFilterCases(t, []filterCase{
		{"list", filter, ints(1, 2, 2), NewInteger(3), nil},
		{"bool", filter, NewBool(true), nil, inputType("boolean", "list, bytes or nodeset")},
	})
	if _, err := applyFilter(filter, NewBool(true)); err.SourceInfo() != sourceInfo(1, 1, 1, 6) {
		t.Errorf("source info = %v", err.SourceInfo())
	}
}

func TestFilterDaysAfterBeforeNow(t *testing.T) {
	now := time.Now().UTC()
	nowPlus30Hours := now.Add(30 * time.Hour)
	runFilterCases(t, []filterCase{
		{"after now", newFilter(ast.FilterDaysAfterNow{}), ValueDate{now}, NewInteger(0), nil},
		{"after now + 30h", newFilter(ast.FilterDaysAfterNow{}), ValueDate{nowPlus30Hours}, NewInteger(1), nil},
		{"before now + 30h", newFilter(ast.FilterDaysBeforeNow{}), ValueDate{nowPlus30Hours}, NewInteger(-1), nil},
		{"not a date", newFilter(ast.FilterDaysBeforeNow{}), NewString("x"), nil, inputType("string", "date")},
	})
}

func TestFilterFirstLast(t *testing.T) {
	first := ast.Filter{SourceInfo: sourceInfo(1, 1, 1, 5), Value: ast.FilterFirst{}}
	last := ast.Filter{SourceInfo: sourceInfo(1, 1, 1, 4), Value: ast.FilterLast{}}
	runFilterCases(t, []filterCase{
		{"first ok", first, ints(1, 3, 5), NewInteger(1), nil},
		{"first empty", first, ValueList{}, nil, inputValue("list is empty")},
		{"first invalid", first, NewBool(true), nil, inputType("boolean", "list")},
		{"last ok", last, ints(1, 3, 5), NewInteger(5), nil},
		{"last empty", last, ValueList{}, nil, inputValue("list is empty")},
		{"last invalid", last, NewBool(true), nil, inputType("boolean", "list")},
	})
}

func newDateFormatFilter(format string) ast.Filter {
	return newFilter(ast.FilterDateFormat{
		Space0: ast.Whitespace{SourceInfo: sourceInfo(7, 1, 8, 1)},
		Fmt:    templateString(format, sourceInfo(8, 1, 8+len(format), 1)),
	})
}

func TestFilterDateFormat(t *testing.T) {
	date := ValueDate{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)}
	runFilterCases(t, []filterCase{
		{"ok", newDateFormatFilter("%m/%d/%Y"), date, NewString("01/01/2025"), nil},
		{"bad input type", newDateFormatFilter("%m/%d/%Y"), NewString("01/01/2025"), nil, inputType("string", "date")},
		{"invalid format", newDateFormatFilter("%%%"), date, nil, RunnerErrorFilterInvalidFormatSpecifier{Value: "%%%"}},
		{"format alias", newFilter(ast.FilterFormat{Fmt: templateString("%A %Z", sourceInfo(1, 1, 1, 1))}), date, NewString("Wednesday UTC"), nil},
	})
}

func TestFilterHTML(t *testing.T) {
	tests := [][2]string{
		{"foo", "foo"},
		{"<tag>", "&lt;tag&gt;"},
		{"foo & bar", "foo &amp; bar"},
		{`string with double quote: "baz"`, "string with double quote: &quot;baz&quot;"},
	}
	var cases []filterCase
	for _, tt := range tests {
		cases = append(cases,
			filterCase{"escape " + tt[0], newFilter(ast.FilterHTMLEscape{}), NewString(tt[0]), NewString(tt[1]), nil},
			filterCase{"unescape " + tt[1], newFilter(ast.FilterHTMLUnescape{}), NewString(tt[1]), NewString(tt[0]), nil},
		)
	}
	runFilterCases(t, cases)
}

func newJSONPathFilter(expr string) ast.Filter {
	tmpl := templateString(expr, sourceInfo(0, 0, 0, 0))
	tmpl.Delimiter = '"'
	return newFilter(ast.FilterJSONPath{Expr: tmpl})
}

func TestFilterJSONPath(t *testing.T) {
	filter := newJSONPathFilter("$.message")
	input := NewString(`{"message":"Hello"}`)
	runFilterCases(t, []filterCase{
		{"coercion", filter, input, NewString("Hello"), nil},
		{"no result", newJSONPathFilter("$.unknown"), input, nil, nil},
		{"invalid JSON", filter, NewString("{"), nil, inputValue("value is not a valid JSON")},
		{"invalid expression", newJSONPathFilter("$$"), input, nil, RunnerErrorQueryInvalidJSONPathExpression{Value: "$$"}},
		{"not a string", filter, NewInteger(1), nil, inputType("integer", "string")},
	})
	got, err := evalFilter(filter, input, NewVariableSet(), false, FilterOptions{UseJSONPathCoercion: false})
	if want := (ValueList{[]Value{NewString("Hello")}}); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("without coercion: got %#v, %v", got, err)
	}
}

func TestFilterLocation(t *testing.T) {
	u, _ := httpclient.ParseUrl("http://localhost:8000/redirected")
	runFilterCases(t, []filterCase{
		{"location", newFilter(ast.FilterLocation{}), ValueHTTPResponse{NewHTTPResponse(&u, 302)}, NewString("http://localhost:8000/redirected"), nil},
		{"no location", newFilter(ast.FilterLocation{}), ValueHTTPResponse{NewHTTPResponse(nil, 200)}, nil, nil},
		{"not a response", newFilter(ast.FilterLocation{}), NewString("x"), nil, inputType("string", "http response")},
	})
	if got := (ValueHTTPResponse{NewHTTPResponse(nil, 200)}).String(); got != "Response(location=None, status=200)" {
		t.Errorf("String() = %q", got)
	}
}

func newNthFilter(n int64, source string) ast.Filter {
	return newFilter(ast.FilterNth{N: ast.IntegerLiteral{Value: ast.I64{Value: n, Source: source}}})
}

func TestFilterNth(t *testing.T) {
	runFilterCases(t, []filterCase{
		{"positive", newNthFilter(2, "2"), ints(0, 1, 2, 3), NewInteger(2), nil},
		{"positive out of bound", newNthFilter(2, "2"), ints(0, 1), nil, inputValue("out of bound - size is 2")},
		{"negative", newNthFilter(-4, "-4"), ints(0, 1, 2, 3), NewInteger(0), nil},
		{"negative out of bound", newNthFilter(-4, "-4"), ints(0), nil, inputValue("out of bound - size is 1")},
	})
	variables := NewVariableSet()
	variables.Insert("n", NewString("x"))
	f := newFilter(ast.FilterNth{N: ast.IntegerPlaceholder{Value: placeholder("n", sourceInfo(1, 7, 1, 8))}})
	_, err := evalFilter(f, ints(0), variables, false, DefaultFilterOptions())
	if err == nil || err.Kind != (RunnerErrorExpressionInvalidType{Value: "string <x>", Expecting: "integer"}) || err.SourceInfo() != sourceInfo(1, 7, 1, 8) {
		t.Errorf("err = %#v", err)
	}
}

func TestTryNth(t *testing.T) {
	values := []int{12, 4, 5}
	for _, tt := range []struct {
		index int64
		want  int
		err   string
	}{
		{0, 12, ""}, {2, 5, ""}, {3, 0, "out of bound - size is 3"}, {-1, 5, ""}, {-3, 12, ""}, {4, 0, "out of bound - size is 3"},
	} {
		got, reason, ok := tryNth(values, tt.index)
		if got != tt.want || reason != tt.err || ok != (tt.err == "") {
			t.Errorf("tryNth(%d) = %d, %q, %v", tt.index, got, reason, ok)
		}
	}
}

func newRegexFilter(pattern string) ast.Filter {
	return ast.Filter{
		SourceInfo: sourceInfo(1, 1, 1, 20),
		Value:      ast.FilterRegex{Value: ast.RegexTemplate{Value: templateString(pattern, sourceInfo(1, 7, 1, 20))}},
	}
}

func TestFilterRegex(t *testing.T) {
	runFilterCases(t, []filterCase{
		{"match", newRegexFilter("Hello (.*)!"), NewString("Hello Bob!"), NewString("Bob"), nil},
		{"no match", newRegexFilter("Hello (.*)!"), NewString("Bye Bob!"), nil, nil},
		{"no group", newRegexFilter("Hello"), NewString("Hello Bob!"), nil, nil},
		{"not a string", newRegexFilter("Hello (.*)!"), NewBool(true), nil, inputType("boolean", "string")},
	})
	_, err := applyFilter(newRegexFilter("Hello (.*)!"), NewBool(true))
	if err.SourceInfo() != sourceInfo(1, 1, 1, 20) {
		t.Errorf("source info = %v", err.SourceInfo())
	}
	_, err = applyFilter(newRegexFilter("???"), NewString("Hello Bob!"))
	if err == nil || err.Kind != (RunnerErrorInvalidRegex{}) || err.SourceInfo() != sourceInfo(1, 7, 1, 20) {
		t.Errorf("invalid regex err = %#v", err)
	}
}

func quotedTemplate(s string) ast.Template {
	tmpl := templateString(s, sourceInfo(0, 0, 0, 0))
	tmpl.Delimiter = '"'
	return tmpl
}

func TestFilterReplace(t *testing.T) {
	replace := func(o, n string) ast.Filter {
		return newFilter(ast.FilterReplace{OldValue: quotedTemplate(o), NewValue: quotedTemplate(n)})
	}
	replaceRegex := newFilter(ast.FilterReplaceRegex{
		Pattern:  ast.RegexTemplate{Value: ast.Template{Elements: []ast.TemplateElement{ast.TemplateString{Value: `\s+`, Source: ","}}, SourceInfo: sourceInfo(1, 7, 1, 20)}},
		NewValue: quotedTemplate(","),
	})
	runFilterCases(t, []filterCase{
		{"replace", replace("bar", "foo"), NewString("foo bar baz"), NewString("foo foo baz"), nil},
		{"first arg is not a regex", replace("[0-9]", "x"), NewString("1234 [0-9]"), NewString("1234 x"), nil},
		{"replace regex", replaceRegex, NewString("1 2\t3  4"), NewString("1,2,3,4"), nil},
		{"replace regex group", newFilter(ast.FilterReplaceRegex{
			Pattern:  ast.RegexTemplate{Value: quotedTemplate(`(\w+)@(\w+)`)},
			NewValue: quotedTemplate("$2 at $1"),
		}), NewString("bob@example"), NewString("example at bob"), nil},
	})
}

func TestFilterSplit(t *testing.T) {
	split := func(sep string) ast.Filter { return newFilter(ast.FilterSplit{Sep: quotedTemplate(sep)}) }
	strs := func(values ...string) Value {
		l := make([]Value, len(values))
		for i, v := range values {
			l[i] = NewString(v)
		}
		return ValueList{l}
	}
	runFilterCases(t, []filterCase{
		{"split", split(","), NewString("1,2,3"), strs("1", "2", "3"), nil},
		{"empty separator", split(""), NewString("ab"), strs("", "a", "b", ""), nil},
		{"empty separator and input", split(""), NewString(""), strs("", ""), nil},
	})
}

func newToDateFilter(format string) ast.Filter {
	return newFilter(ast.FilterToDate{Fmt: templateString(format, sourceInfo(0, 0, 0, 0))})
}

func TestFilterToDate(t *testing.T) {
	runFilterCases(t, []filterCase{
		{"with timezone", newToDateFilter("%Y-%m-%d %H:%M:%S %:z"), NewString("2020-08-27 09:07:46 +02:00"), ValueDate{time.Date(2020, 8, 27, 7, 7, 46, 0, time.UTC)}, nil},
		{"without timezone", newToDateFilter("%Y-%m-%d %H:%M:%S"), NewString("2020-08-27 09:07:46"), ValueDate{time.Date(2020, 8, 27, 9, 7, 46, 0, time.UTC)}, nil},
		{"date only", newToDateFilter("%Y-%m-%d"), NewString("2020-08-27"), ValueDate{time.Date(2020, 8, 27, 0, 0, 0, 0, time.UTC)}, nil},
		{"invalid format", newToDateFilter("ymd"), NewString("2020-08-27"), nil, RunnerErrorFilterDateParsingError{Date: "2020-08-27", Format: "ymd"}},
		{"invalid input", newToDateFilter("%Y-%m-%d"), ValueBytes{[]byte{0xc4, 0xe3, 0xba}}, nil, inputType("bytes", "string")},
		{"rfc3339", newToDateFilter("%+"), NewString("2022-01-03T10:20:30.123456+01:00"), ValueDate{time.Date(2022, 1, 3, 9, 20, 30, 123456000, time.UTC)}, nil},
	})
}

func TestFilterConvert(t *testing.T) {
	runFilterCases(t, []filterCase{
		{"toFloat string", newFilter(ast.FilterToFloat{}), NewString("3.1415"), NewFloat(3.1415), nil},
		{"toFloat float", newFilter(ast.FilterToFloat{}), NewFloat(3.1415), NewFloat(3.1415), nil},
		{"toFloat float 3.0", newFilter(ast.FilterToFloat{}), NewFloat(3.0), NewFloat(3.0), nil},
		{"toFloat integer", newFilter(ast.FilterToFloat{}), NewInteger(3), NewFloat(3.0), nil},
		{"toFloat invalid string", newFilter(ast.FilterToFloat{}), NewString("3x.1415"), nil, inputValue("string <3x.1415>")},
		{"toFloat bool", newFilter(ast.FilterToFloat{}), NewBool(true), nil, inputType("boolean", "float, integer or string")},
		{"toFloat big integer", newFilter(ast.FilterToFloat{}), NewBigInteger("100000000000000000000"), nil, inputValue("integer <100000000000000000000> is too big to be cast as a float")},
		{"toHex ok", newFilter(ast.FilterToHex{}), ValueBytes{[]byte{0x3c, 0x3c, 0x3f, 0x3f, 0x3f, 0x3e, 0x3e}}, NewString("3c3c3f3f3f3e3e"), nil},
		{"toHex invalid", newFilter(ast.FilterToHex{}), NewString("你好世界"), nil, inputType("string", "bytes")},
		{"toInt string", newFilter(ast.FilterToInt{}), NewString("123"), NewInteger(123), nil},
		{"toInt integer", newFilter(ast.FilterToInt{}), NewInteger(123), NewInteger(123), nil},
		{"toInt float", newFilter(ast.FilterToInt{}), NewFloat(1.6), NewInteger(1), nil},
		{"toInt invalid string", newFilter(ast.FilterToInt{}), NewString("123x"), nil, inputValue("string <123x>")},
		{"toInt bool", newFilter(ast.FilterToInt{}), NewBool(true), nil, inputType("boolean", "float, integer or string")},
		{"toString", newFilter(ast.FilterToString{}), NewInteger(100), NewString("100"), nil},
		{"toString list", newFilter(ast.FilterToString{}), ValueList{}, nil, inputValue("list <[]> can not be converted to a string")},
	})
}

func TestFilterURL(t *testing.T) {
	urlFilter := func(v ast.FilterValue) ast.Filter { return ast.Filter{SourceInfo: sourceInfo(0, 0, 0, 0), Value: v} }
	runFilterCases(t, []filterCase{
		{"decode", urlFilter(ast.FilterURLDecode{}), NewString("https://mozilla.org/?x=%D1%88%D0%B5%D0%BB%D0%BB%D1%8B"), NewString("https://mozilla.org/?x=шеллы"), nil},
		{"decode invalid escapes", urlFilter(ast.FilterURLDecode{}), NewString("100%-%4"), NewString("100%-%4"), nil},
		{"encode", urlFilter(ast.FilterURLEncode{}), NewString("https://mozilla.org/?x=шеллы"), NewString("https%3A//mozilla.org/%3Fx%3D%D1%88%D0%B5%D0%BB%D0%BB%D1%8B"), nil},
	})
}

func newParamFilter(param string) ast.Filter {
	return newFilter(ast.FilterURLQueryParam{
		Space0: ast.Whitespace{SourceInfo: sourceInfo(14, 0, 15, 0)},
		Param:  templateString(param, sourceInfo(15, 0, 15+len(param), 0)),
	})
}

func TestFilterURLQueryParam(t *testing.T) {
	url := NewString("http://localhost:8000/api/v1/doc?text=你好世界")
	runFilterCases(t, []filterCase{
		{"ok", newParamFilter("text"), url, NewString("你好世界"), nil},
		{"not found", newParamFilter("length"), url, nil, nil},
		{"invalid input", newParamFilter("text"), ValueBytes{[]byte{0xc4, 0xe3, 0xba, 0xc3, 0xca, 0xc0, 0xbd, 0xe7}}, nil, inputType("bytes", "string")},
		{"invalid url", newParamFilter("text"), NewString("localhost:8000/api/v1/doc?text=你好世界"), nil, RunnerErrorInvalidURL{URL: "localhost:8000/api/v1/doc?text=你好世界", Message: "Missing scheme <http://> or <https://>"}},
	})
}

func TestFilterUTF8(t *testing.T) {
	si := sourceInfo(1, 1, 1, 10)
	for _, tt := range []struct {
		bytes []byte
		want  string
	}{
		{[]byte("Hello World"), "Hello World"},
		{[]byte("Hello \xF0\x90\x80World"), "Hello \uFFFDWorld"},
		{[]byte("\xF0\x9F\x8D\x8E\xF0\x9F\x8D\x8C\xF0\x9F\x8D\x90\xF0\x9F\x8D\x8A"), "🍎🍌🍐🍊"},
		{[]byte("a\xC0\xAFb\xED\xA0\x80c\xFF"), "a\uFFFD\uFFFDb\uFFFD\uFFFD\uFFFDc\uFFFD"},
	} {
		got, err := evalUTF8Decode(ValueBytes{tt.bytes}, si, false)
		if err != nil || !Equal(got, NewString(tt.want)) {
			t.Errorf("utf8Decode(%q) = %q, %v", tt.bytes, got, err)
		}
	}
	if _, err := evalUTF8Decode(NewInteger(42), si, false); err == nil {
		t.Error("utf8Decode(42) should fail")
	}
	for _, tt := range []struct {
		s    string
		want []byte
	}{
		{"Hello World", []byte("Hello World")},
		{"🐶🐱🐭🐹", []byte("\xF0\x9F\x90\xB6\xF0\x9F\x90\xB1\xF0\x9F\x90\xAD\xF0\x9F\x90\xB9")},
	} {
		got, err := evalUTF8Encode(NewString(tt.s), si, false)
		if err != nil || !reflect.DeepEqual(got, ValueBytes{tt.want}) {
			t.Errorf("utf8Encode(%q) = %v, %v", tt.s, got, err)
		}
	}
}

func newXPathFilter(expr string) ast.Filter {
	return newFilter(ast.FilterXPath{
		Space0: ast.Whitespace{SourceInfo: sourceInfo(6, 1, 7, 1)},
		Expr:   templateString(expr, sourceInfo(7, 1, 7+len(expr), 1)),
	})
}

func TestFilterXPath(t *testing.T) {
	html := NewString("<html><body>你好世界</body></html>")
	runFilterCases(t, []filterCase{
		{"ok", newXPathFilter("string(//body/text())"), html, NewString("你好世界"), nil},
		{"invalid xpath", newXPathFilter("str(//body/text())"), html, nil, RunnerErrorInvalidXPathEval{}},
		{"invalid xml", newXPathFilter("string(//body/text())"), NewString(""), nil, inputValue("value is not a valid XML")},
		{"invalid input", newXPathFilter("string(//body/text())"), ValueBytes{[]byte{0xc4, 0xe3, 0xba, 0xc3, 0xca, 0xc0, 0xbd, 0xe7}}, nil, inputType("bytes", "string")},
	})
}
