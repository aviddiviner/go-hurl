package parallel

import (
	"fmt"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/parser"
)

// workerMessage is a message sent from a worker to the runner: one of
// inputReadErrorMsg, parsingErrorMsg, runningMsg and completedMsg.
type workerMessage interface{ workerMessage() }

// inputReadErrorMsg is sent when the file can't be read.
type inputReadErrorMsg struct {
	workerID int
	job      Job
	err      error
}

// parsingErrorMsg is sent when the file isn't a valid Hurl file. stderr is
// the standard error of the worker for this job.
type parsingErrorMsg struct {
	workerID int
	job      Job
	stderr   string
}

// runningMsg is sent regularly while the job is run.
type runningMsg struct {
	workerID     int
	job          Job
	currentEntry ast.Index
	lastEntry    ast.Index
	retryCount   int
}

// completedMsg is sent when the Hurl file has been run, successfully or
// not, with the buffered standard output and error of the job.
type completedMsg struct {
	workerID int
	result   JobResult
	stdout   []byte
	stderr   string
}

func (inputReadErrorMsg) workerMessage() {}
func (parsingErrorMsg) workerMessage()   {}
func (runningMsg) workerMessage()        {}
func (completedMsg) workerMessage()      {}

// worker runs jobs received on jobs in its own goroutine, and reports to
// the runner on messages. It stops when jobs is closed or done is closed,
// and, as in Hurl, after an input or parsing error.
type worker struct {
	id       int
	jobs     <-chan Job
	messages chan<- workerMessage
	done     <-chan struct{}
}

func (w *worker) send(msg workerMessage) bool {
	select {
	case w.messages <- msg:
		return true
	case <-w.done:
		return false
	}
}

func (w *worker) run() {
	for {
		var job Job
		var ok bool
		select {
		case job, ok = <-w.jobs:
			if !ok {
				return
			}
		case <-w.done:
			return
		}
		if !w.runJob(job) {
			return
		}
	}
}

// runJob runs a job, and returns false if the worker must stop.
func (w *worker) runJob(job Job) bool {
	// In parallel runs, standard output and error are buffered (in
	// sequential runs, they're written immediately).
	stdout := term.NewStdout(term.WriteModeBuffered)
	stderr := term.NewStderr(term.WriteModeBuffered)
	l := logger.New(job.LoggerOptions, stderr, job.Variables.Secrets())
	progress := &workerProgress{worker: w, job: job}

	content, err := job.Filename.ReadToString()
	if err != nil {
		w.send(inputReadErrorMsg{workerID: w.id, job: job, err: err})
		return false
	}
	hurlFile, perr := parser.ParseHurlFile(content)
	if perr != nil {
		message := sourceerr.Render(perr, job.Filename.String(), content, nil, sourceerr.OutputFormatTerminal{Color: l.Color})
		l.ErrorRich(message)
		w.send(parsingErrorMsg{workerID: w.id, job: job, stderr: l.Stderr.Buffer()})
		return false
	}
	filename := job.Filename
	result := runner.RunEntries(hurlFile.Entries, content, &filename, job.RunnerOptions, job.Variables, stdout, progress, l)
	if result.Success && len(result.Entries) == 0 {
		l.Warning(fmt.Sprintf("No entry have been executed for file %s", job.Filename))
	}
	return w.send(completedMsg{
		workerID: w.id,
		result:   JobResult{Job: job, Content: content, HurlResult: result},
		stdout:   stdout.Buffer(),
		stderr:   l.Stderr.Buffer(),
	})
}

// workerProgress reports the progression of a job to the runner.
type workerProgress struct {
	worker *worker
	job    Job
}

func (p *workerProgress) OnEntryRunning(current, last ast.Index, retryCount int) {
	p.worker.send(runningMsg{workerID: p.worker.id, job: p.job, currentEntry: current, lastEntry: last, retryCount: retryCount})
}
