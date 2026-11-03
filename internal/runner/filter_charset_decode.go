package runner

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

// evalCharsetDecode decodes bytes to a string using an encoding (a WHATWG
// label, as encoding_rs's Encoding::for_label). Malformed input is an error.
func evalCharsetDecode(value Value, encoding ast.Template, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	label, err := evalTemplate(encoding, variables)
	if err != nil {
		return nil, err
	}
	b, ok := value.(ValueBytes)
	if !ok {
		return nil, invalidInputType(value, "bytes", si, assert)
	}
	charset := httpclient.CharsetForLabel(label)
	if charset == nil {
		return nil, NewRunnerError(si, RunnerErrorFilterInvalidEncoding{Value: label}, assert)
	}
	decoded, ok := charset.Decode(b.Value)
	if !ok {
		return nil, NewRunnerError(si, RunnerErrorFilterDecode{Value: label}, assert)
	}
	return ValueString{decoded}, nil
}
