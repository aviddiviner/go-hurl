// Package output serializes a Hurl run result to standard output or a
// file, ported from hurl/src/output: either the last response body (raw),
// or the whole run as JSON.
package output

import (
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// WriteLastBody writes the last response of result to out (standard output
// if nil). With includeHeaders, the response headers are written before the
// body. With appendOut, an existing file is appended instead of truncated.
// The body is prettified according to prettyMode.
//
// The error (Rust's OutputError) renders as a runner error.
func WriteLastBody(result *runner.HurlResult, includeHeaders, color bool, prettyMode pretty.PrettyMode, out util.Output, stdout *term.Stdout, appendOut bool) *runner.RunnerError {
	if len(result.Entries) == 0 {
		return nil
	}
	last := &result.Entries[len(result.Entries)-1]
	return last.WriteResponse(out, stdout, includeHeaders, color, prettyMode, appendOut, last.SourceInfo)
}

// WriteJSON writes the JSON representation of result to out (standard
// output if nil). content and filename are the source of the Hurl file,
// used to build the errors of failed asserts and captures.
func WriteJSON(result *runner.HurlResult, content string, filename input.Input, out util.Output, stdout *term.Stdout, appendOut bool) error {
	// Secrets are only redacted from standard error and reports: the
	// structured output may be used for processing, so it's kept as is.
	v, err := result.ToJSON(content, filename, "", nil)
	if err != nil {
		return err
	}
	b := append(runner.EncodeJSON(v), '\n')
	if out == nil {
		out = util.OutputStdout{}
	}
	return out.Write(b, stdout, appendOut)
}
