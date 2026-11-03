package cli

import (
	"fmt"

	"github.com/aviddiviner/go-hurl/internal/text"
)

// OptionsError is an error (or an informational exit) from parsing the
// command line options: one of OptionsErrorDisplayHelp,
// OptionsErrorDisplayVersion, OptionsErrorNoInput, OptionsErrorError and
// OptionsErrorInvalidInputFile (Rust's CliOptionsError).
type OptionsError interface {
	error
	optionsError()
}

type OptionsErrorDisplayHelp struct{ Message string }
type OptionsErrorDisplayVersion struct{ Message string }
type OptionsErrorNoInput struct{ Message string }
type OptionsErrorError struct{ Message string }
type OptionsErrorInvalidInputFile struct{ Path string }

func (OptionsErrorDisplayHelp) optionsError()      {}
func (OptionsErrorDisplayVersion) optionsError()   {}
func (OptionsErrorNoInput) optionsError()          {}
func (OptionsErrorError) optionsError()            {}
func (OptionsErrorInvalidInputFile) optionsError() {}

func (e OptionsErrorDisplayHelp) Error() string    { return e.Message }
func (e OptionsErrorDisplayVersion) Error() string { return e.Message }
func (e OptionsErrorNoInput) Error() string        { return e.Message }
func (e OptionsErrorError) Error() string          { return "error: " + e.Message }
func (e OptionsErrorInvalidInputFile) Error() string {
	return fmt.Sprintf("error: Cannot access '%s': No such file or directory", e.Path)
}

func optionsErrorf(format string, a ...any) OptionsError {
	return OptionsErrorError{Message: fmt.Sprintf(format, a...)}
}

// CliError is an error from the command line program: one of
// CliErrorInputRead, CliErrorInvalidOption, CliErrorParsing,
// CliErrorOutputWrite and CliErrorGenericIO.
//
// Assert failures are not represented here: they happen when the program
// has successfully run and returned results with false assertions.
type CliError interface {
	error
	cliError()
}

type CliErrorInputRead struct{ Message string }
type CliErrorInvalidOption struct{ Message string }

// CliErrorParsing is a parse error of a Hurl file, already displayed.
type CliErrorParsing struct{}
type CliErrorOutputWrite struct{ Message string }
type CliErrorGenericIO struct{ Message string }

func (CliErrorInputRead) cliError()     {}
func (CliErrorInvalidOption) cliError() {}
func (CliErrorParsing) cliError()       {}
func (CliErrorOutputWrite) cliError()   {}
func (CliErrorGenericIO) cliError()     {}

func (e CliErrorInputRead) Error() string     { return e.Message }
func (e CliErrorInvalidOption) Error() string { return e.Message }
func (CliErrorParsing) Error() string         { return "" }
func (e CliErrorOutputWrite) Error() string   { return e.Message }
func (e CliErrorGenericIO) Error() string     { return e.Message }

// BaseLogger logs app related events (start, high level errors...) on
// standard error.
type BaseLogger struct {
	format  text.Format
	verbose bool
}

func NewBaseLogger(color, verbose bool) *BaseLogger {
	format := text.FormatPlain
	if color {
		format = text.FormatANSI
	}
	return &BaseLogger{format: format, verbose: verbose}
}

// Info prints an informational message on standard error.
func (l *BaseLogger) Info(message string) {
	eprintln(message)
}

// Debug prints a debug message on standard error in verbose mode.
func (l *BaseLogger) Debug(message string) {
	if !l.verbose {
		return
	}
	var s text.StyledString
	s.PushWith("*", text.NewStyle().Blue().Bold())
	if message != "" {
		s.Push(" " + message)
	}
	eprintln(s.Render(l.format))
}

// Error prints an error message on standard error.
func (l *BaseLogger) Error(message string) {
	if message == "" {
		return
	}
	var s text.StyledString
	s.PushWith("error", text.NewStyle().Red().Bold())
	s.Push(": ")
	s.PushWith(message, text.NewStyle().Bold())
	eprintln(s.Render(l.format))
}
