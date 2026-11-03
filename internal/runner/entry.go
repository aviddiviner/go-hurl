package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/term"
)

// runEntry runs an entry with client and returns its result. The Calls of
// the result hold every step of the redirections, if the client follows
// them. variables are used to render values and are updated by captures.
func runEntry(entry *ast.Entry, entryIndex ast.Index, client *httpclient.Client, variables *VariableSet, options RunnerOptions, l *logger.Logger) EntryResult {
	sourceInfo := entry.SourceInfo()
	contextDir := options.ContextDir
	result := newEntryResult(entryIndex, sourceInfo)
	result.Compressed = options.Compressed

	// Secrets can't be created when the logger is immediate and verbose:
	// network logs have already been written and may have leaked secrets
	// before captures evaluation. In --test mode, the logger is buffered so
	// there is no restriction.
	if entry.Response != nil && l.Stderr.Mode() == term.WriteModeImmediate && l.Verbosity != logger.VerbosityNone {
		for _, c := range entry.Response.Captures() {
			if c.Redacted {
				result.SourceInfo = c.Name.SourceInfo
				result.Errors = []*RunnerError{NewRunnerError(c.Name.SourceInfo, RunnerErrorPossibleLoggedSecret{}, false)}
				return result
			}
		}
	}

	spec, err := evalRequest(&entry.Request, variables, contextDir)
	if err != nil {
		result.Errors = []*RunnerError{err}
		return result
	}

	clientOptions := options.clientOptions(l.Verbosity)

	// Experimental features with cookie storage.
	if s, ok := cmdCookieStorageSet(&entry.Request); ok {
		if cookie, err := httpclient.CookieFromNetscape(s); err == nil {
			client.AddCookie(cookie, l)
		} else {
			l.Warning("Cookie string can not be parsed: '" + s + "'")
		}
	}
	if cmdCookieStorageClear(&entry.Request) {
		client.ClearCookieStorage(l)
	}

	result.CurlCmd = client.CurlCommandLine(spec, contextDir, options.Output, clientOptions, l)

	if !l.IsLowVerbosity() {
		logRequest(client, result.CurlCmd, spec, l)
	}

	calls, herr := client.ExecuteWithRedirect(spec, clientOptions, l)
	if herr != nil {
		si := ast.SourceInfo{Start: entry.Request.URL.SourceInfo.Start, End: entry.Request.URL.SourceInfo.End}
		result.Errors = []*RunnerError{NewRunnerError(si, RunnerErrorHTTP{Value: herr}, false)}
		return result
	}
	result.Calls = calls

	// Captures and asserts are computed on the last call of the chain.
	responses := make([]*httpclient.Response, len(calls))
	for i := range calls {
		responses[i] = &calls[i].Response
		result.TransferDuration += calls[i].Timings.Total
	}
	last := responses[len(responses)-1]

	// Asserts and captures are processed in this order:
	// 1. implicit asserts on status and version: if KO, the entry fails,
	// 2. captures, as they may be used in asserts,
	// 3. the remaining asserts.
	cache := &bodyCache{}
	queryOptions := QueryOptions{UseJSONPathCoercion: options.UseJSONPathCoercion}

	if !options.NoAssert && entry.Response != nil {
		result.Asserts = evalVersionStatusAsserts(entry.Response, last)
		if errors := assertsToErrors(result.Asserts); len(errors) > 0 {
			l.Debug("")
			result.Errors = errors
			return result
		}
	}

	if entry.Response != nil {
		captures, err := evalCaptures(entry.Response, responses, cache, variables, queryOptions)
		if err != nil {
			result.Errors = []*RunnerError{err}
			return result
		}
		result.Captures = captures
	}

	// Captures may have added new secrets to redact.
	l.SetSecrets(variables.Secrets())

	logCaptures(result.Captures, l)
	l.Debug("")

	if !options.NoAssert && entry.Response != nil {
		warnDeprecated(entry.Response, l)
		result.Asserts = append(result.Asserts, evalAsserts(entry.Response, variables, responses, cache, contextDir, queryOptions)...)
	}
	result.Errors = assertsToErrors(result.Asserts)
	return result
}

func assertsToErrors(asserts []AssertResult) []*RunnerError {
	var errors []*RunnerError
	for _, a := range asserts {
		if err := a.ToRunnerError(); err != nil {
			errors = append(errors, NewRunnerError(err.SourceInfo(), err.Kind, true))
		}
	}
	return errors
}

func logRequest(client *httpclient.Client, curlCmd httpclient.CurlCmd, spec httpclient.RequestSpec, l *logger.Logger) {
	cookieStore := client.CookieStore(l)

	l.Debug("")
	l.DebugImportant("Cookie store:")
	for _, cookie := range cookieStore.Cookies() {
		l.Debug(cookie.String())
	}

	l.Debug("")
	l.DebugImportant("Request:")
	l.Debug(spec.Method.String() + " " + spec.Url.Raw())
	for _, h := range spec.Headers.All() {
		l.Debug(h.String())
	}
	if len(spec.Querystring) > 0 {
		l.Debug("[Query]")
		for _, p := range spec.Querystring {
			l.Debug(p.String())
		}
	}
	if len(spec.Form) > 0 {
		l.Debug("[Form]")
		for _, p := range spec.Form {
			l.Debug(p.String())
		}
	}
	if len(spec.Multipart) > 0 {
		l.Debug("[Multipart]")
		for _, p := range spec.Multipart {
			l.Debug(p.String())
		}
	}
	if len(spec.Cookies) > 0 {
		l.Debug("[Cookies]")
		for _, c := range spec.Cookies {
			l.Debug(c.String())
		}
	}
	l.DebugCurlCmd(curlCmd)
}

func logCaptures(captures []CaptureResult, l *logger.Logger) {
	if len(captures) == 0 {
		return
	}
	l.DebugImportant("Captures:")
	for _, c := range captures {
		l.Capture(c.Name, c.Value.String())
	}
}

func warnDeprecated(response *ast.Response, l *logger.Logger) {
	asserts := response.Asserts()
	captures := response.Captures()
	assertsHave := func(f func(ast.FilterValue) bool) bool {
		for _, a := range asserts {
			if hasFilter(a.Filters, f) {
				return true
			}
		}
		return false
	}
	capturesHave := func(f func(ast.FilterValue) bool) bool {
		for _, c := range captures {
			if hasFilter(c.Filters, f) {
				return true
			}
		}
		return false
	}
	isFormat := func(f ast.FilterValue) bool { _, ok := f.(ast.FilterFormat); return ok }
	isDecode := func(f ast.FilterValue) bool { _, ok := f.(ast.FilterDecode); return ok }

	for _, a := range asserts {
		if _, ok := a.Predicate.PredicateFunc.Value.(ast.PredicateFuncInclude); ok {
			l.Warning("<includes> predicate is now deprecated in favor of <contains> predicate")
			break
		}
	}
	if capturesHave(isFormat) {
		l.Warning("<format> filter in captures is now deprecated in favor of <dateFormat> filter")
	}
	if capturesHave(isDecode) {
		l.Warning("<decode> filter in captures is now deprecated in favor of <charsetDecode> filter")
	}
	if assertsHave(isFormat) {
		l.Warning("<format> filter in asserts is now deprecated in favor of <dateFormat> filter")
	}
	if assertsHave(isDecode) {
		l.Warning("<decode> filter in asserts is now deprecated in favor of <charsetDecode> filter")
	}
}

func hasFilter(filters []ast.SpacedFilter, f func(ast.FilterValue) bool) bool {
	for _, sf := range filters {
		if f(sf.Filter.Value) {
			return true
		}
	}
	return false
}
