package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// evalVersionStatusAsserts returns the implicit asserts on the response
// HTTP version and status code.
func evalVersionStatusAsserts(response *ast.Response, httpResponse *httpclient.Response) []AssertResult {
	asserts := []AssertResult{AssertResultImplicitVersion{
		Actual:     httpResponse.Version.String(),
		Expected:   response.Version.Value.String(),
		SourceInfo: response.Version.SourceInfo,
	}}
	if s, ok := response.Status.Value.(ast.StatusSpecific); ok {
		asserts = append(asserts, AssertResultImplicitStatus{
			Actual:     uint64(httpResponse.Status),
			Expected:   s.Value,
			SourceInfo: response.Status.SourceInfo,
		})
	}
	return asserts
}

// evalAsserts returns the implicit asserts on headers and body, and the
// explicit asserts of a response spec. Version and status asserts are
// evaluated by evalVersionStatusAsserts.
func evalAsserts(response *ast.Response, variables *VariableSet, responses []*httpclient.Response, cache *bodyCache, contextDir util.ContextDir, options QueryOptions) []AssertResult {
	var asserts []AssertResult
	last := responses[len(responses)-1]

	for _, header := range response.Headers {
		expected, err := evalTemplate(header.Value, variables)
		if err != nil {
			asserts = append(asserts, AssertResultImplicitHeader{ActualErr: err, SourceInfo: header.Key.SourceInfo})
			continue
		}
		name, err := evalTemplate(header.Key, variables)
		if err != nil {
			asserts = append(asserts, AssertResultImplicitHeader{ActualErr: err, Expected: expected, SourceInfo: header.Value.SourceInfo})
			continue
		}
		actuals := last.Headers.Values(name)
		switch len(actuals) {
		case 0:
			err := NewRunnerError(header.Key.SourceInfo, RunnerErrorQueryHeaderNotFound{}, false)
			asserts = append(asserts, AssertResultImplicitHeader{ActualErr: err, Expected: expected, SourceInfo: header.Key.SourceInfo})
		case 1:
			asserts = append(asserts, AssertResultImplicitHeader{Actual: actuals[0], Expected: expected, SourceInfo: header.Value.SourceInfo})
		default:
			// Failure by default: if the expected value isn't found, actual
			// is the full list.
			actual := "["
			for i, v := range actuals {
				if i > 0 {
					actual += ", "
				}
				actual += `"` + v + `"`
			}
			actual += "]"
			for _, v := range actuals {
				if v == expected {
					actual = v
					break
				}
			}
			asserts = append(asserts, AssertResultImplicitHeader{Actual: actual, Expected: expected, SourceInfo: header.Value.SourceInfo})
		}
	}

	if response.Body != nil {
		asserts = append(asserts, evalImplicitBodyAsserts(*response.Body, variables, last, contextDir))
	}

	for _, assert := range response.Asserts() {
		asserts = append(asserts, evalExplicitAssert(assert, variables, responses, cache, contextDir, options))
	}
	return asserts
}

// evalImplicitBodyAsserts checks the body of an actual HTTP response against
// a spec body.
func evalImplicitBodyAsserts(specBody ast.Body, variables *VariableSet, httpResponse *httpclient.Response, contextDir util.ContextDir) AssertResult {
	errorPos := specBody.Space0.SourceInfo.End
	httpErr := func(e httpclient.HttpError) *RunnerError {
		return NewRunnerError(ast.SourceInfo{Start: errorPos, End: errorPos}, RunnerErrorHTTP{Value: e}, true)
	}
	text := func(a *AssertResultImplicitBody) {
		if s, err := httpResponse.Text(); err != nil {
			a.ActualErr = httpErr(err)
		} else {
			a.Actual = NewString(s)
		}
	}
	bytes := func(a *AssertResultImplicitBody) {
		if b, err := httpResponse.UncompressBody(); err != nil {
			a.ActualErr = httpErr(err)
		} else {
			a.Actual = ValueBytes{b}
		}
	}
	expectString := func(a *AssertResultImplicitBody, s string, err *RunnerError) {
		if err != nil {
			a.ExpectedErr = err
		} else {
			a.Expected = NewString(s)
		}
	}

	a := AssertResultImplicitBody{SourceInfo: specBody.Space0.SourceInfo}
	switch b := specBody.Value.(type) {
	case ast.BytesJSON:
		s, err := evalJSONValue(b.Value, variables, true)
		expectString(&a, s, err)
		text(&a)
	case ast.BytesXML:
		a.Expected = NewString(b.Value)
		text(&a)
	case ast.BytesOnelineString:
		s, err := evalTemplate(b.Value, variables)
		expectString(&a, s, err)
		text(&a)
		a.SourceInfo = b.Value.SourceInfo
	case ast.BytesMultilineString:
		s, err := evalMultiline(b.Value, variables)
		expectString(&a, s, err)
		text(&a)
		a.SourceInfo = b.Value.Value().SourceInfo
	case ast.BytesBase64:
		a.Expected = ValueBytes{b.Value.Value}
		bytes(&a)
		a.SourceInfo = ast.SourceInfo{Start: b.Value.Space0.SourceInfo.End, End: b.Value.Space1.SourceInfo.Start}
	case ast.BytesHex:
		a.Expected = ValueBytes{b.Value.Value}
		bytes(&a)
		a.SourceInfo = ast.SourceInfo{Start: b.Value.Space0.SourceInfo.End, End: b.Value.Space1.SourceInfo.Start}
	case ast.BytesFile:
		if body, err := evalBody(specBody, variables, contextDir); err != nil {
			a.ExpectedErr = err
		} else {
			a.Expected = ValueBytes{body.Bytes()}
		}
		bytes(&a)
	}
	return a
}

// evalCaptures evaluates the captures of a response spec, and inserts them
// in variables as they're computed, so a capture can use the previous ones.
func evalCaptures(response *ast.Response, responses []*httpclient.Response, cache *bodyCache, variables *VariableSet, options QueryOptions) ([]CaptureResult, *RunnerError) {
	var captures []CaptureResult
	for _, capture := range response.Captures() {
		result, err := evalCapture(capture, variables, responses, cache, options)
		if err != nil {
			return nil, err
		}
		// Only secret strings are supported.
		if capture.Redacted {
			s, ok := result.Value.(ValueString)
			if !ok {
				kind := RunnerErrorUnsupportedSecretType{Value: result.Value.Kind().String()}
				return nil, NewRunnerError(capture.Name.SourceInfo, kind, false)
			}
			variables.InsertSecret(result.Name, s.Value)
		} else {
			variables.Insert(result.Name, result.Value)
		}
		captures = append(captures, result)
	}
	return captures, nil
}
