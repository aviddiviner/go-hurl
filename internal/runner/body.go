package runner

import (
	"os"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// evalBody evaluates a request body.
func evalBody(body ast.Body, variables *VariableSet, contextDir util.ContextDir) (httpclient.Body, *RunnerError) {
	return evalBytes(body.Value, variables, contextDir)
}

func evalBytes(b ast.Bytes, variables *VariableSet, contextDir util.ContextDir) (httpclient.Body, *RunnerError) {
	switch b := b.(type) {
	case ast.BytesOnelineString:
		s, err := evalTemplate(b.Value, variables)
		if err != nil {
			return nil, err
		}
		return httpclient.BodyText{Value: s}, nil
	case ast.BytesMultilineString:
		s, err := evalMultiline(b.Value, variables)
		if err != nil {
			return nil, err
		}
		return httpclient.BodyText{Value: s}, nil
	case ast.BytesXML:
		return httpclient.BodyText{Value: b.Value}, nil
	case ast.BytesJSON:
		s, err := evalJSONValue(b.Value, variables, true)
		if err != nil {
			return nil, err
		}
		return httpclient.BodyText{Value: s}, nil
	case ast.BytesBase64:
		return httpclient.BodyBinary{Value: b.Value.Value}, nil
	case ast.BytesHex:
		return httpclient.BodyBinary{Value: b.Value.Value}, nil
	case ast.BytesFile:
		value, err := evalFile(b.Value.Filename, variables, contextDir)
		if err != nil {
			return nil, err
		}
		filename, err := evalTemplate(b.Value.Filename, variables)
		if err != nil {
			return nil, err
		}
		return httpclient.BodyFile{Value: value, Filename: filename}, nil
	}
	panic("unknown bytes")
}

// evalFile reads the file named by a template. To avoid leaking private
// data, the file must be inside the context directory (--file-root).
func evalFile(filename ast.Template, variables *VariableSet, contextDir util.ContextDir) ([]byte, *RunnerError) {
	path, err := evalTemplate(filename, variables)
	if err != nil {
		return nil, err
	}
	if !contextDir.IsAccessAllowed(path) {
		return nil, NewRunnerError(filename.SourceInfo, RunnerErrorUnauthorizedFileAccess{Path: path}, false)
	}
	data, rerr := os.ReadFile(contextDir.ResolvedPath(path))
	if rerr != nil {
		return nil, NewRunnerError(filename.SourceInfo, RunnerErrorFileReadAccess{Path: path}, false)
	}
	return data, nil
}
