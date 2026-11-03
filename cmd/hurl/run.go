package main

import (
	"fmt"
	"os"

	xterm "golang.org/x/term"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/cli"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/output"
	"github.com/aviddiviner/go-hurl/internal/parallel"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// runSeq runs the Hurl files sequentially, from the directory currentDir.
// The error is a cli.CliError.
func runSeq(files []input.Input, currentDir string, opts *cli.CliOptions) ([]*hurlRun, error) {
	var runs []*hurlRun
	repeat := opts.Repeat
	if repeat == nil {
		repeat = ast.CountFinite{Value: 1}
	}
	queue := newInputQueue(files, repeat)

	// When dumping HTTP responses, the first response truncates the output
	// file, and the next ones are appended.
	appendOut := false
	for {
		filename, ok := queue.next()
		if !ok {
			break
		}
		content, err := filename.ReadToString()
		if err != nil {
			return nil, cli.CliErrorInputRead{Message: fmt.Sprintf("Issue reading from %s: %s", filename, err)}
		}
		variables := runner.VariableSetFrom(opts.Variables)
		for name, value := range opts.Secrets {
			variables.InsertSecret(name, value)
		}
		runnerOptions, err := opts.ToRunnerOptions(filename, currentDir)
		if err != nil {
			return nil, err
		}
		loggerOptions := opts.ToLoggerOptions()

		// The run can only fail on a parsing error, which has already been
		// displayed.
		result, err := runner.Run(content, &filename, runnerOptions, variables, loggerOptions)
		if err != nil {
			return nil, cli.CliErrorParsing{}
		}

		// In sequential runs, standard output is written immediately.
		stdout := term.NewStdout(term.WriteModeImmediate)
		if err := printOutput(result, content, filename, opts, stdout, appendOut); err != nil {
			return nil, err
		}
		appendOut = true
		runs = append(runs, &hurlRun{content: content, filename: filename, result: result})
	}
	return runs, nil
}

// printOutput writes result either as the last HTTP response body, or as
// JSON, to the output file of opts or stdout.
func printOutput(result *runner.HurlResult, content string, filename input.Input, opts *cli.CliOptions, stdout *term.Stdout, appendOut bool) error {
	if result.Success && opts.OutputType == cli.OutputTypeResponseBody {
		err := output.WriteLastBody(result, opts.Include, opts.ColorStdout.Get(), opts.Pretty, opts.Output, stdout, appendOut)
		if err != nil {
			message := sourceerr.Render(err, filename.String(), content, nil, sourceerr.OutputFormatTerminal{Color: opts.ColorStderr.Get()})
			return cli.CliErrorOutputWrite{Message: message}
		}
	}
	if opts.OutputType == cli.OutputTypeJSON {
		if err := output.WriteJSON(result, content, filename, opts.Output, stdout, appendOut); err != nil {
			name := "stdout"
			if f, ok := opts.Output.(util.OutputFile); ok {
				name = f.Path
			}
			return cli.CliErrorOutputWrite{Message: fmt.Sprintf("%s can not be written (%s)", name, err)}
		}
	}
	return nil
}

// runPar runs the Hurl files in parallel with workersCount workers, from the
// directory currentDir. The error is a cli.CliError.
func runPar(files []input.Input, currentDir string, opts *cli.CliOptions, workersCount int) ([]*hurlRun, error) {
	// We don't need more workers than files (a repeat multiplies the number
	// of files).
	switch r := opts.Repeat.(type) {
	case ast.CountFinite:
		workersCount = min(len(files)*r.Value, workersCount)
	case nil:
		workersCount = min(len(files), workersCount)
	}
	variables := runner.VariableSetFrom(opts.Variables)
	for name, value := range opts.Secrets {
		variables.InsertSecret(name, value)
	}
	var outputType parallel.OutputType
	switch opts.OutputType {
	case cli.OutputTypeResponseBody:
		outputType = parallel.OutputTypeResponseBody{IncludeHeaders: opts.Include, Color: opts.ColorStdout.Get(), Pretty: opts.Pretty}
	case cli.OutputTypeJSON:
		outputType = parallel.OutputTypeJSON{}
	default:
		outputType = parallel.OutputTypeNoOutput{}
	}
	maxWidth := 0
	if w, _, err := xterm.GetSize(int(os.Stdout.Fd())); err == nil {
		maxWidth = w
	}
	var jobs []parallel.Job
	for seq, in := range files {
		runnerOptions, err := opts.ToRunnerOptions(in, currentDir)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, parallel.NewJob(in, seq, runnerOptions, variables, opts.ToLoggerOptions()))
	}
	repeat := opts.Repeat
	if repeat == nil {
		repeat = ast.CountFinite{Value: 1}
	}
	r := parallel.NewParallelRunner(workersCount, outputType, repeat, opts.Test, opts.ProgressBar.Get(), opts.ColorStderr.Get(), maxWidth)
	results, err := r.Run(jobs)
	if err != nil {
		switch e := err.(type) {
		case parallel.JobErrorInputRead:
			return nil, cli.CliErrorGenericIO{Message: e.Message}
		case parallel.JobErrorParsing:
			return nil, cli.CliErrorParsing{}
		case parallel.JobErrorOutputWrite:
			return nil, cli.CliErrorOutputWrite{Message: e.Message}
		}
		return nil, cli.CliErrorGenericIO{Message: err.Error()}
	}
	runs := make([]*hurlRun, 0, len(results))
	for i := range results {
		res := &results[i]
		runs = append(runs, &hurlRun{content: res.Content, filename: res.Job.Filename, result: &res.HurlResult})
	}
	return runs, nil
}

// inputQueue is a queue of inputs, which can repeat its input sequence a
// number of times, or forever.
type inputQueue struct {
	inputs      []input.Input
	index       int
	repeat      ast.Count
	repeatIndex int
}

func newInputQueue(inputs []input.Input, repeat ast.Count) *inputQueue {
	return &inputQueue{inputs: inputs, repeat: repeat}
}

func (q *inputQueue) next() (input.Input, bool) {
	if q.index < len(q.inputs) {
		q.index++
		return q.inputs[q.index-1], true
	}
	q.repeatIndex++
	if n, ok := q.repeat.(ast.CountFinite); ok && q.repeatIndex >= n.Value {
		return input.Input{}, false
	}
	if len(q.inputs) == 0 {
		return input.Input{}, false
	}
	q.index = 1
	return q.inputs[0], true
}
