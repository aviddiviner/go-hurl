package runner

import (
	"bytes"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/reader"
)

// HurlResult is the result of running a valid Hurl file.
type HurlResult struct {
	// Entries are the entry results of this run (retried entries included).
	Entries []EntryResult
	// Duration is the total duration of the run, including asserts and
	// captures.
	Duration time.Duration
	// Success is false if there have been runtime or assert errors.
	Success bool
	// CookieStore holds the cookies at the end of the run.
	CookieStore *httpclient.CookieStore
	// Timestamp is the start of the run (UNIX timestamp, in seconds).
	Timestamp int64
	// Variables are the variables at the end of the run.
	Variables *VariableSet
}

// EntryError is an error of an entry, with the source information of the
// entry.
type EntryError struct {
	Error      *RunnerError
	SourceInfo ast.SourceInfo
}

// Errors returns the effective errors of this result, with the source
// information of their entry. Errors of entries that have been retried are
// ignored.
func (r *HurlResult) Errors() []EntryError {
	var errors []EntryError
	for i, entry := range r.Entries {
		if i+1 < len(r.Entries) && r.Entries[i+1].EntryIndex == entry.EntryIndex {
			continue
		}
		for _, err := range entry.Errors {
			errors = append(errors, EntryError{Error: err, SourceInfo: entry.SourceInfo})
		}
	}
	return errors
}

// EntryResult is the result of running an entry.
type EntryResult struct {
	// EntryIndex is the index of the entry in the file.
	EntryIndex ast.Index
	SourceInfo ast.SourceInfo
	// Calls are the HTTP requests and responses executed, redirections
	// included. Without --location, there is exactly one call.
	Calls    []httpclient.Call
	Captures []CaptureResult
	Asserts  []AssertResult
	Errors   []*RunnerError
	// TransferDuration is the duration of all the HTTP transfers, excluding
	// asserts and captures.
	TransferDuration time.Duration
	// Compressed is set when the entry has been run with --compressed: the
	// response should be uncompressed when written to standard output.
	Compressed bool
	// CurlCmd is the debug curl command line of this entry.
	CurlCmd httpclient.CurlCmd
}

func newEntryResult(entryIndex ast.Index, sourceInfo ast.SourceInfo) EntryResult {
	return EntryResult{EntryIndex: entryIndex, SourceInfo: sourceInfo}
}

// DefaultEntryResult returns an empty entry result for the first entry.
func DefaultEntryResult() EntryResult {
	pos := reader.Pos{Line: 1, Column: 1}
	return newEntryResult(ast.NewIndex(1), ast.SourceInfo{Start: pos, End: pos})
}

// WriteResponse writes the last HTTP response of this entry to output
// (standard output if nil). The response is uncompressed if the entry has
// been run with --compressed. Write access must be checked by the caller.
func (e *EntryResult) WriteResponse(output Output, stdout *term.Stdout, includeHeaders, color bool, prettyMode pretty.PrettyMode, append bool, si ast.SourceInfo) *RunnerError {
	if len(e.Calls) == 0 {
		return nil
	}
	response := &e.Calls[len(e.Calls)-1].Response
	var out bytes.Buffer

	if includeHeaders {
		out.WriteString(response.GetStatusLineHeaders(color))
		out.WriteByte('\n')
	}

	body := response.Body
	if e.Compressed {
		var err httpclient.HttpError
		if body, err = response.UncompressBody(); err != nil {
			return NewRunnerError(si, RunnerErrorHTTP{Value: err}, false)
		}
	}

	// Only JSON-like responses are prettified for the moment.
	doPretty := false
	switch prettyMode {
	case pretty.Automatic:
		doPretty = response.IsJSON()
	case pretty.Force:
		doPretty = true
	}
	if doPretty {
		colorPretty := pretty.NoColor
		if color {
			colorPretty = pretty.Ansi
		}
		// On pretty-print error, discard partial output and fall back to raw
		// bytes.
		before := out.Len()
		if err := pretty.Format(body, colorPretty, &out); err != nil {
			out.Truncate(before)
			out.Write(body)
		}
	} else {
		out.Write(body)
	}

	// Like curl, warn about binary output on a terminal when the user hasn't
	// used --output.
	if output == nil && stdout.IsTerminal() && isBinary(out.Bytes()) {
		return NewRunnerError(si, RunnerErrorBinaryOutput{}, false)
	}

	if output == nil {
		output = util.OutputStdout{}
	}
	if err := output.Write(out.Bytes(), stdout, append); err != nil {
		return NewRunnerError(si, RunnerErrorFileWriteAccess{Path: output.String(), Error: err.Error()}, false)
	}
	return nil
}

// HasResponseTrailingNewline reports whether the last HTTP response body
// ends with a newline.
func (e *EntryResult) HasResponseTrailingNewline() bool {
	if len(e.Calls) == 0 {
		return false
	}
	return e.Calls[len(e.Calls)-1].Response.HasTrailingNewline()
}

// isBinary is curl's heuristic for binary content: a NUL in the first 2000
// bytes.
func isBinary(b []byte) bool {
	return bytes.IndexByte(b[:min(2000, len(b))], 0) >= 0
}
