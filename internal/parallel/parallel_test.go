package parallel

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/text"
)

func newJob(file string, seq int) Job {
	return NewJob(input.New(file), seq, runner.DefaultRunnerOptions(), runner.NewVariableSet(), logger.LoggerOptions{})
}

func assertNext(t *testing.T, q *JobQueue, file string, seq int) {
	t.Helper()
	job, ok := q.Next()
	if !ok || job.Filename.String() != file || job.Seq != seq {
		t.Errorf("got %v %d %v, want %s %d", job.Filename, job.Seq, ok, file, seq)
	}
}

func TestJobQueueIsFinite(t *testing.T) {
	jobs := []Job{newJob("a.hurl", 0), newJob("b.hurl", 1), newJob("c.hurl", 2)}
	q := NewJobQueue(jobs, ast.CountFinite{Value: 2})
	assertNext(t, q, "a.hurl", 0)
	assertNext(t, q, "b.hurl", 1)
	assertNext(t, q, "c.hurl", 2)
	assertNext(t, q, "a.hurl", 3)
	assertNext(t, q, "b.hurl", 4)
	assertNext(t, q, "c.hurl", 5)
	if _, ok := q.Next(); ok {
		t.Error("expected the end of the queue")
	}
	if q.JobsCount() != (ast.CountFinite{Value: 6}) {
		t.Errorf("got %v", q.JobsCount())
	}
}

func TestJobQueueIsInfinite(t *testing.T) {
	q := NewJobQueue([]Job{newJob("foo.hurl", 0)}, ast.CountInfinite{})
	for i := range 5 {
		assertNext(t, q, "foo.hurl", i)
	}
	if q.JobsCount() != (ast.CountInfinite{}) {
		t.Errorf("got %v", q.JobsCount())
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		current, last int
		want          string
	}{
		{1, 20, "[>                       ] 1/20"},
		{2, 20, "[=>                      ] 2/20"},
		{5, 20, "[====>                   ] 5/20"},
		{10, 20, "[==========>             ] 10/20"},
		{15, 20, "[================>       ] 15/20"},
		{20, 20, "[======================> ] 20/20"},
		{1, 3, "[>                       ] 1/3"},
		{2, 3, "[========>               ] 2/3"},
		{3, 3, "[================>       ] 3/3"},
		{1, 1, "[>                       ] 1/1"},
	}
	for _, tt := range tests {
		if got := progressBar(ast.NewIndex(tt.current), ast.NewIndex(tt.last)); got != tt.want {
			t.Errorf("progressBar(%d, %d) = %q, want %q", tt.current, tt.last, got, tt.want)
		}
	}
}

func TestAllWorkersRunning(t *testing.T) {
	var jobs []Job
	for i, f := range []string{"a.hurl", "b.hurl", "c.hurl", "d.hurl", "e.hurl", "f.hurl", "g.hurl"} {
		jobs = append(jobs, newJob(f, i))
	}
	running := func(job int, current, last, retry int) workerState {
		return workerState{job: &jobs[job], currentEntry: ast.NewIndex(current), lastEntry: ast.NewIndex(last), retryCount: retry}
	}
	idle := workerState{}
	total := ast.CountFinite{Value: 100}
	build := func(workers ...workerState) string {
		s, ok := buildProgress(workers, 75, total, 3, text.FormatPlain, 0)
		if !ok {
			return "<none>"
		}
		return s
	}
	tests := []struct {
		workers []workerState
		want    string
	}{
		{[]workerState{idle, idle, idle, idle, idle}, "<none>"},
		{[]workerState{running(0, 1, 10, 0), running(1, 1, 2, 0), running(2, 1, 5, 0), running(3, 1, 7, 0), running(4, 1, 4, 0)},
			"Executed files: 75/100 (75%)\n" +
				"[>                       ] 1/10  Running a.hurl\n" +
				"[>                       ] 1/2   Running b.hurl\n" +
				"[>                       ] 1/5   Running c.hurl\n" +
				"...2 more\n"},
		{[]workerState{running(0, 6, 10, 0), running(1, 2, 2, 0), running(2, 3, 5, 0), running(3, 4, 7, 0), running(4, 2, 4, 0)},
			"Executed files: 75/100 (75%)\n" +
				"[============>           ] 6/10  Running a.hurl\n" +
				"[============>           ] 2/2   Running b.hurl\n" +
				"[=========>              ] 3/5   Running c.hurl\n" +
				"...2 more\n"},
		{[]workerState{running(0, 10, 10, 0), running(5, 1, 6, 0), running(2, 5, 5, 0), running(3, 6, 7, 0), running(4, 3, 4, 0)},
			"Executed files: 75/100 (75%)\n" +
				"[=====================>  ] 10/10 Running a.hurl\n" +
				"[===================>    ] 5/5   Running c.hurl\n" +
				"[=================>      ] 6/7   Running d.hurl\n" +
				"...2 more\n"},
		{[]workerState{idle, running(5, 3, 6, 0), idle, idle, running(4, 4, 4, 0)},
			"Executed files: 75/100 (75%)\n" +
				"[==================>     ] 4/4 Running e.hurl\n" +
				"[========>               ] 3/6 Running f.hurl\n"},
		{[]workerState{idle, running(5, 6, 6, 0), idle, idle, idle},
			"Executed files: 75/100 (75%)\n" +
				"[====================>   ] 6/6 Running f.hurl\n"},
		{[]workerState{idle, running(5, 6, 6, 1), idle, idle, idle},
			"Executed files: 75/100 (75%)\n" +
				"[====================>   ] 6/6 Running f.hurl (retry 1)\n"},
	}
	for i, tt := range tests {
		if got := build(tt.workers...); got != tt.want {
			t.Errorf("case %d:\ngot:\n%s\nwant:\n%s", i, got, tt.want)
		}
	}
}

func TestParallelRunnerOrdersResults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.Path)
	}))
	defer srv.Close()
	dir := t.TempDir()
	var jobs []Job
	for i := range 6 {
		path := filepath.Join(dir, fmt.Sprintf("%d.hurl", i))
		content := fmt.Sprintf("GET %s/%d\nHTTP 200\n[Asserts]\nbody == \"/%d\"\n", srv.URL, i, i)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, newJob(path, i))
	}
	r := NewParallelRunner(3, OutputTypeNoOutput{}, ast.CountFinite{Value: 2}, false, false, false, 0)
	results, err := r.Run(jobs)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 12 {
		t.Fatalf("got %d results", len(results))
	}
	for i, res := range results {
		if res.Job.Seq != i || !res.HurlResult.Success {
			t.Errorf("result %d: seq %d, success %v", i, res.Job.Seq, res.HurlResult.Success)
		}
	}
}

func TestParallelRunnerParsingError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.hurl")
	if err := os.WriteFile(path, []byte("GET\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewParallelRunner(2, OutputTypeNoOutput{}, ast.CountFinite{Value: 1}, false, false, false, 0)
	_, err := r.Run([]Job{newJob(path, 0), newJob(path, 1)})
	if _, ok := err.(JobErrorParsing); !ok {
		t.Errorf("got %v", err)
	}
}
