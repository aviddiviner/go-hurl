package cli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestDurationFromStrError(t *testing.T) {
	tests := []struct{ value, want string }{
		{"", "Invalid duration"},
		{"s", "Invalid duration"},
		{"10s10", "Invalid duration"},
		{"10mm", "Invalid duration unit mm"},
	}
	for _, tt := range tests {
		_, err := durationFromStr(tt.value, ast.DurationUnitMilliSecond)
		if err != (OptionsErrorError{Message: tt.want}) {
			t.Errorf("durationFromStr(%q) error = %v, want %q", tt.value, err, tt.want)
		}
	}
}

func TestDurationFromStr(t *testing.T) {
	tests := []struct {
		value string
		unit  ast.DurationUnit
		want  time.Duration
	}{
		{"10", ast.DurationUnitMilliSecond, 10 * time.Millisecond},
		{"10s", ast.DurationUnitMilliSecond, 10 * time.Second},
		{"10000ms", ast.DurationUnitSecond, 10000 * time.Millisecond},
		{"5m", ast.DurationUnitSecond, 5 * time.Minute},
		{"3h", ast.DurationUnitMilliSecond, 3 * time.Hour},
	}
	for _, tt := range tests {
		got, err := durationFromStr(tt.value, tt.unit)
		if err != nil || got != tt.want {
			t.Errorf("durationFromStr(%q) = %v, %v, want %v", tt.value, got, err, tt.want)
		}
	}
}

func TestParseVariable(t *testing.T) {
	tests := []struct {
		s     string
		kind  typeKind
		name  string
		value runner.Value
	}{
		{"name=Jennifer", typeInferred, "name", runner.NewString("Jennifer")},
		{"female=true", typeInferred, "female", runner.NewBool(true)},
		{"age=30", typeInferred, "age", runner.NewInteger(30)},
		{"height=1.7", typeInferred, "height", runner.NewFloat(1.7)},
		{`id="123"`, typeInferred, "id", runner.NewString("123")},
		{"id=9223372036854775808", typeInferred, "id", runner.ValueNumber{Value: runner.NumberBigInteger{Value: "9223372036854775808"}}},
		{"a_null=null", typeInferred, "a_null", runner.ValueNull{}},
		{"a_null=null", typeString, "a_null", runner.NewString("null")},
	}
	for _, tt := range tests {
		name, value, err := parseVariable(tt.s, tt.kind)
		if err != nil || name != tt.name || !reflect.DeepEqual(value, tt.value) {
			t.Errorf("parseVariable(%q) = %q, %#v, %v", tt.s, name, value, err)
		}
	}
	_, _, err := parseVariable("name", typeInferred)
	if err != (OptionsErrorError{Message: "Missing value for variable name!"}) {
		t.Errorf("got %v", err)
	}
}

func TestParseVariableValue(t *testing.T) {
	tests := []struct {
		s    string
		kind typeKind
		want runner.Value
	}{
		{"Jennifer", typeInferred, runner.NewString("Jennifer")},
		{"true", typeInferred, runner.NewBool(true)},
		{"30", typeInferred, runner.NewInteger(30)},
		{"30", typeString, runner.NewString("30")},
		{"1.7", typeInferred, runner.NewFloat(1.7)},
		{"1.7", typeString, runner.NewString("1.7")},
		{"1.0", typeInferred, runner.NewFloat(1.0)},
		{"-1.0", typeInferred, runner.NewFloat(-1.0)},
		{`"123"`, typeInferred, runner.NewString("123")},
		{`"123"`, typeString, runner.NewString(`"123"`)},
		{"null", typeInferred, runner.ValueNull{}},
	}
	for _, tt := range tests {
		got, err := parseVariableValue(tt.s, tt.kind)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("parseVariableValue(%q) = %#v, %v, want %#v", tt.s, got, err, tt.want)
		}
	}
	_, err := parseVariableValue(`"123`, typeInferred)
	if err != (OptionsErrorError{Message: "Value should end with a double quote"}) {
		t.Errorf("got %v", err)
	}
}

type namedValue struct {
	name  string
	value runner.Value
}

func readAllVariables(t *testing.T, content string, kind typeKind) []namedValue {
	t.Helper()
	path := filepath.Join(t.TempDir(), "file.env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var vars []namedValue
	err := readVariablesFile(path, kind, func(name string, value runner.Value) error {
		vars = append(vars, namedValue{name, value})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return vars
}

func TestVariablesFileInferred(t *testing.T) {
	got := readAllVariables(t, "foo=bar\nflag=true\nid=123\n", typeInferred)
	want := []namedValue{
		{"foo", runner.NewString("bar")},
		{"flag", runner.NewBool(true)},
		{"id", runner.NewInteger(123)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestVariablesFileString(t *testing.T) {
	got := readAllVariables(t, "foo=bar\n# With some comments\n# bla bla bla\nflag=true\nid=123\n", typeString)
	want := []namedValue{
		{"foo", runner.NewString("bar")},
		{"flag", runner.NewString("true")},
		{"id", runner.NewString("123")},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestEnvVarsColor(t *testing.T) {
	if _, ok := NewEnvVars(map[string]string{"A": "B"}).color(); ok {
		t.Error("expected no color env var")
	}
	tests := []struct {
		name, value string
		want, ok    bool
	}{
		{"HURL_COLOR", "0", false, true},
		{"HURL_COLOR", "1", true, true},
		{"HURL_COLOR", "true", true, true},
		{"HURL_COLOR", "TRUE", true, true},
		{"HURL_COLOR", "false", false, true},
		{"HURL_COLOR", "FALSE", false, true},
	}
	for _, tt := range tests {
		got, ok := NewEnvVars(map[string]string{tt.name: tt.value}).color()
		if got != tt.want || ok != tt.ok {
			t.Errorf("env var %s=%s: got %v, %v", tt.name, tt.value, got, ok)
		}
	}
}

func TestEnvVarsNoColor(t *testing.T) {
	tests := []struct {
		name, value string
		want        bool
	}{
		{"NO_COLOR", "0", true},
		{"NO_COLOR", "1", true},
		{"NO_COLOR", "true", true},
		{"NO_COLOR", "TRUE", true},
		{"NO_COLOR", "false", true},
		{"NO_COLOR", "FALSE", true},
		{"HURL_NO_COLOR", "0", false},
		{"HURL_NO_COLOR", "1", true},
		{"HURL_NO_COLOR", "true", true},
		{"HURL_NO_COLOR", "TRUE", true},
		{"HURL_NO_COLOR", "false", false},
		{"HURL_NO_COLOR", "FALSE", false},
	}
	for _, tt := range tests {
		got, ok := NewEnvVars(map[string]string{tt.name: tt.value}).noColor()
		if got != tt.want || !ok {
			t.Errorf("env var %s=%s: got %v, %v", tt.name, tt.value, got, ok)
		}
	}
}

func TestEnvVarsEmptyVariablesSecrets(t *testing.T) {
	env := NewEnvVars(map[string]string{"FOO": "xxx", "BAR": "yyy", "BAZ": "yyy"})
	if len(env.variables()) != 0 || len(env.secrets()) != 0 {
		t.Error("expected no variables and no secrets")
	}
}

func TestEnvVarsVariables(t *testing.T) {
	env := NewEnvVars(map[string]string{
		"FOO":               "xxx",
		"BAR":               "yyy",
		"BAZ":               "yyy",
		"HURL_VARIABLE_foo": "true",
		"HURL_VARIABLE_id":  "1234",
		"HURL_VARIABLE":     "1234",
		"HURL_VARIABLE_":    "abcd",
		"HURL_VARIABLE_FOO": "def",
		"HURL_COLOR":        "1",
		"HURL_NO_COLOR":     "1",
	})
	want := map[string]string{"foo": "true", "id": "1234", "FOO": "def"}
	if got := env.variables(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
	if len(env.secrets()) != 0 {
		t.Error("expected no secrets")
	}
}

func TestOptionsVariablesOverrideByEnvVars(t *testing.T) {
	o := DefaultCliOptions()
	o.Variables = map[string]runner.Value{"var1": runner.NewString("zzz"), "foo": runner.NewString("FOO")}
	env := NewEnvVars(map[string]string{
		"FOO":               "xxx",
		"HURL_VARIABLE_foo": "48",
		"HURL_VARIABLE_bar": "BAR",
		"HURL_baz":          "abcd",
		"NOT_A_VARIABLE":    "bar",
	})
	if err := parseEnvVars(env, o); err != nil {
		t.Fatal(err)
	}
	want := map[string]runner.Value{
		"foo":  runner.NewInteger(48),
		"var1": runner.NewString("zzz"),
		"bar":  runner.NewString("BAR"),
	}
	if !reflect.DeepEqual(o.Variables, want) {
		t.Errorf("got %v", o.Variables)
	}
}

func TestOptionsSecretsOverrideByEnvVars(t *testing.T) {
	o := DefaultCliOptions()
	o.Secrets = map[string]string{"secret1": "SECRET1"}
	env := NewEnvVars(map[string]string{
		"QUX":                 "qux",
		"HURL_SECRET_secret2": "SECRET2",
		"HURL_VARIABLE_bar":   "BAR",
		"HURL_SECRET_secret3": "SECRET3",
	})
	if err := parseEnvVars(env, o); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(o.Variables, map[string]runner.Value{"bar": runner.NewString("BAR")}) {
		t.Errorf("got variables %v", o.Variables)
	}
	want := map[string]string{"secret1": "SECRET1", "secret2": "SECRET2", "secret3": "SECRET3"}
	if !reflect.DeepEqual(o.Secrets, want) {
		t.Errorf("got secrets %v", o.Secrets)
	}
}

func TestEnvVarsErrors(t *testing.T) {
	tests := []struct{ name, value, want string }{
		{"HURL_MAX_REDIRS", "-5", "-5 is not in -1..=2147483647 (HURL_MAX_REDIRS environment variable)"},
		{"HURL_MAX_REDIRS", "a", "invalid digit found in string (HURL_MAX_REDIRS environment variable)"},
		{"HURL_JOBS", "-1", "invalid digit found in string (HURL_JOBS environment variable)"},
		{"HURL_DELAY", "x", "Invalid duration (HURL_DELAY environment variable)"},
		{"HURL_TLS_MAX", "2", "expecting 1.0, 1.1, 1.2 or 1.3 (HURL_TLS_MAX environment variable)"},
		{"HURL_HEADER", "foo", "Invalid header <foo>, missing `:` (HURL_HEADER environment variable)"},
	}
	for _, tt := range tests {
		err := parseEnvVars(NewEnvVars(map[string]string{tt.name: tt.value}), DefaultCliOptions())
		if err != (OptionsErrorError{Message: tt.want}) {
			t.Errorf("%s=%s: got %v, want %q", tt.name, tt.value, err, tt.want)
		}
	}
}

func TestSkipWhitespaceAndComments(t *testing.T) {
	r := reader.New("# This is a comment\n   # Another comment\n   option=value\n")
	skipWhitespaceAndComments(r)
	if got := r.Cursor().Pos; got != (reader.Pos{Line: 3, Column: 4}) {
		t.Errorf("got %v", got)
	}
}

func TestParseValueSeparator(t *testing.T) {
	r := reader.New("  x")
	if err := parseValueSeparator(r); err != nil || r.Cursor().Pos != (reader.Pos{Line: 1, Column: 3}) {
		t.Errorf("got %v, %v", r.Cursor().Pos, err)
	}
	r = reader.New("= x")
	if err := parseValueSeparator(r); err != nil || r.Cursor().Pos != (reader.Pos{Line: 1, Column: 2}) {
		t.Errorf("got %v, %v", r.Cursor().Pos, err)
	}
	r = reader.New("x")
	err := parseValueSeparator(r)
	if err == nil || *err != *newConfigFileError(reader.Pos{Line: 1, Column: 1}, "Expecting a value using space or '=' separator") {
		t.Errorf("got %v", err)
	}
}

func TestExpectNoValue(t *testing.T) {
	tests := []struct {
		s    string
		want reader.Pos
	}{
		{"\n", reader.Pos{Line: 2, Column: 1}},
		{"  \n", reader.Pos{Line: 2, Column: 1}},
		{"  ", reader.Pos{Line: 1, Column: 3}},
		{" # Comment\n", reader.Pos{Line: 2, Column: 1}},
	}
	for _, tt := range tests {
		r := reader.New(tt.s)
		if err := expectNoValue(r); err != nil || r.Cursor().Pos != tt.want {
			t.Errorf("expectNoValue(%q): got %v, %v", tt.s, r.Cursor().Pos, err)
		}
	}
	err := expectNoValue(reader.New("x\n"))
	if err == nil || *err != *newConfigFileError(reader.Pos{Line: 1, Column: 1}, "Not expecting a value for this option") {
		t.Errorf("got %v", err)
	}
}

func TestParseValue(t *testing.T) {
	tests := []struct {
		s     string
		want  string
		index int
	}{
		{"\n", "", 1},
		{"1\n", "1", 2},
		{"\"Hello\"\n", "Hello", 8},
		{"\"Hello\nBob!\"\n", "Hello\nBob!", 13},
		{"\"Hello\"   \n", "Hello", 11},
		{"\"Hello\" # this is a comment\n", "Hello", 28},
	}
	for _, tt := range tests {
		r := reader.New(tt.s)
		got, err := parseValue(r)
		if err != nil || got != tt.want || r.Cursor().Index != tt.index {
			t.Errorf("parseValue(%q) = %q, %v (index %d)", tt.s, got, err, r.Cursor().Index)
		}
	}

	r := reader.New("\"1\n")
	_, err := parseValue(r)
	if err == nil || *err != *newConfigFileError(reader.Pos{Line: 2, Column: 1}, "Missing closing quote") || r.Cursor().Index != 3 {
		t.Errorf("got %v", err)
	}
	_, err = parseValue(reader.New("\"Hello\" world\n"))
	if err == nil || *err != *newConfigFileError(reader.Pos{Line: 1, Column: 9}, "characters after the closing quote") {
		t.Errorf("got %v", err)
	}
}

func parseOneOption(t *testing.T, content string) (*CliOptions, *reader.Reader) {
	t.Helper()
	r := reader.New(content)
	o := DefaultCliOptions()
	if err := parseConfigOption(r, o); err != nil {
		t.Fatalf("parseConfigOption(%q): %v", content, err)
	}
	return o, r
}

func TestParseConfig(t *testing.T) {
	o := DefaultCliOptions()
	if err := parseConfig("# ignore\n\n--verbose\n", o); err != nil {
		t.Fatal(err)
	}
	if o.ColorStdout != BoolOptAuto || o.ColorStderr != BoolOptAuto || o.Verbosity != VerbosityVerbose {
		t.Errorf("got %+v", o)
	}
	err := parseConfig("--user-agent=\"Mozilla/5.0 A\" --verbose\n", DefaultCliOptions())
	if err == nil || err.message != "characters after the closing quote" {
		t.Errorf("got %v", err)
	}
}

func TestParseConfigOptions(t *testing.T) {
	next := reader.Pos{Line: 2, Column: 1}
	tests := []struct {
		content string
		check   func(o *CliOptions) bool
		pos     reader.Pos
	}{
		{"\n\n--verbose\n", func(o *CliOptions) bool { return o.Verbosity == VerbosityVerbose }, reader.Pos{Line: 4, Column: 1}},
		{"\n\n--very-verbose\n", func(o *CliOptions) bool { return o.Verbosity == VerbosityDebug }, reader.Pos{Line: 4, Column: 1}},
		{"\n\n--verbosity=brief\n", func(o *CliOptions) bool { return o.Verbosity == VerbosityBrief }, reader.Pos{Line: 4, Column: 1}},
		{"# ignore\n\n", func(o *CliOptions) bool { return true }, reader.Pos{Line: 3, Column: 1}},
		{"--header=header1:value1\n--verbose\n", func(o *CliOptions) bool { return reflect.DeepEqual(o.Headers, []string{"header1:value1"}) }, next},
		{"--header header2:value2\n--verbose\n", func(o *CliOptions) bool { return reflect.DeepEqual(o.Headers, []string{"header2:value2"}) }, next},
		{"--header --test:1\n", func(o *CliOptions) bool { return reflect.DeepEqual(o.Headers, []string{"--test:1"}) }, next},
		{"--variable=hobby=tennis\n", func(o *CliOptions) bool {
			return reflect.DeepEqual(o.Variables["hobby"], runner.NewString("tennis"))
		}, next},
		{"--test\n", func(o *CliOptions) bool { return o.Test }, next},
		{"--secret=password=secret\n", func(o *CliOptions) bool { return o.Secrets["password"] == "secret" }, next},
		{"--user-agent=\"Mozilla/5.0 A\"", func(o *CliOptions) bool { return *o.UserAgent == "Mozilla/5.0 A" }, reader.Pos{Line: 1, Column: 29}},
		{"--user=bob@email.com:secret\n", func(o *CliOptions) bool { return *o.User == "bob@email.com:secret" }, next},
		{"--delay=1s\n", func(o *CliOptions) bool { return o.Delay == time.Second }, next},
		{"--connect-timeout=500ms\n", func(o *CliOptions) bool { return o.ConnectTimeout == 500*time.Millisecond }, next},
		{"--jobs=4\n", func(o *CliOptions) bool { return o.Jobs == 4 }, next},
		{"--max-filesize=255\n", func(o *CliOptions) bool { return *o.MaxFilesize == 255 }, next},
		{"--limit-rate=2000000\n", func(o *CliOptions) bool { return *o.LimitRate == 2_000_000 }, next},
		{"--max-time=30s\n", func(o *CliOptions) bool { return o.Timeout == 30*time.Second }, next},
		{"--color\n", func(o *CliOptions) bool { return o.ColorStdout.Get() && o.ColorStderr.Get() }, next},
		{"--no-color\n", func(o *CliOptions) bool { return o.ColorStdout == BoolOptFalse && o.ColorStderr == BoolOptFalse }, next},
		{"--insecure\n", func(o *CliOptions) bool { return o.Insecure }, next},
		{"--continue-on-error\n", func(o *CliOptions) bool { return o.ContinueOnError }, next},
		{"--error-format=long\n", func(o *CliOptions) bool { return o.ErrorFormat == ErrorFormatLong }, next},
		{"--http1.1\n", func(o *CliOptions) bool { return o.HttpVersion == HttpVersionV11 }, next},
		{"--http1.0\n", func(o *CliOptions) bool { return o.HttpVersion == HttpVersionV10 }, next},
		{"--http2\n", func(o *CliOptions) bool { return o.HttpVersion == HttpVersionV2 }, next},
		{"--http3\n", func(o *CliOptions) bool { return o.HttpVersion == HttpVersionV3 }, next},
		{"--location\n", func(o *CliOptions) bool { return o.FollowLocation && !o.FollowLocationTrusted }, next},
		{"--location-trusted\n", func(o *CliOptions) bool { return o.FollowLocation && o.FollowLocationTrusted }, next},
		{"--compressed\n", func(o *CliOptions) bool { return o.Compressed }, next},
		{"--ipv6\n", func(o *CliOptions) bool { return o.IpResolve == IpResolveIpV6 }, next},
		{"--fail-with-body\n", func(o *CliOptions) bool { return o.FailWithBody }, next},
		{"--no-output\n", func(o *CliOptions) bool { return o.OutputType == OutputTypeNoOutput }, next},
		{"--no-assert\n", func(o *CliOptions) bool { return o.NoAssert }, next},
		{"--no-cookie-store\n", func(o *CliOptions) bool { return o.NoCookieStore }, next},
		{"--no-header=user-agent\n", func(o *CliOptions) bool { return reflect.DeepEqual(o.NoHeaders, []string{"user-agent"}) }, next},
		{"--no-proxy=127.0.0.1\n", func(o *CliOptions) bool { return *o.NoProxy == "127.0.0.1" }, next},
		{"--proxy=127.0.0.1:3128\n", func(o *CliOptions) bool { return o.Proxy == "127.0.0.1:3128" }, next},
		{"--proxy-header=From-Proxy:Hello\n", func(o *CliOptions) bool {
			return reflect.DeepEqual(o.ProxyHeaders, []string{"From-Proxy:Hello"})
		}, next},
		{"--no-pretty\n", func(o *CliOptions) bool { return o.Pretty == pretty.None }, next},
		{"--no-jsonpath-coercion\n", func(o *CliOptions) bool { return o.NoJSONPathCoercion }, next},
		{"--retry=10\n", func(o *CliOptions) bool { return o.Retry == ast.CountFinite{Value: 10} }, next},
		{"--retry-interval=100ms\n", func(o *CliOptions) bool { return o.RetryInterval == 100*time.Millisecond }, next},
		{"--pretty\n", func(o *CliOptions) bool { return o.Pretty == pretty.Force }, next},
	}
	for _, tt := range tests {
		o, r := parseOneOption(t, tt.content)
		if !tt.check(o) {
			t.Errorf("parseConfigOption(%q): unexpected options %+v", tt.content, o)
		}
		if r.Cursor().Pos != tt.pos {
			t.Errorf("parseConfigOption(%q): pos %v, want %v", tt.content, r.Cursor().Pos, tt.pos)
		}
	}
}

func TestParseConfigOptionErrors(t *testing.T) {
	tests := []struct {
		content string
		pos     reader.Pos
		message string
	}{
		{"--verbosity=invalid\n", reader.Pos{Line: 1, Column: 13}, "Option --verbosity requires one of the following values: brief, verbose, debug"},
		{"--jobs=abc\n", reader.Pos{Line: 1, Column: 8}, "Option --jobs requires an integer value"},
		{"--jobs=0\n", reader.Pos{Line: 1, Column: 8}, "Option --jobs requires an integer value >= 1"},
		{"--max-filesize=abc\n", reader.Pos{Line: 1, Column: 16}, "Option --max-filesize requires an integer value"},
		{"--max-time=abc\n", reader.Pos{Line: 1, Column: 12}, "Option --max-time has an invalid duration"},
		{"--error-format=foo\n", reader.Pos{Line: 1, Column: 16}, "Option --error-format requires one of the following values: short, long"},
		{"verbose\n", reader.Pos{Line: 1, Column: 1}, "Expecting an option starting with --"},
		{"--xxx", reader.Pos{Line: 1, Column: 1}, "Unknown option <--xxx>"},
		{"--verbosexxx", reader.Pos{Line: 1, Column: 1}, "Unknown option <--verbosexxx>"},
		{"--header\n", reader.Pos{Line: 1, Column: 9}, "Expecting a value using space or '=' separator"},
		{"--verbose=1\n", reader.Pos{Line: 1, Column: 10}, "Not expecting a value for this option"},
		{"--max-redirs=a\n", reader.Pos{Line: 1, Column: 14}, "Option --max-redirs requires an integer value"},
		{"--delay=abc\n", reader.Pos{Line: 1, Column: 9}, "Option --delay has an invalid duration"},
		{"--limit-rate=abc\n", reader.Pos{Line: 1, Column: 14}, "Option --limit-rate requires an integer value"},
		{"--retry=abc\n", reader.Pos{Line: 1, Column: 9}, "Option --retry requires an integer value"},
		{"--retry=-2\n", reader.Pos{Line: 1, Column: 9}, "Option --retry requires an integer value >= 1"},
		{"--retry-interval=abc\n", reader.Pos{Line: 1, Column: 18}, "Option --retry-interval has an invalid duration"},
	}
	for _, tt := range tests {
		err := parseConfigOption(reader.New(tt.content), DefaultCliOptions())
		if err == nil || err.pos != tt.pos || err.message != tt.message {
			t.Errorf("parseConfigOption(%q) = %v, want %v %q", tt.content, err, tt.pos, tt.message)
		}
	}
}
