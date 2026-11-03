// Command hurlfmt formats Hurl files, ported from Hurl's hurlfmt.
package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/curlimport"
	"github.com/aviddiviner/go-hurl/internal/format"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/lint"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/text"
	"github.com/aviddiviner/go-hurl/parser"
)

const (
	exitOK           = 0
	exitError        = 1
	exitInvalidInput = 2
	exitLintIssue    = 3
)

func main() {
	opts, err := parseArgs(os.Args)
	if err != nil {
		var e *optionsError
		if errors.As(err, &e) && e.info {
			fmt.Print(e.message)
			os.Exit(exitOK)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(exitError)
	}
	color := term.IsTerminal(int(os.Stdout.Fd()))
	if opts.color != nil {
		color = *opts.color
	}
	logger := newLogger(color)

	switch {
	case opts.check:
		os.Exit(checkCommand(opts.inputFiles, opts.outputFile, logger))
	case opts.inPlace:
		os.Exit(formatCommand(opts.inputFiles, logger))
	default:
		os.Exit(exportCommand(opts, logger, color))
	}
}

// fileError is an error on a file: either an IO error or a parse error.
type fileError struct {
	filename string
	message  string // IO error message, if parseErr is nil
	content  string
	parseErr *parser.ParseError
}

func (e *fileError) Error() string { return e.filename + ": " + e.message }

func (l *logger) fileError(e *fileError) {
	if e.parseErr != nil {
		l.errorParsing(e.content, e.filename, e.parseErr)
	} else {
		l.error(fmt.Sprintf("Input file %s can not be read - %s", e.filename, e.message))
	}
}

func readAndParse(in input.Input) (string, *ast.HurlFile, *fileError) {
	content, err := in.ReadToString()
	if err != nil {
		return "", nil, &fileError{filename: in.String(), message: err.Error()}
	}
	file, perr := parser.ParseHurlFile(content)
	if perr != nil {
		return "", nil, &fileError{filename: in.String(), content: content, parseErr: perr}
	}
	return content, file, nil
}

func checkCommand(inputs []input.Input, outputFile string, l *logger) int {
	count := 0
	invalidInput := false
	output := ""
	errored := false
	for _, in := range inputs {
		content, file, ferr := readAndParse(in)
		if ferr != nil {
			l.fileError(ferr)
			invalidInput, errored = true, true
			continue
		}
		if lint.HurlFile(file) != content {
			output += fmt.Sprintf("would reformat: %s\n", in)
			count++
			errored = true
		}
	}
	if !errored {
		return exitOK
	}
	if count > 0 {
		plural := ""
		if count > 1 {
			plural = "s"
		}
		output += fmt.Sprintf("%d file%s would be reformatted", count, plural)
	}
	writeOutput(output, outputFile, l)
	if invalidInput {
		return exitInvalidInput
	}
	return exitLintIssue
}

func formatCommand(inputs []input.Input, l *logger) int {
	var paths []string
	for _, in := range inputs {
		f, ok := in.Kind().(input.File)
		if !ok {
			l.error("Standard input can be formatted in place!")
			return exitInvalidInput
		}
		paths = append(paths, f.Path)
	}
	failed := false
	for _, path := range paths {
		if ferr := formatFile(path); ferr != nil {
			l.fileError(ferr)
			failed = true
		}
	}
	if failed {
		return exitInvalidInput
	}
	return exitOK
}

// formatFile formats a file in place. As Hurl, it reads the file as is,
// without removing any BOM.
func formatFile(path string) *fileError {
	b, err := os.ReadFile(path)
	if err != nil {
		return &fileError{filename: path, message: rustIOError(err).Error()}
	}
	if !utf8.Valid(b) {
		return &fileError{filename: path, message: "stream did not contain valid UTF-8"}
	}
	content := string(b)
	file, perr := parser.ParseHurlFile(content)
	if perr != nil {
		return &fileError{filename: path, content: content, parseErr: perr}
	}
	if err := os.WriteFile(path, []byte(lint.HurlFile(file)), 0o666); err != nil {
		return &fileError{filename: path, message: rustIOError(err).Error()}
	}
	return nil
}

func exportCommand(opts *options, l *logger, color bool) int {
	failed := false
	output := ""
	for _, in := range opts.inputFiles {
		s, err := export(in, opts, color)
		if err != nil {
			failed = true
			var ferr *fileError
			if errors.As(err, &ferr) {
				l.fileError(ferr)
			} else {
				l.error(err.Error())
			}
			continue
		}
		output += s
	}
	writeOutput(output, opts.outputFile, l)
	if failed {
		return exitInvalidInput
	}
	return exitOK
}

func export(in input.Input, opts *options, color bool) (string, error) {
	content, err := in.ReadToString()
	if err != nil {
		return "", &fileError{filename: in.String(), message: err.Error()}
	}
	if opts.inputFormat == inputCurl {
		if content, err = curlimport.Parse(content); err != nil {
			return "", err
		}
	}
	file, perr := parser.ParseHurlFile(content)
	if perr != nil {
		return "", &fileError{filename: in.String(), content: content, parseErr: perr}
	}
	switch opts.outputFormat {
	case outputJSON:
		return format.JSON(file), nil
	case outputHTML:
		return format.HTML(file, opts.standalone), nil
	}
	linted, perr := parser.ParseHurlFile(lint.HurlFile(file))
	if perr != nil {
		panic(perr)
	}
	return format.Text(linted, color), nil
}

func writeOutput(content, filename string, l *logger) {
	if len(content) == 0 || content[len(content)-1] != '\n' {
		content += "\n"
	}
	if filename == "" {
		if _, err := os.Stdout.WriteString(content); err != nil {
			l.error(fmt.Sprintf("Issue writing to stdout: %s", rustIOError(err)))
			os.Exit(exitError)
		}
		return
	}
	f, err := os.Create(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Issue writing to %s: %s\n", filename, rustIODebug(err))
		os.Exit(exitError)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		panic(fmt.Sprintf("writing bytes to file: %s", rustIODebug(err)))
	}
}

// logger logs high level errors on standard error.
type logger struct {
	format text.Format
}

func newLogger(color bool) *logger {
	if color {
		return &logger{format: text.FormatANSI}
	}
	return &logger{format: text.FormatPlain}
}

func (l *logger) error(message string) {
	var s text.StyledString
	s.PushWith("error", text.NewStyle().Red().Bold())
	s.Push(": ")
	s.PushWith(message, text.NewStyle().Bold())
	fmt.Fprintln(os.Stderr, s.Render(l.format))
}

func (l *logger) errorParsing(content, filename string, err *parser.ParseError) {
	message := sourceerr.Render(err, filename, content, nil,
		sourceerr.OutputFormatTerminal{Color: l.format == text.FormatANSI})
	var s text.StyledString
	s.PushWith("error", text.NewStyle().Red().Bold())
	s.Push(": ")
	s.Push(message)
	s.Push("\n")
	fmt.Fprintln(os.Stderr, s.Render(l.format))
}

// rustIOError rewords an OS error like Rust's io::Error Display, e.g.
// "No such file or directory (os error 2)".
func rustIOError(err error) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	return fmt.Errorf("%s (os error %d)", capitalize(errno.Error()), int(errno))
}

// rustIODebug formats an OS error like Rust's io::Error Debug.
func rustIODebug(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return fmt.Sprintf("%q", err.Error())
	}
	kind := "Uncategorized"
	switch errno {
	case syscall.ENOENT:
		kind = "NotFound"
	case syscall.EACCES, syscall.EPERM:
		kind = "PermissionDenied"
	case syscall.EEXIST:
		kind = "AlreadyExists"
	case syscall.EISDIR:
		kind = "IsADirectory"
	case syscall.ENOTDIR:
		kind = "NotADirectory"
	case syscall.EROFS:
		kind = "ReadOnlyFilesystem"
	case syscall.ENOSPC:
		kind = "StorageFull"
	}
	return fmt.Sprintf("Os { code: %d, kind: %s, message: %q }", int(errno), kind, capitalize(errno.Error()))
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
