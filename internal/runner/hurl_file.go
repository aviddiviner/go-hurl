package runner

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/parser"
)

// Run parses and runs a Hurl content, and returns its result. The success of
// the run can be read in HurlResult.Success; the only error returned is a
// *parser.ParseError, when content is not a valid Hurl file (it has already
// been logged). content must not start with a BOM. filename is an optional
// source, used when displaying errors.
//
// Standard output and standard error are written immediately.
func Run(content string, filename *input.Input, options RunnerOptions, variables *VariableSet, loggerOptions logger.LoggerOptions) (*HurlResult, error) {
	stdout := term.NewStdout(term.WriteModeImmediate)
	stderr := term.NewStderr(term.WriteModeImmediate)
	l := logger.New(loggerOptions, stderr, variables.Secrets())

	hurlFile, perr := parser.ParseHurlFile(content)
	if perr != nil {
		message := sourceerr.Render(perr, inputName(filename), content, nil, sourceerr.OutputFormatTerminal{Color: l.Color})
		l.ErrorRich(message)
		return nil, perr
	}

	result := RunEntries(hurlFile.Entries, content, filename, options, variables, stdout, nil, l)
	if result.Success && len(result.Entries) == 0 {
		l.Warning("No entry have been executed for file " + inputName(filename))
	}
	return &result, nil
}

func inputName(filename *input.Input) string {
	if filename == nil {
		return ""
	}
	return filename.String()
}

// RunEntries runs a list of entries and returns the result. content is the
// source of the entries, used to display rich errors. listener, if not nil,
// is notified of each entry run (for the progress bar in --test mode).
func RunEntries(entries []ast.Entry, content string, filename *input.Input, options RunnerOptions, variables *VariableSet, stdout *term.Stdout, listener EventListener, l *logger.Logger) HurlResult {
	if len(entries) == 0 {
		return HurlResult{Success: true, Variables: variables.Clone(), CookieStore: httpclient.NewCookieStore()}
	}

	client := httpclient.NewClient()
	defer client.Close()
	var results []EntryResult
	variables = variables.Clone()
	current := 1
	if options.FromEntry > 0 {
		current = options.FromEntry
	}
	last := len(entries)
	if options.ToEntry > 0 {
		last = options.ToEntry
	}
	repeatCount := 0
	defaultVerbosity := l.Verbosity
	start := time.Now()
	timestamp := start.Unix()

	logRunInfo(entries, options, variables, l)

	// The entry index is not always incremented on each loop: an entry can
	// be retried or repeated. Each entry is run with the global options
	// overridden by its [Options] section.
	for current <= last {
		entry := &entries[current-1]
		index := ast.NewIndex(current)

		// Entry options can modify the logger verbosity, and the preamble
		// is displayed with the entry verbosity.
		l.Verbosity = defaultVerbosity
		if v, err := entryVerbosity(entry, defaultVerbosity, variables); err == nil {
			l.Verbosity = v
		}

		logRunEntry(index, l)

		if listener != nil {
			listener.OnEntryRunning(index, ast.NewIndex(last), 0)
		}

		entryOpts, err := entryOptions(entry, options, variables, l)
		if err != nil {
			// Errors evaluating options are not retried.
			result := newEntryResult(index, entry.SourceInfo())
			result.Errors = []*RunnerError{err}
			logErrors(&result, content, filename, false, l)
			results = append(results, result)
			if options.ContinueOnError {
				current++
				continue
			}
			break
		}

		if entryOpts.Skip {
			l.Debug("")
			l.DebugImportant(fmt.Sprintf("Entry %d has been skipped", current))
			current++
			continue
		}

		// Repeat 0 is equivalent to skip.
		if r, ok := entryOpts.Repeat.(ast.CountFinite); ok && r.Value == 0 {
			l.Debug("")
			l.DebugImportant(fmt.Sprintf("Entry %d is skipped (repeat 0 times)", current))
			current++
			continue
		}

		if delay := entryOpts.Delay.Milliseconds(); delay > 0 {
			l.Debug("")
			l.DebugImportant(fmt.Sprintf("Delay entry %d (pause %d ms)", current, delay))
			time.Sleep(entryOpts.Delay)
		}

		entryResults := runRequest(entry, index, ast.NewIndex(last), content, filename, client, entryOpts, variables, stdout, listener, l)
		hasError := len(entryResults[len(entryResults)-1].Errors) > 0
		results = append(results, entryResults...)

		if !options.ContinueOnError && hasError {
			break
		}

		repeatCount++
		switch r := entryOpts.Repeat.(type) {
		case nil:
			repeatCount = 0
			current++
		case ast.CountFinite:
			if repeatCount >= r.Value {
				repeatCount = 0
				current++
			} else {
				l.DebugImportant(fmt.Sprintf("Repeat entry %d (x%d/%d)", current, repeatCount, r.Value))
			}
		case ast.CountInfinite:
			l.DebugImportant(fmt.Sprintf("Repeat entry %d (x%d)", current, repeatCount))
		}
	}

	return HurlResult{
		Entries:     results,
		Duration:    time.Since(start),
		Success:     isSuccess(results),
		CookieStore: client.CookieStore(l),
		Timestamp:   timestamp,
		Variables:   variables,
	}
}

// runRequest runs an entry, retrying it on errors until there are none or
// the maximum retry count is reached. It returns the results of each try.
func runRequest(entry *ast.Entry, current, last ast.Index, content string, filename *input.Input, client *httpclient.Client, options RunnerOptions, variables *VariableSet, stdout *term.Stdout, listener EventListener, l *logger.Logger) []EntryResult {
	var results []EntryResult
	retryCount := 0

	for {
		result := runEntry(entry, current, client, variables, options, l)
		hasError := len(result.Errors) > 0

		// The retry threshold can only be reached with a finite number of
		// retries.
		retryMaxReached := false
		if r, ok := options.Retry.(ast.CountFinite); ok {
			retryMaxReached = retryCount >= r.Value
		}
		// The warning is displayed before any assert error, so that the
		// error is the last thing displayed.
		if retryMaxReached {
			l.DebugImportant("Retry max count reached, no more retry")
			l.Debug("")
		}

		// Output errors are not retried.
		retry := options.Retry != nil && !retryMaxReached && hasError

		// The response body is written when --output is set on the entry and
		// there is no error, or when --fail-with-body is set, there are
		// errors and we're not retrying.
		var output Output
		switch {
		case !hasError:
			output = options.Output
		case options.FailWithBody && !retry:
			output = options.Output
			if output == nil {
				output = util.OutputStdout{}
			}
		}
		if output != nil {
			writeEntryResponse(entry, &result, output, options, stdout)
		}

		// Writing the response may have added errors.
		if len(result.Errors) > 0 {
			// Error messages start on a new line if the response is written
			// on stdout.
			if _, ok := output.(util.OutputStdout); ok && !result.HasResponseTrailingNewline() {
				_ = stdout.WriteAll([]byte("\n"))
			}
			logErrors(&result, content, filename, retry, l)
		}

		results = append(results, result)

		if !retry {
			break
		}
		retryCount++
		retryMax := "ꝏ"
		if r, ok := options.Retry.(ast.CountFinite); ok {
			retryMax = strconv.Itoa(r.Value)
		}
		l.Debug("")
		l.DebugImportant(fmt.Sprintf("Retry on entry %d (count: %d/%s, interval: %d ms)", current.Get(), retryCount, retryMax, options.RetryInterval.Milliseconds()))

		// The running event is reported before sleeping, so the retry hint
		// takes the interval into account.
		if listener != nil {
			listener.OnEntryRunning(current, last, retryCount)
		}

		time.Sleep(options.RetryInterval)

		logRunEntry(current, l)
	}
	return results
}

// writeEntryResponse writes the response of an entry to output, appending
// write errors (I/O or unauthorized access) to the result errors.
func writeEntryResponse(entry *ast.Entry, result *EntryResult, output Output, options RunnerOptions, stdout *term.Stdout) {
	si := outputSourceInfo(entry)
	output, err := outputTryWith(output, options.ContextDir, si)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return
	}
	// Headers are not included, and existing files are truncated.
	if err := result.WriteResponse(output, stdout, false, options.ColorStdout, options.Pretty, false, si); err != nil {
		result.Errors = append(result.Errors, err)
	}
}

// outputSourceInfo returns the source info of the entry output option, or
// of the entry.
func outputSourceInfo(entry *ast.Entry) ast.SourceInfo {
	si := entry.SourceInfo()
	for _, option := range entry.Request.Options() {
		if o, ok := option.Kind.(ast.OptionOutput); ok {
			si = o.Value.SourceInfo
		}
	}
	return si
}

// isSuccess reports whether all the entries are successful. Only the last
// result of a given entry index is checked (previous ones are retries).
func isSuccess(results []EntryResult) bool {
	for i, r := range results {
		if i+1 == len(results) {
			return len(r.Errors) == 0
		}
		if results[i+1].EntryIndex != r.EntryIndex && len(r.Errors) > 0 {
			return false
		}
	}
	return true
}

type nameValue struct{ name, value string }

// nonDefaultOptions returns the options that have non-default values.
func nonDefaultOptions(options RunnerOptions) []nameValue {
	d := DefaultRunnerOptions()
	var opts []nameValue
	if options.ContinueOnError != d.ContinueOnError {
		opts = append(opts, nameValue{"continue_on_error", strconv.FormatBool(options.ContinueOnError)})
	}
	if options.Delay != d.Delay {
		opts = append(opts, nameValue{"delay", fmt.Sprintf("%dms", options.Delay.Milliseconds())})
	}
	if options.FollowLocation != d.FollowLocation && options.FollowLocation.Follow {
		value := "true"
		if options.FollowLocation.Forwarding == httpclient.AllHosts {
			value = "true (trusted)"
		}
		opts = append(opts, nameValue{"follow redirect", value})
	}
	if options.Insecure != d.Insecure {
		opts = append(opts, nameValue{"insecure", strconv.FormatBool(options.Insecure)})
	}
	if options.MaxRedirect != d.MaxRedirect {
		opts = append(opts, nameValue{"max redirect", options.MaxRedirect.String()})
	}
	if options.Proxy != d.Proxy {
		opts = append(opts, nameValue{"proxy", options.Proxy})
	}
	if options.Retry != d.Retry {
		value := "none"
		if options.Retry != nil {
			value = options.Retry.String()
		}
		opts = append(opts, nameValue{"retry", value})
	}
	if options.UnixSocket != d.UnixSocket {
		opts = append(opts, nameValue{"unix socket", options.UnixSocket})
	}
	return opts
}

// logRunInfo logs debug information at the start of a run.
func logRunInfo(entries []ast.Entry, options RunnerOptions, variables *VariableSet, l *logger.Logger) {
	if l.Verbosity != logger.VerbosityNone {
		if opts := nonDefaultOptions(options); len(opts) > 0 {
			l.DebugImportant("Options:")
			for _, o := range opts {
				l.Debug(fmt.Sprintf("    %s: %s", o.name, o.value))
			}
		}
	}

	var public []Variable
	for _, v := range variables.All() {
		if !v.IsSecret() {
			public = append(public, v)
		}
	}
	if len(public) > 0 {
		slices.SortFunc(public, func(a, b Variable) int { return cmp.Compare(a.Name, b.Name) })
		l.DebugImportant("Variables:")
		for _, v := range public {
			l.Debug(fmt.Sprintf("    %s: %s", v.Name, v.Value))
		}
	}
	if options.ToEntry > 0 {
		l.Debug(fmt.Sprintf("Executing %d/%d entries", options.ToEntry, len(entries)))
	}
}

// logErrors logs the errors of an entry. When the entry is going to be
// retried, errors are only logged in verbose mode.
func logErrors(result *EntryResult, content string, filename *input.Input, retry bool, l *logger.Logger) {
	if retry {
		for _, err := range result.Errors {
			l.DebugError(content, filename, err, result.SourceInfo)
		}
		return
	}

	if l.ErrorFormat == logger.ErrorFormatLong && len(result.Calls) > 0 {
		l.InfoCurlCmd(result.CurlCmd)
		l.Info("")
		result.Calls[len(result.Calls)-1].Response.LogInfoAll(l)
	}

	for _, err := range result.Errors {
		message := sourceerr.Render(err, inputName(filename), content, &result.SourceInfo, sourceerr.OutputFormatTerminal{Color: l.Color})
		l.ErrorRich(message)
	}
}

// logRunEntry logs the header of an entry run.
func logRunEntry(index ast.Index, l *logger.Logger) {
	l.DebugImportant("------------------------------------------------------------------------------")
	l.DebugImportant(fmt.Sprintf("Executing entry %d", index.Get()))
}
