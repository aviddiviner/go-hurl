package runner

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/xpath"
)

func placeholderOption(name string) ast.Placeholder { return placeholder(name, sourceInfo(0, 0, 0, 0)) }

func TestEvalBooleanOption(t *testing.T) {
	vars := NewVariableSet()
	if v, err := evalBooleanOption(ast.BooleanLiteral{Value: true}, vars); err != nil || !v {
		t.Errorf("literal: %v, %v", v, err)
	}
	vars.Insert("verbose", NewBool(true))
	if v, err := evalBooleanOption(ast.BooleanPlaceholder{Value: placeholderOption("verbose")}, vars); err != nil || !v {
		t.Errorf("placeholder: %v, %v", v, err)
	}
}

func TestEvalBooleanOptionError(t *testing.T) {
	vars := NewVariableSet()
	option := ast.BooleanPlaceholder{Value: placeholderOption("verbose")}
	_, err := evalBooleanOption(option, vars)
	if err == nil || err.Assert || err.Kind != (RunnerErrorTemplateVariableNotDefined{Name: "verbose"}) {
		t.Errorf("err = %#v", err)
	}
	vars.Insert("verbose", NewInteger(10))
	_, err = evalBooleanOption(option, vars)
	if err == nil || err.Kind != (RunnerErrorExpressionInvalidType{Value: "integer <10>", Expecting: "boolean"}) {
		t.Errorf("err = %#v", err)
	}
}

func TestEvalDurationOption(t *testing.T) {
	vars := NewVariableSet()
	second := ast.DurationUnitSecond
	got, err := evalDurationOption(ast.DurationLiteral{Value: ast.Duration{Value: ast.U64{Value: 1, Source: "1"}, Unit: &second}}, vars, ast.DurationUnitMilliSecond)
	if err != nil || got != time.Second {
		t.Errorf("literal: %v, %v", got, err)
	}
	vars.Insert("retry", NewInteger(10))
	got, err = evalDurationOption(ast.DurationPlaceholder{Value: placeholderOption("retry")}, vars, ast.DurationUnitMilliSecond)
	if err != nil || got != 10*time.Millisecond {
		t.Errorf("placeholder: %v, %v", got, err)
	}
	vars.Insert("retry", NewInteger(-1))
	_, err = evalDurationOption(ast.DurationPlaceholder{Value: placeholderOption("retry")}, vars, ast.DurationUnitMilliSecond)
	if err == nil || err.Kind != (RunnerErrorExpressionInvalidType{Value: "integer <-1>", Expecting: "positive integer"}) {
		t.Errorf("err = %#v", err)
	}
}

func TestEvalCountOption(t *testing.T) {
	vars := NewVariableSet()
	option := ast.CountPlaceholder{Value: placeholderOption("n")}
	for n, want := range map[int64]ast.Count{-1: ast.CountInfinite{}, 0: ast.CountFinite{Value: 0}, 3: ast.CountFinite{Value: 3}} {
		vars.Insert("n", NewInteger(n))
		if got, err := evalCountOption(option, vars); err != nil || got != want {
			t.Errorf("%d: got %v, %v", n, got, err)
		}
	}
	vars.Insert("n", NewInteger(-2))
	if _, err := evalCountOption(option, vars); err == nil || err.Kind != (RunnerErrorExpressionInvalidType{Value: "integer <-2>", Expecting: "integer >= -1"}) {
		t.Errorf("err = %#v", err)
	}
}

func testLogger() *logger.Logger {
	return logger.New(logger.LoggerOptions{}, term.NewStderr(term.WriteModeBuffered), nil)
}

func entryWithOptions(options ...ast.OptionKind) *ast.Entry {
	var opts []ast.EntryOption
	for _, o := range options {
		opts = append(opts, ast.EntryOption{Kind: o})
	}
	return &ast.Entry{Request: ast.Request{Method: "GET", URL: templateString("http://localhost", sourceInfo(1, 5, 1, 21)), Sections: []ast.Section{{Value: ast.SectionOptions{Value: opts}}}}}
}

func TestEntryOptions(t *testing.T) {
	global := DefaultRunnerOptions()
	global.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("A", "1"))
	entry := entryWithOptions(
		ast.OptionHTTP3{Value: ast.BooleanLiteral{Value: true}},
		ast.OptionNtlm{Value: ast.BooleanLiteral{Value: true}},
		ast.OptionAWSSigV4{Value: templateString("aws:amz:eu-central-1:sts", sourceInfo(0, 0, 0, 0))},
		ast.OptionHeader{Value: templateString("B: 2", sourceInfo(0, 0, 0, 0))},
		ast.OptionLimitRate{Value: ast.NaturalLiteral{Value: ast.U64{Value: 100}}},
		ast.OptionIPv6{Value: ast.BooleanLiteral{Value: false}},
		ast.OptionVariable{Value: ast.VariableDefinition{Name: "x", Value: ast.VariableNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 1}}}}},
		ast.OptionFollowLocationTrusted{Value: ast.BooleanLiteral{Value: true}},
	)
	vars := NewVariableSet()
	o, err := entryOptions(entry, global, vars, testLogger())
	if err != nil {
		t.Fatal(err)
	}
	if o.HttpVersion != httpclient.RequestedHttpVersionHttp3 || o.AllowReuse || !o.Ntlm || o.AwsSigv4 != "aws:amz:eu-central-1:sts" {
		t.Errorf("options = %+v", o)
	}
	if o.MaxRecvSpeed != 100 || o.MaxSendSpeed != 100 || o.IpResolve != httpclient.IpResolveIpV4 || o.FollowLocation != (httpclient.FollowLocation{Follow: true, Forwarding: httpclient.AllHosts}) {
		t.Errorf("options = %+v", o)
	}
	if !reflect.DeepEqual(o.Headers.Values("B"), []string{"2"}) || global.Headers.Len() != 1 {
		t.Errorf("headers = %v, global = %v", o.Headers, global.Headers)
	}
	if v, ok := vars.Get("x"); !ok || !Equal(v.Value, NewInteger(1)) {
		t.Errorf("variable x = %#v", v)
	}

	// The unsupported features are passed to the HTTP client, which fails
	// loudly.
	co := o.clientOptions(logger.VerbosityNone)
	if co.HttpVersion != httpclient.RequestedHttpVersionHttp3 || !co.Ntlm || co.AwsSigv4 == "" {
		t.Errorf("client options = %+v", co)
	}
}

func TestEntryOptionsErrors(t *testing.T) {
	_, err := entryOptions(entryWithOptions(ast.OptionHeader{Value: templateString("foo", sourceInfo(2, 9, 2, 12))}), DefaultRunnerOptions(), NewVariableSet(), testLogger())
	if err == nil || err.Kind != (RunnerErrorInvalidOptionValue{Name: "header", Value: "foo", Message: "missing `:`"}) || err.SourceInfo() != sourceInfo(2, 9, 2, 12) {
		t.Errorf("header: err = %#v", err)
	}
	_, err = entryOptions(entryWithOptions(ast.OptionTLSMax{Value: templateString("1.4", sourceInfo(2, 10, 2, 13))}), DefaultRunnerOptions(), NewVariableSet(), testLogger())
	if err == nil || err.Kind != (RunnerErrorInvalidOptionValue{Name: "tls-max", Value: "1.4", Message: "expecting 1.0, 1.1, 1.2, 1.3"}) {
		t.Errorf("tls-max: err = %#v", err)
	}
}

func TestEntryVerbosity(t *testing.T) {
	entry := entryWithOptions(ast.OptionVerbosity{Value: ast.VerbosityDebug}, ast.OptionVerbose{Value: ast.BooleanLiteral{Value: false}})
	if v, err := entryVerbosity(entry, logger.VerbosityVerbose, NewVariableSet()); err != nil || v != logger.VerbosityNone {
		t.Errorf("got %v, %v", v, err)
	}
	entry = entryWithOptions(ast.OptionVerbosity{Value: ast.VerbosityBrief})
	if v, _ := entryVerbosity(entry, logger.VerbosityNone, NewVariableSet()); v != logger.VerbosityLowVerbose {
		t.Errorf("got %v", v)
	}
}

func TestNonDefaultOptions(t *testing.T) {
	if opts := nonDefaultOptions(DefaultRunnerOptions()); len(opts) != 0 {
		t.Errorf("default: %v", opts)
	}
	o := DefaultRunnerOptions()
	o.Delay = 500 * time.Millisecond
	if opts := nonDefaultOptions(o); !reflect.DeepEqual(opts, []nameValue{{"delay", "500ms"}}) {
		t.Errorf("delay: %v", opts)
	}
}

func TestIsSuccess(t *testing.T) {
	result := func(index int, errors int) EntryResult {
		r := newEntryResult(ast.NewIndex(index), sourceInfo(index, 1, index, 1))
		for range errors {
			r.Errors = append(r.Errors, NewRunnerError(sourceInfo(index, 1, index, 1), RunnerErrorNoQueryResult{}, false))
		}
		return r
	}
	// Retried entries only count for their last try.
	ok := []EntryResult{result(1, 1), result(1, 0), result(2, 0), result(3, 1), result(3, 0)}
	if !isSuccess(ok) {
		t.Error("retried entries should succeed")
	}
	ko := []EntryResult{result(1, 0), result(2, 1), result(3, 0)}
	if isSuccess(ko) {
		t.Error("failing entry")
	}
	r := HurlResult{Entries: ok}
	if errors := r.Errors(); len(errors) != 0 {
		t.Errorf("errors = %v", errors)
	}
	r.Entries = ko
	if errors := r.Errors(); len(errors) != 1 || errors[0].SourceInfo != sourceInfo(2, 1, 2, 1) {
		t.Errorf("errors = %v", errors)
	}
}

func TestBodyCacheHTML(t *testing.T) {
	doc, err := xpath.Parse("<!DOCTYPE html> <html> <body> <h1>My First Heading</h1> <p>My first paragraph.</p> </body> </html>", xpath.HTML)
	if err != nil {
		t.Fatal(err)
	}
	cache := &bodyCache{}
	if cache.xml != nil {
		t.Fatal("cache not empty")
	}
	cache.setXML(doc)
	if v, err := xpathEval(cache.xml, "string(//h1)"); err != nil || !Equal(v, NewString("My First Heading")) {
		t.Errorf("got %v, %v", v, err)
	}
}

func TestIsBinary(t *testing.T) {
	if isBinary([]byte("hello")) || !isBinary([]byte("he\x00llo")) {
		t.Error("isBinary")
	}
	if isBinary(append([]byte(strings.Repeat("a", 2000)), 0)) {
		t.Error("NUL after 2000 bytes")
	}
}
