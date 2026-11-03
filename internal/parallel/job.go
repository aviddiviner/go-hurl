// Package parallel runs Hurl files in parallel, ported from
// hurl/src/parallel. Workers are goroutines receiving jobs on a channel and
// reporting their progression on another one; the runner, on the calling
// goroutine, displays outputs and the progress bar.
package parallel

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/runner"
)

// Job is a Hurl file to run, with its options.
type Job struct {
	Filename      input.Input
	RunnerOptions runner.RunnerOptions
	// Variables injected in the Hurl file.
	Variables     *runner.VariableSet
	LoggerOptions logger.LoggerOptions
	// Seq is the 0-based index of the job in the jobs list.
	Seq int
}

// NewJob creates a new job.
func NewJob(filename input.Input, seq int, runnerOptions runner.RunnerOptions, variables *runner.VariableSet, loggerOptions logger.LoggerOptions) Job {
	return Job{
		Filename:      filename,
		RunnerOptions: runnerOptions,
		Variables:     variables.Clone(),
		LoggerOptions: loggerOptions,
		Seq:           seq,
	}
}

// JobResult is the result of a job.
type JobResult struct {
	Job Job
	// Content is the source content of the job.
	Content    string
	HurlResult runner.HurlResult
}

// JobError is an error when running a job: one of JobErrorInputRead,
// JobErrorParsing and JobErrorOutputWrite.
type JobError interface {
	error
	jobError()
}

type JobErrorInputRead struct{ Message string }

// JobErrorParsing is a parse error of a Hurl file, already displayed.
type JobErrorParsing struct{}
type JobErrorOutputWrite struct{ Message string }

func (JobErrorInputRead) jobError()   {}
func (JobErrorParsing) jobError()     {}
func (JobErrorOutputWrite) jobError() {}

func (e JobErrorInputRead) Error() string   { return e.Message }
func (JobErrorParsing) Error() string       { return "" }
func (e JobErrorOutputWrite) Error() string { return e.Message }

// JobQueue is a queue of jobs, which can repeat its input sequence a number
// of times, or forever.
type JobQueue struct {
	jobs        []Job
	index       int
	repeat      ast.Count
	repeatIndex int
}

// NewJobQueue creates a queue of jobs, repeated repeat times.
func NewJobQueue(jobs []Job, repeat ast.Count) *JobQueue {
	return &JobQueue{jobs: jobs, repeat: repeat}
}

// JobsCount returns the effective total number of jobs.
func (q *JobQueue) JobsCount() ast.Count {
	if n, ok := q.repeat.(ast.CountFinite); ok {
		return ast.CountFinite{Value: len(q.jobs) * n.Value}
	}
	return ast.CountInfinite{}
}

// jobAt returns the job at index: a repeated job gets a sequence number
// relative to the current repeat index.
func (q *JobQueue) jobAt(index int) Job {
	job := q.jobs[index]
	job.Seq = q.jobs[index].Seq + len(q.jobs)*q.repeatIndex
	return job
}

// Next returns the next job, or false if the queue is exhausted.
func (q *JobQueue) Next() (Job, bool) {
	if q.index < len(q.jobs) {
		q.index++
		return q.jobAt(q.index - 1), true
	}
	q.repeatIndex++
	if n, ok := q.repeat.(ast.CountFinite); ok && q.repeatIndex >= n.Value {
		return Job{}, false
	}
	if len(q.jobs) == 0 {
		return Job{}, false
	}
	q.index = 1
	return q.jobAt(0), true
}
