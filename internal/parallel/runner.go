package parallel

import (
	"fmt"
	"slices"
	"sync"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/output"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/term"
)

// OutputType is what is written on standard output when a job is
// completed: one of OutputTypeResponseBody, OutputTypeJSON and
// OutputTypeNoOutput.
type OutputType interface{ outputType() }

// OutputTypeResponseBody outputs the last HTTP response body.
type OutputTypeResponseBody struct {
	IncludeHeaders bool
	Color          bool
	Pretty         pretty.PrettyMode
}

// OutputTypeJSON outputs the whole run as JSON.
type OutputTypeJSON struct{}

// OutputTypeNoOutput outputs nothing.
type OutputTypeNoOutput struct{}

func (OutputTypeResponseBody) outputType() {}
func (OutputTypeJSON) outputType()         {}
func (OutputTypeNoOutput) outputType()     {}

const maxRunningDisplayed = 8

// ParallelRunner runs jobs with a pool of workers. Each worker runs a Hurl
// file in its own goroutine, with buffered standard output and error, and
// reports its progression to the runner. The runner, on the calling
// goroutine, displays the outputs and the progress bar.
type ParallelRunner struct {
	workersCount int
	progress     *parProgress
	outputType   OutputType
	repeat       ast.Count
}

// NewParallelRunner creates a runner with workersCount workers.
//
// When a job is completed, its result is written on standard output
// according to outputType. The jobs sequence is run repeat times. In test
// mode, the success or failure of each file is reported on standard error,
// with a progress bar if progressBar is true, wrapped at maxWidth (0 for no
// wrapping). colorStderr enables colors on standard error.
func NewParallelRunner(workersCount int, outputType OutputType, repeat ast.Count, test, progressBar, colorStderr bool, maxWidth int) *ParallelRunner {
	return &ParallelRunner{
		workersCount: workersCount,
		progress:     newParProgress(maxRunningDisplayed, newProgressMode(test, progressBar), colorStderr, maxWidth),
		outputType:   outputType,
		repeat:       repeat,
	}
}

// Run runs jobs in parallel and returns the results ordered by job
// sequence number (the order of jobs), independently of the execution
// order. The error is a JobError.
func (r *ParallelRunner) Run(jobs []Job) ([]JobResult, error) {
	// Without workers (--repeat 0 for instance), there's nothing to run.
	if r.workersCount == 0 {
		return nil, nil
	}
	// Workers buffer their output and error; the runner displays them.
	stdout := term.NewStdout(term.WriteModeImmediate)
	stderr := term.NewStderr(term.WriteModeImmediate)

	jobsCh := make(chan Job, r.workersCount)
	messages := make(chan workerMessage, r.workersCount)
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := range r.workersCount {
		w := &worker{id: i, jobs: jobsCh, messages: messages, done: done}
		wg.Go(w.run)
	}
	states := make([]workerState, r.workersCount)

	queue := NewJobQueue(jobs, r.repeat)
	jobsCount := queue.JobsCount()
	for range r.workersCount {
		if job, ok := queue.Next(); ok {
			jobsCh <- job
		}
	}

	// On error, we stop the workers without waiting for them, as Hurl does.
	stop := func() { close(done) }

	// When dumping HTTP responses, the first response truncates the output
	// file, and the next ones are appended.
	appendOut := false
	var results []JobResult
	for msg := range messages {
		switch msg := msg.(type) {
		// On an input or parsing error, we take no more jobs and exit in
		// error, as in a sequential run.
		case inputReadErrorMsg:
			r.progress.clearProgressBar(stderr)
			stop()
			return nil, JobErrorInputRead{Message: fmt.Sprintf("Issue reading from %s: %s", msg.job.Filename, msg.err)}
		case parsingErrorMsg:
			r.progress.clearProgressBar(stderr)
			stderr.Eprint(msg.stderr)
			stop()
			return nil, JobErrorParsing{}
		case runningMsg:
			job := msg.job
			states[msg.workerID] = workerState{job: &job, currentEntry: msg.currentEntry, lastEntry: msg.lastEntry, retryCount: msg.retryCount}
			// We can receive a lot of running messages: the progress bar is
			// throttled to avoid flickering.
			if r.progress.canUpdate() {
				r.progress.clearProgressBar(stderr)
				r.progress.updateProgressBar(states, len(results), jobsCount, stderr)
			}
		case completedMsg:
			// On completion, the progress bar is always refreshed.
			r.progress.clearProgressBar(stderr)
			states[msg.workerID] = workerState{}

			if len(msg.stdout) > 0 {
				if err := stdout.WriteAll(msg.stdout); err != nil {
					stop()
					return nil, JobErrorOutputWrite{Message: "Issue writing to stdout"}
				}
			}
			if msg.stderr != "" {
				stderr.Eprint(msg.stderr)
			}
			if err := r.printOutput(&msg.result, stdout, appendOut); err != nil {
				stop()
				return nil, err
			}
			appendOut = true

			r.progress.printCompleted(&msg.result, stderr)
			results = append(results, msg.result)
			r.progress.updateProgressBar(states, len(results), jobsCount, stderr)
			// The next running message must refresh the progress bar, so the
			// new jobs are displayed.
			r.progress.forceNextUpdate()

			if job, ok := queue.Next(); ok {
				jobsCh <- job
			} else if n, ok := jobsCount.(ast.CountFinite); ok && len(results) == n.Value {
				close(jobsCh)
				wg.Wait()
				slices.SortStableFunc(results, func(a, b JobResult) int { return a.Job.Seq - b.Job.Seq })
				return results, nil
			}
		}
	}
	return results, nil
}

// printOutput writes a job result on standard output, either as the last
// HTTP response body, or as JSON. With appendOut, an existing output file is
// appended instead of truncated.
func (r *ParallelRunner) printOutput(result *JobResult, stdout *term.Stdout, appendOut bool) error {
	job := &result.Job
	switch o := r.outputType.(type) {
	case OutputTypeResponseBody:
		if !result.HurlResult.Success {
			return nil
		}
		err := output.WriteLastBody(&result.HurlResult, o.IncludeHeaders, o.Color, o.Pretty, job.RunnerOptions.Output, stdout, appendOut)
		if err != nil {
			message := sourceerr.Render(err, job.Filename.String(), result.Content, nil, sourceerr.OutputFormatTerminal{Color: o.Color})
			return JobErrorOutputWrite{Message: message}
		}
	case OutputTypeJSON:
		if err := output.WriteJSON(&result.HurlResult, result.Content, job.Filename, job.RunnerOptions.Output, stdout, appendOut); err != nil {
			return JobErrorOutputWrite{Message: err.Error()}
		}
	}
	return nil
}
