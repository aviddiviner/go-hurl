package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

// CaptureResult is a capture of an HTTP response: data extracted by
// querying the response, that can be reused in the next requests.
type CaptureResult struct {
	Name  string
	Value Value
}

// evalCapture evaluates a capture against a list of HTTP responses. cache
// holds the parsed XML/JSON response bodies.
func evalCapture(capture ast.Capture, variables *VariableSet, responses []*httpclient.Response, cache *bodyCache, options QueryOptions) (CaptureResult, *RunnerError) {
	name, err := evalTemplate(capture.Name, variables)
	if err != nil {
		return CaptureResult{}, err
	}
	value, err := evalQuery(capture.Query, variables, responses, cache, options)
	if err != nil {
		return CaptureResult{}, err
	}
	if value == nil {
		return CaptureResult{}, NewRunnerError(capture.Query.SourceInfo, RunnerErrorNoQueryResult{}, false)
	}
	fs := filters(capture.Filters)
	filterOptions := FilterOptions{UseJSONPathCoercion: options.UseJSONPathCoercion}
	value, err = evalFilters(fs, value, variables, false, filterOptions)
	if err != nil {
		return CaptureResult{}, err
	}
	if value == nil {
		// We don't know which filter in the chain first returns no value, so
		// the whole chain is reported.
		si := ast.SourceInfo{Start: fs[0].SourceInfo.Start, End: fs[len(fs)-1].SourceInfo.End}
		return CaptureResult{}, NewRunnerError(si, RunnerErrorNoFilterResult{}, false)
	}
	return CaptureResult{Name: name, Value: value}, nil
}
