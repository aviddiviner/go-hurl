package runner

import (
	"errors"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// Output is where response bodies are written: util.OutputFile or
// util.OutputStdout.
type Output = util.Output

// outputTryWith returns a new output given contextDir, or an
// UnauthorizedFileAccess error if the file can't be written in this context.
func outputTryWith(output Output, contextDir util.ContextDir, si ast.SourceInfo) (Output, *RunnerError) {
	o, err := output.TryWith(contextDir)
	if err != nil {
		var unauthorized *util.UnauthorizedFileAccessError
		if errors.As(err, &unauthorized) {
			return nil, NewRunnerError(si, RunnerErrorUnauthorizedFileAccess{Path: unauthorized.Path}, false)
		}
		return nil, NewRunnerError(si, RunnerErrorFileWriteAccess{Path: output.String(), Error: err.Error()}, false)
	}
	return o, nil
}
