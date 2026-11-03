package cli

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/reader"
)

// configFileError is an error at a position of the config file.
type configFileError struct {
	pos     reader.Pos
	message string
}

func (e *configFileError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.pos.Line, e.pos.Column, e.message)
}

func newConfigFileError(pos reader.Pos, message string) *configFileError {
	return &configFileError{pos: pos, message: message}
}

// parseConfigFile updates options from the config file at path, if it
// exists (see docs/spec/options/config_file.md).
func parseConfigFile(path string, o *CliOptions) error {
	if path == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return optionsErrorf("Failed to read config file %s: %s", path, rustIOError(err))
	}
	if err := parseConfig(string(b), o); err != nil {
		return optionsErrorf("%s:%d:%d: %s", path, err.pos.Line, err.pos.Column, err.message)
	}
	return nil
}

func parseConfig(content string, o *CliOptions) *configFileError {
	r := reader.New(content)
	for !r.IsEOF() {
		if err := parseConfigOption(r, o); err != nil {
			return err
		}
	}
	return nil
}

type configOption func(r *reader.Reader, o *CliOptions, optionPos reader.Pos) *configFileError

// configFlag is an option without value.
func configFlag(set func(o *CliOptions)) configOption {
	return func(r *reader.Reader, o *CliOptions, _ reader.Pos) *configFileError {
		if err := expectNoValue(r); err != nil {
			return err
		}
		set(o)
		return nil
	}
}

// configValue is an option with a value, set may fail with a message for
// the value position.
func configValue(set func(o *CliOptions, value string) string) configOption {
	return func(r *reader.Reader, o *CliOptions, _ reader.Pos) *configFileError {
		if err := parseValueSeparator(r); err != nil {
			return err
		}
		valuePos := r.Cursor().Pos
		value, err := parseValue(r)
		if err != nil {
			return err
		}
		if msg := set(o, value); msg != "" {
			return newConfigFileError(valuePos, msg)
		}
		return nil
	}
}

// configRequiredValue is an option with a non empty value.
func configRequiredValue(name string, set func(o *CliOptions, value string)) configOption {
	return func(r *reader.Reader, o *CliOptions, optionPos reader.Pos) *configFileError {
		if err := parseValueSeparator(r); err != nil {
			return err
		}
		value, err := parseValue(r)
		if err != nil {
			return err
		}
		if value == "" {
			return newConfigFileError(optionPos, "Option --"+name+" requires a value")
		}
		set(o, value)
		return nil
	}
}

func configDuration(name string, unit ast.DurationUnit, set func(o *CliOptions, d time.Duration)) configOption {
	return configValue(func(o *CliOptions, value string) string {
		d, err := durationFromStr(value, unit)
		if err != nil {
			return "Option --" + name + " has an invalid duration"
		}
		set(o, d)
		return ""
	})
}

var configOptions map[string]configOption

func init() {
	configOptions = map[string]configOption{
		"color":      configFlag(func(o *CliOptions) { o.ColorStdout, o.ColorStderr = BoolOptTrue, BoolOptTrue }),
		"compressed": configFlag(func(o *CliOptions) { o.Compressed = true }),
		"connect-timeout": configDuration("connect-timeout", ast.DurationUnitSecond, func(o *CliOptions, d time.Duration) {
			o.ConnectTimeout = d
		}),
		"continue-on-error": configFlag(func(o *CliOptions) { o.ContinueOnError = true }),
		"delay": configDuration("delay", ast.DurationUnitMilliSecond, func(o *CliOptions, d time.Duration) {
			o.Delay = d
		}),
		"error-format": configValue(func(o *CliOptions, value string) string {
			f, err := parseErrorFormat(value)
			if err != nil {
				return "Option --error-format requires one of the following values: short, long"
			}
			o.ErrorFormat = f
			return ""
		}),
		"fail-with-body": configFlag(func(o *CliOptions) { o.FailWithBody = true }),
		"header":         configRequiredValue("header", func(o *CliOptions, v string) { o.Headers = append(o.Headers, v) }),
		"http1.0":        configFlag(func(o *CliOptions) { o.HttpVersion = HttpVersionV10 }),
		"http1.1":        configFlag(func(o *CliOptions) { o.HttpVersion = HttpVersionV11 }),
		"http2":          configFlag(func(o *CliOptions) { o.HttpVersion = HttpVersionV2 }),
		"http3":          configFlag(func(o *CliOptions) { o.HttpVersion = HttpVersionV3 }),
		"insecure":       configFlag(func(o *CliOptions) { o.Insecure = true }),
		"ipv6":           configFlag(func(o *CliOptions) { o.IpResolve = IpResolveIpV6 }),
		"jobs": configValue(func(o *CliOptions, value string) string {
			n, err := rustParseInt(value, 0, math.MaxInt64)
			if err != nil {
				return "Option --jobs requires an integer value"
			}
			if n < 1 {
				return "Option --jobs requires an integer value >= 1"
			}
			o.Jobs = int(n)
			return ""
		}),
		"limit-rate": configValue(func(o *CliOptions, value string) string {
			n, err := rustParseUint64(value)
			if err != nil {
				return "Option --limit-rate requires an integer value"
			}
			o.LimitRate = &n
			return ""
		}),
		"location": configFlag(func(o *CliOptions) { o.FollowLocation = true }),
		"location-trusted": configFlag(func(o *CliOptions) {
			o.FollowLocation = true
			o.FollowLocationTrusted = true
		}),
		"max-filesize": configValue(func(o *CliOptions, value string) string {
			n, err := rustParseUint64(value)
			if err != nil {
				return "Option --max-filesize requires an integer value"
			}
			o.MaxFilesize = &n
			return ""
		}),
		"max-redirs": configValue(func(o *CliOptions, value string) string {
			n, err := rustParseInt(value, math.MinInt32, math.MaxInt32)
			if err != nil {
				return "Option --max-redirs requires an integer value"
			}
			c, err := ast.CountFromInt32(int32(n))
			if err != nil {
				return "Option --max-redirs requires an integer value >= -1"
			}
			o.MaxRedirect = c
			return ""
		}),
		"max-time": configDuration("max-time", ast.DurationUnitSecond, func(o *CliOptions, d time.Duration) {
			o.Timeout = d
		}),
		"no-assert":            configFlag(func(o *CliOptions) { o.NoAssert = true }),
		"no-color":             configFlag(func(o *CliOptions) { o.ColorStdout, o.ColorStderr = BoolOptFalse, BoolOptFalse }),
		"no-cookie-store":      configFlag(func(o *CliOptions) { o.NoCookieStore = true }),
		"no-header":            configRequiredValue("no-header", func(o *CliOptions, v string) { o.NoHeaders = append(o.NoHeaders, v) }),
		"no-jsonpath-coercion": configFlag(func(o *CliOptions) { o.NoJSONPathCoercion = true }),
		"no-output":            configFlag(func(o *CliOptions) { o.OutputType = OutputTypeNoOutput }),
		"no-pretty":            configFlag(func(o *CliOptions) { o.Pretty = pretty.None }),
		"no-proxy":             configRequiredValue("no-proxy", func(o *CliOptions, v string) { o.NoProxy = &v }),
		// As in Hurl 8.1.0, --no-progress-bar enables the progress bar.
		"no-progress-bar": configFlag(func(o *CliOptions) { o.ProgressBar = BoolOptTrue }),
		"pretty":          configFlag(func(o *CliOptions) { o.Pretty = pretty.Force }),
		"proxy":           configRequiredValue("proxy", func(o *CliOptions, v string) { o.Proxy = v }),
		"proxy-header":    configRequiredValue("proxy-header", func(o *CliOptions, v string) { o.ProxyHeaders = append(o.ProxyHeaders, v) }),
		"retry": configValue(func(o *CliOptions, value string) string {
			n, err := rustParseInt(value, math.MinInt32, math.MaxInt32)
			if err != nil {
				return "Option --retry requires an integer value"
			}
			c, err := ast.CountFromInt32(int32(n))
			if err != nil {
				return "Option --retry requires an integer value >= 1"
			}
			o.Retry = c
			return ""
		}),
		"retry-interval": configDuration("retry-interval", ast.DurationUnitMilliSecond, func(o *CliOptions, d time.Duration) {
			o.RetryInterval = d
		}),
		"secret": configValue(func(o *CliOptions, value string) string {
			name, v, err := parseVariable(value, typeString)
			if err != nil {
				return err.Error()
			}
			if err := addSecret(o.Secrets, name, v); err != nil {
				return err.Error()
			}
			return ""
		}),
		"test": configFlag(func(o *CliOptions) { o.Test = true }),
		"tls-max": configValue(func(o *CliOptions, value string) string {
			t, msg := parseTlsVersion(value)
			if msg != "" {
				return "Option --tls-max has an invalid value, " + msg
			}
			o.TlsMax = t
			return ""
		}),
		"tls-min": configValue(func(o *CliOptions, value string) string {
			t, msg := parseTlsVersion(value)
			if msg != "" {
				return "Option --tls-min has an invalid value, " + msg
			}
			o.TlsMin = t
			return ""
		}),
		"user": configRequiredValue("user", func(o *CliOptions, v string) { o.User = &v }),
		"user-agent": configValue(func(o *CliOptions, value string) string {
			o.UserAgent = &value
			return ""
		}),
		"variable": configValue(func(o *CliOptions, value string) string {
			name, v, err := parseVariable(value, typeInferred)
			if err != nil {
				return err.Error()
			}
			o.Variables[name] = v
			return ""
		}),
		"verbose":      configFlag(func(o *CliOptions) { o.Verbosity = VerbosityVerbose }),
		"very-verbose": configFlag(func(o *CliOptions) { o.Verbosity = VerbosityDebug }),
		"verbosity": configValue(func(o *CliOptions, value string) string {
			v, err := parseVerbosity(value)
			if err != nil {
				return "Option --verbosity requires one of the following values: brief, verbose, debug"
			}
			o.Verbosity = v
			return ""
		}),
	}
}

// parseConfigOption parses and applies a single option. Empty lines and
// lines starting with # are ignored.
func parseConfigOption(r *reader.Reader, o *CliOptions) *configFileError {
	skipWhitespaceAndComments(r)
	if r.IsEOF() {
		return nil
	}
	optionPos := r.Cursor().Pos
	if r.ReadN(2) != "--" {
		return newConfigFileError(optionPos, "Expecting an option starting with --")
	}
	name := r.ReadWhile(func(c rune) bool {
		return unicode.IsLetter(c) || unicode.IsNumber(c) || c == '-' || c == '_' || c == '.'
	})
	option, ok := configOptions[name]
	if !ok {
		return newConfigFileError(optionPos, "Unknown option <--"+name+">")
	}
	return option(r, o, optionPos)
}

// skipWhitespaceAndComments skips whitespaces and comment lines.
func skipWhitespaceAndComments(r *reader.Reader) {
	for {
		r.ReadWhile(unicode.IsSpace)
		if r.IsEOF() {
			return
		}
		if c, _ := r.Peek(); c != '#' {
			return
		}
		skipComment(r)
	}
}

// skipComment skips to the end of the line, consuming the newline.
func skipComment(r *reader.Reader) {
	r.ReadWhile(func(c rune) bool { return c != '\n' })
	if c, ok := r.Peek(); ok && c == '\n' {
		r.Read()
	}
}

func isBlank(c rune) bool { return c == ' ' || c == '\t' }

// expectNoValue expects the end of the line after a flag, skipping trailing
// whitespaces and comments.
func expectNoValue(r *reader.Reader) *configFileError {
	for {
		r.ReadWhile(isBlank)
		if r.IsEOF() {
			return nil
		}
		c, _ := r.Peek()
		switch c {
		case '#':
			skipComment(r)
		case '\n':
			r.Read()
			return nil
		default:
			return newConfigFileError(r.Cursor().Pos, "Not expecting a value for this option")
		}
	}
}

// parseValue parses an option value on the same line as the option, quoted
// or not.
func parseValue(r *reader.Reader) (string, *configFileError) {
	if c, ok := r.Peek(); !ok || c != '"' {
		s := strings.TrimSpace(r.ReadWhile(func(c rune) bool { return c != '\n' }))
		if c, ok := r.Peek(); ok && c == '\n' {
			r.Read()
		}
		return s, nil
	}
	r.Read()
	value := r.ReadWhile(func(c rune) bool { return c != '"' })
	if c, ok := r.Peek(); !ok || c != '"' {
		return "", newConfigFileError(r.Cursor().Pos, "Missing closing quote")
	}
	r.Read()
	// Allow trailing spaces and an inline comment, but nothing else on the
	// same line.
	r.ReadWhile(isBlank)
	if r.IsEOF() {
		return strings.TrimSpace(value), nil
	}
	switch c, _ := r.Peek(); c {
	case '#':
		skipComment(r)
	case '\n':
		r.Read()
	default:
		return "", newConfigFileError(r.Cursor().Pos, "characters after the closing quote")
	}
	return strings.TrimSpace(value), nil
}

// parseValueSeparator parses the separator between an option and its
// value: one or more spaces, or '='.
func parseValueSeparator(r *reader.Reader) *configFileError {
	if r.ReadWhile(isBlank) != "" {
		return nil
	}
	if c, ok := r.Peek(); ok && c == '=' {
		r.Read()
		return nil
	}
	return newConfigFileError(r.Cursor().Pos, "Expecting a value using space or '=' separator")
}
