// Command hurl runs and tests HTTP requests with plain text, ported from
// Hurl's hurl binary (main.rs, run.rs).
package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/term"

	"github.com/aviddiviner/go-hurl/internal/cli"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/report/curl"
	"github.com/aviddiviner/go-hurl/internal/report/html"
	"github.com/aviddiviner/go-hurl/internal/report/json"
	"github.com/aviddiviner/go-hurl/internal/report/junit"
	"github.com/aviddiviner/go-hurl/internal/report/tap"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

const (
	exitOK               = 0
	exitErrorCommandline = 1
	exitErrorParsing     = 2
	exitErrorRuntime     = 3
	exitErrorAssert      = 4
	exitErrorUndefined   = 127
)

// hurlRun is the result of a Hurl file run, with the content of the file.
type hurlRun struct {
	content  string
	filename input.Input
	result   *runner.HurlResult
}

func main() {
	os.Exit(run())
}

func run() int {
	// The run context and the environment variables: this is the only place
	// where we read environment variables.
	env := map[string]string{}
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok {
			env[k] = v
		}
	}
	envVars := cli.NewEnvVars(env)
	isTerm := func(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }
	ctx := cli.NewRunContext(envVars, isTerm(os.Stdin), isTerm(os.Stdout), isTerm(os.Stderr))

	opts, err := cli.Parse(os.Args, ctx, envVars)
	if err != nil {
		switch err.(type) {
		case cli.OptionsErrorDisplayHelp, cli.OptionsErrorDisplayVersion:
			fmt.Print(err.Error())
			return exitOK
		}
		fmt.Fprintln(os.Stderr, err.Error())
		return exitErrorCommandline
	}

	// A basic logger for info, warning or error generic messages; a richer
	// logger is used when running Hurl files.
	verbose := opts.Verbosity == cli.VerbosityVerbose || opts.Verbosity == cli.VerbosityDebug
	baseLogger := cli.NewBaseLogger(opts.ColorStderr.Get(), verbose)
	currentDir, err := os.Getwd()
	if err != nil {
		baseLogger.Error(rustIOError(err))
		return exitErrorUndefined
	}
	start := time.Now()

	var runs []*hurlRun
	if opts.Parallel.Get() {
		workersCount := opts.Jobs
		if workersCount == 0 {
			workersCount = runtime.NumCPU()
		}
		baseLogger.Debug(fmt.Sprintf("Parallel run using %d workers", workersCount))
		runs, err = runPar(opts.InputFiles, currentDir, opts, workersCount)
	} else {
		runs, err = runSeq(opts.InputFiles, currentDir, opts)
	}
	// False assertions are not errors here: they have been displayed and are
	// in the results.
	if err != nil {
		switch e := err.(type) {
		case cli.CliErrorInvalidOption:
			baseLogger.Error(e.Message)
			return exitErrorCommandline
		// Input read errors are considered as parsing errors.
		case cli.CliErrorInputRead:
			baseLogger.Error(e.Message)
			return exitErrorParsing
		case cli.CliErrorGenericIO:
			baseLogger.Error(e.Message)
			return exitErrorParsing
		// Parsing errors have already been displayed.
		case cli.CliErrorParsing:
			return exitErrorParsing
		case cli.CliErrorOutputWrite:
			baseLogger.Error(e.Message)
			return exitErrorRuntime
		}
		baseLogger.Error(err.Error())
		return exitErrorUndefined
	}

	// The duration doesn't take the reports writing into account.
	duration := time.Since(start)

	if hasReport(opts) {
		if err := exportResults(runs, opts, baseLogger); err != nil {
			baseLogger.Error(err.Error())
			return exitErrorUndefined
		}
	}

	if opts.Test {
		results := make([]*runner.HurlResult, len(runs))
		for i, r := range runs {
			results[i] = r.result
		}
		baseLogger.Info(cli.Summary(results, duration))
	}
	return exitCode(runs)
}

// hasReport reports whether any kind of report should be created.
func hasReport(opts *cli.CliOptions) bool {
	return opts.CurlFile != "" || opts.JunitFile != "" || opts.TapFile != "" ||
		opts.HtmlDir != "" || opts.JSONReportDir != "" || opts.CookieOutputFile != ""
}

// exportResults writes the curl, JUnit, TAP, HTML, JSON reports and the
// cookies file. The error is a cli.CliError.
func exportResults(runs []*hurlRun, opts *cli.CliOptions, l *cli.BaseLogger) error {
	// Secrets can be produced during the execution, so we take them from the
	// results, without duplicates.
	set := map[string]struct{}{}
	for _, r := range runs {
		if r.result.Variables != nil {
			for _, s := range r.result.Variables.Secrets() {
				set[s] = struct{}{}
			}
		}
	}
	secrets := slices.Sorted(maps.Keys(set))

	results := make([]*runner.HurlResult, len(runs))
	for i, r := range runs {
		results[i] = r.result
	}
	if opts.CurlFile != "" {
		if err := curl.WriteCurl(results, opts.CurlFile, secrets); err != nil {
			return genericIO(err)
		}
	}
	if opts.JunitFile != "" {
		l.Debug("Writing JUnit report to " + opts.JunitFile)
		var testcases []junit.Testcase
		for _, r := range runs {
			testcases = append(testcases, junit.NewTestcase(r.result, r.content, r.filename))
		}
		if err := junit.WriteReport(opts.JunitFile, testcases, secrets); err != nil {
			return genericIO(err)
		}
	}
	if opts.TapFile != "" {
		// TAP reports don't need to be redacted: they only expose file names.
		l.Debug("Writing TAP report to " + opts.TapFile)
		var testcases []tap.Testcase
		for _, r := range runs {
			testcases = append(testcases, tap.NewTestcase(r.result, r.filename))
		}
		if err := tap.WriteReport(opts.TapFile, testcases); err != nil {
			return genericIO(err)
		}
	}
	if opts.HtmlDir != "" {
		l.Debug("Writing HTML report to " + opts.HtmlDir)
		if err := createHTMLReport(runs, opts.HtmlDir, secrets); err != nil {
			return err
		}
	}
	if opts.JSONReportDir != "" {
		l.Debug("Writing JSON report to " + opts.JSONReportDir)
		storeDir := rustJoin(opts.JSONReportDir, "store")
		if err := os.MkdirAll(storeDir, 0o755); err != nil {
			return cli.CliErrorGenericIO{Message: rustIOError(err)}
		}
		var testcases []json.Testcase
		for _, r := range runs {
			testcases = append(testcases, json.NewTestcase(r.result, r.content, r.filename))
		}
		indexPath := rustJoin(opts.JSONReportDir, "report.json")
		if err := json.WriteReport(indexPath, testcases, storeDir, secrets); err != nil {
			return genericIO(err)
		}
	}
	if opts.CookieOutputFile != "" {
		l.Debug("Writing cookies to " + opts.CookieOutputFile)
		if err := createCookiesFile(runs, opts.CookieOutputFile, secrets); err != nil {
			return err
		}
	}
	return nil
}

func genericIO(err error) error {
	return cli.CliErrorGenericIO{Message: err.Error()}
}

// rustJoin joins like Rust's Path::join, without cleaning the path.
func rustJoin(dir, name string) string {
	if dir == "" || strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

func createHTMLReport(runs []*hurlRun, dir string, secrets []string) error {
	storeDir := rustJoin(dir, "store")
	if err := os.MkdirAll(storeDir, 0o755); err != nil {
		return cli.CliErrorGenericIO{Message: rustIOError(err)}
	}
	var testcases []html.Testcase
	for _, r := range runs {
		testcase := html.NewTestcase(r.result, r.filename)
		if err := testcase.WriteHTML(r.content, r.result.Entries, storeDir, secrets); err != nil {
			return genericIO(err)
		}
		testcases = append(testcases, testcase)
	}
	if err := html.WriteReport(dir, testcases); err != nil {
		return genericIO(err)
	}
	return nil
}

// exitCode returns the exit code of the runs: runtime errors first, then
// assert errors.
func exitCode(runs []*hurlRun) int {
	runtimeErrors, assertErrors := 0, 0
	for _, r := range runs {
		errs := r.result.Errors()
		if len(errs) == 0 {
			continue
		}
		if slices.ContainsFunc(errs, func(e runner.EntryError) bool { return !e.Error.Assert }) {
			runtimeErrors++
		} else {
			assertErrors++
		}
	}
	switch {
	case runtimeErrors > 0:
		return exitErrorRuntime
	case assertErrors > 0:
		return exitErrorAssert
	}
	return exitOK
}

// createCookiesFile exports the cookies of the runs to filename, in the
// Netscape cookie format.
func createCookiesFile(runs []*hurlRun, filename string, secrets []string) error {
	if err := util.CreateDirAll(filename); err != nil {
		return cli.CliErrorGenericIO{Message: fmt.Sprintf("Issue creating parent directories for %s: %s", filename, rustIODebug(err))}
	}
	f, err := os.Create(filename)
	if err != nil {
		return cli.CliErrorGenericIO{Message: fmt.Sprintf("Issue writing to %s: %s", filename, rustIODebug(err))}
	}
	defer f.Close()
	s := "# Netscape HTTP Cookie File\n# This file was generated by Hurl\n\n"
	if len(runs) == 0 {
		return cli.CliErrorGenericIO{Message: "Issue fetching results"}
	}
	for _, r := range runs {
		s += fmt.Sprintf("# Cookies for file <%s>\n", r.filename)
		if r.result.CookieStore != nil {
			s += util.Redact(r.result.CookieStore.ToNetscape(), secrets)
		}
	}
	if _, err := f.WriteString(s); err != nil {
		return cli.CliErrorGenericIO{Message: fmt.Sprintf("Issue writing to %s: %s", filename, rustIODebug(err))}
	}
	return nil
}

// rustIOError formats an OS error like Rust's io::Error Display, e.g.
// "No such file or directory (os error 2)".
func rustIOError(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	return fmt.Sprintf("%s (os error %d)", capitalize(errno.Error()), int(errno))
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
