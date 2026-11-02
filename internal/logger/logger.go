// Package logger is the Hurl logger: debug, verbose, warning and error
// messages on standard error, with secrets redacted. Ported from
// hurl/src/util/logger.rs.
//
// This package is a leaf: methods that take an http.CurlCmd or a
// runner.Value in Rust take a RedactedStringer or an already formatted
// string here.
package logger

import (
	"slices"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/text"
	"github.com/aviddiviner/go-hurl/internal/util"
)

type ErrorFormat int

const (
	ErrorFormatShort ErrorFormat = iota
	ErrorFormatLong
)

// Verbosity is the logger verbosity. The zero value VerbosityNone is Rust's
// None.
type Verbosity int

const (
	VerbosityNone Verbosity = iota
	VerbosityLowVerbose
	VerbosityVerbose
	VerbosityVeryVerbose
)

type LoggerOptions struct {
	Color bool
	// ErrorFormat controls the format of error messages. With
	// ErrorFormatLong, the HTTP request and response that has errors is
	// displayed.
	ErrorFormat ErrorFormat
	Verbosity   Verbosity
}

// RedactedStringer is implemented by values rendered with secrets redacted
// before they are encoded, like httpclient.CurlCmd.
type RedactedStringer interface {
	ToRedactedString(secrets []string) string
}

// Logger is a dedicated logger for a Hurl file. It can display rich parsing
// and runtime errors.
type Logger struct {
	Color       bool
	ErrorFormat ErrorFormat
	Verbosity   Verbosity
	Stderr      *term.Stderr
	secrets     []string
}

func New(options LoggerOptions, stderr *term.Stderr, secrets []string) *Logger {
	return &Logger{
		Color:       options.Color,
		ErrorFormat: options.ErrorFormat,
		Verbosity:   options.Verbosity,
		Stderr:      stderr,
		secrets:     append([]string(nil), secrets...),
	}
}

func (l *Logger) format() text.Format {
	if l.Color {
		return text.FormatANSI
	}
	return text.FormatPlain
}

var (
	styleDebug = text.NewStyle().Blue().Bold()
	styleBold  = text.NewStyle().Bold()
	styleName  = text.NewStyle().Cyan().Bold()
)

// Info prints message, whatever the verbosity.
func (l *Logger) Info(message string) { l.eprintln(message) }

// InfoCurlCmd prints a curl command, whatever the verbosity. Lines start
// with "*".
func (l *Logger) InfoCurlCmd(cmd RedactedStringer) {
	var s text.StyledString
	s.PushWith("*", styleDebug)
	s.Push(" ")
	s.Push("Request can be run with the following curl command:\n")
	s.PushWith("*", styleDebug)
	s.Push(" ")
	s.Push(cmd.ToRedactedString(l.secrets))
	l.eprintln(s.Render(l.format()))
}

// IsLowVerbosity reports whether this logger has no verbosity or a low
// verbosity (debug messages are not displayed).
func (l *Logger) IsLowVerbosity() bool {
	return l.Verbosity == VerbosityNone || l.Verbosity == VerbosityLowVerbose
}

// Debug prints a debug message in verbose and very verbose mode. Lines start
// with "*".
func (l *Logger) Debug(message string) {
	if l.IsLowVerbosity() {
		return
	}
	var s text.StyledString
	s.PushWith("*", styleDebug)
	if message != "" {
		s.Push(" ")
		s.Push(message)
	}
	l.eprintln(s.Render(l.format()))
}

// DebugCurlCmd prints a curl command in verbose and very verbose mode.
// Secrets are redacted before shell encoding the command.
func (l *Logger) DebugCurlCmd(cmd RedactedStringer) {
	c := cmd.ToRedactedString(l.secrets)
	l.Debug("")
	l.Debug("Request can be run with the following curl command:")
	l.Debug(c)
	l.Debug("")
}

// DebugImportant prints a debug message in bold, in verbose and very
// verbose mode.
func (l *Logger) DebugImportant(message string) {
	if l.IsLowVerbosity() {
		return
	}
	var s text.StyledString
	s.PushWith("*", styleDebug)
	if message != "" {
		s.Push(" ")
		s.PushWith(message, styleBold)
	}
	l.eprintln(s.Render(l.format()))
}

// DebugCurl prints a debug message from the HTTP engine, in verbose and very
// verbose mode. Lines start with "**".
func (l *Logger) DebugCurl(message string) {
	if l.IsLowVerbosity() {
		return
	}
	var s text.StyledString
	s.PushWith("**", styleDebug)
	if message != "" {
		s.Push(" ")
		s.Push(message)
	}
	l.eprintln(s.Render(l.format()))
}

// DebugError prints an error (syntax or runtime) when a verbosity is set.
func (l *Logger) DebugError(content string, filename *input.Input, err sourceerr.DisplaySourceError, entrySrcInfo ast.SourceInfo) {
	if l.Verbosity == VerbosityNone {
		return
	}
	name := ""
	if filename != nil {
		name = filename.String()
	}
	message := sourceerr.Render(err, name, content, &entrySrcInfo, sourceerr.OutputFormatTerminal{Color: l.Color})
	for _, line := range sourceerr.Lines(message) {
		l.Debug(line)
	}
}

// DebugHeaderIn prints a response header, when a verbosity is set. Lines
// start with "<".
func (l *Logger) DebugHeaderIn(name, value string) {
	l.debugHeader("< ", name, value)
}

func (l *Logger) DebugHeaderInEnd() {
	if l.Verbosity == VerbosityNone {
		return
	}
	l.eprintln("<")
}

// Deprecated: use DebugHeaderIn.
func (l *Logger) DebugHeadersIn(headers [][2]string) {
	for _, h := range headers {
		l.DebugHeaderIn(h[0], h[1])
	}
}

// DebugHeaderOut prints a request header, when a verbosity is set. Lines
// start with ">".
func (l *Logger) DebugHeaderOut(name, value string) {
	l.debugHeader("> ", name, value)
}

// Deprecated: use DebugHeaderOut.
func (l *Logger) DebugHeadersOut(headers [][2]string) {
	for _, h := range headers {
		l.DebugHeaderOut(h[0], h[1])
	}
}

func (l *Logger) DebugHeaderOutEnd() {
	if l.Verbosity == VerbosityNone {
		return
	}
	l.eprintln(">")
}

func (l *Logger) debugHeader(prefix, name, value string) {
	if l.Verbosity == VerbosityNone {
		return
	}
	var s text.StyledString
	s.Push(prefix)
	s.PushWith(name, styleName)
	s.Push(": ")
	s.Push(value)
	l.eprintln(s.Render(l.format()))
}

// DebugStatusVersionIn prints the response status line, when a verbosity is
// set.
func (l *Logger) DebugStatusVersionIn(line string) {
	if l.Verbosity == VerbosityNone {
		return
	}
	var s text.StyledString
	s.Push("< ")
	s.PushWith(line, text.NewStyle().Green().Bold())
	l.eprintln(s.Render(l.format()))
}

// Warning prints a warning, whatever the verbosity. Lines start with
// "warning:".
func (l *Logger) Warning(message string) {
	var s text.StyledString
	s.PushWith("warning", text.NewStyle().Yellow().Bold())
	s.Push(": ")
	s.PushWith(message, styleBold)
	l.eprintln(s.Render(l.format()))
}

// ErrorRich prints an error message, whatever the verbosity.
func (l *Logger) ErrorRich(message string) {
	var s text.StyledString
	s.PushWith("error", text.NewStyle().Red().Bold())
	s.Push(": ")
	s.Push(message)
	s.Push("\n")
	l.eprintln(s.Render(l.format()))
}

// DebugMethodVersionOut prints the request method and HTTP version line,
// when a verbosity is set.
func (l *Logger) DebugMethodVersionOut(line string) {
	if l.Verbosity == VerbosityNone {
		return
	}
	var s text.StyledString
	s.Push("> ")
	s.PushWith(line, text.NewStyle().Purple().Bold())
	l.eprintln(s.Render(l.format()))
}

// Capture prints a capture in verbose and very verbose mode. value is the
// already formatted captured value.
func (l *Logger) Capture(name, value string) {
	if l.IsLowVerbosity() {
		return
	}
	var s text.StyledString
	s.PushWith("*", styleDebug)
	s.Push(" ")
	s.PushWith(name, text.NewStyle().Yellow().Bold())
	s.Push(": ")
	s.Push(value)
	l.eprintln(s.Render(l.format()))
}

// Secrets returns the secrets redacted by this logger.
func (l *Logger) Secrets() []string { return l.secrets }

// SetSecrets updates the logger secrets. A buffered standard error is
// rewritten so that new secrets are redacted.
func (l *Logger) SetSecrets(secrets []string) {
	if slices.Equal(l.secrets, secrets) {
		return
	}
	l.secrets = secrets
	if l.Stderr.Mode() == term.WriteModeBuffered {
		l.Stderr.SetBuffer(util.Redact(l.Stderr.Buffer(), l.secrets))
	}
}

func (l *Logger) eprintln(message string) {
	if len(l.secrets) > 0 {
		message = util.Redact(message, l.secrets)
	}
	l.Stderr.Eprintln(message)
}
