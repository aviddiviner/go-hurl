package util

import (
	"fmt"
	"os"

	"github.com/aviddiviner/go-hurl/internal/term"
)

// Output is where bytes are written: OutputFile or OutputStdout.
type Output interface {
	output()
	String() string
	// TryWith returns a new output given contextDir. If the output is a file
	// that can't be written given contextDir, it returns an
	// *UnauthorizedFileAccessError.
	TryWith(contextDir ContextDir) (Output, error)
	// Write writes bytes to the output. stdout is used for OutputStdout. If
	// append is true, a file output is opened in append mode, otherwise it's
	// truncated.
	Write(bytes []byte, stdout *term.Stdout, append bool) error
}

// OutputFile writes to a file.
type OutputFile struct{ Path string }

// OutputStdout writes to standard output.
type OutputStdout struct{}

func (OutputFile) output()   {}
func (OutputStdout) output() {}

func (o OutputFile) String() string { return o.Path }
func (OutputStdout) String() string { return "-" }

// NewOutput creates an output from a filename, "-" being standard output.
func NewOutput(filename string) Output {
	if filename == "-" {
		return OutputStdout{}
	}
	return OutputFile{Path: filename}
}

// UnauthorizedFileAccessError is returned by Output.TryWith; the runner maps
// it to its UnauthorizedFileAccess error kind.
type UnauthorizedFileAccessError struct{ Path string }

func (e *UnauthorizedFileAccessError) Error() string {
	return fmt.Sprintf("unauthorized access to file %s", e.Path)
}

func (o OutputStdout) TryWith(ContextDir) (Output, error) { return o, nil }

func (o OutputFile) TryWith(contextDir ContextDir) (Output, error) {
	if !contextDir.IsAccessAllowed(o.Path) {
		return nil, &UnauthorizedFileAccessError{Path: o.Path}
	}
	return OutputFile{Path: contextDir.ResolvedPath(o.Path)}, nil
}

func (OutputStdout) Write(bytes []byte, stdout *term.Stdout, _ bool) error {
	if err := stdout.WriteAll(bytes); err != nil {
		return fmt.Errorf("stdout can not be written (%v)", err)
	}
	return nil
}

func (o OutputFile) Write(bytes []byte, _ *term.Stdout, append bool) error {
	flags := os.O_CREATE | os.O_WRONLY
	if append {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(o.Path, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(bytes); err != nil {
		return fmt.Errorf("%s can not be written (%v)", o.Path, err)
	}
	return nil
}
