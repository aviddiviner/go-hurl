package parallel

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/text"
)

// workerState is the state of a worker: idle if job is nil, else running
// the entry currentEntry of job (lastEntry is the last entry to be run).
type workerState struct {
	job          *Job
	currentEntry ast.Index
	lastEntry    ast.Index
	retryCount   int
}

// progressMode is the mode of the progress reporter.
type progressMode int

const (
	// modeDefault is a run without --test.
	modeDefault progressMode = iota
	// modeTestWithProgress is a run with --test and a progress bar.
	modeTestWithProgress
	// modeTestWithoutProgress is a run with --test and no progress bar.
	modeTestWithoutProgress
)

func newProgressMode(test, progressBar bool) progressMode {
	switch {
	case test && progressBar:
		return modeTestWithProgress
	case test:
		return modeTestWithoutProgress
	}
	return modeDefault
}

const (
	// updateInterval is the minimum duration between two progress bar
	// redraws (to avoid flickering).
	updateInterval = 100 * time.Millisecond
	// firstThrottle is the duration before the progress bar is throttled,
	// to let the UI stabilize.
	firstThrottle = 16 * time.Millisecond
)

// parProgress displays the advancement of parallel runs in test mode.
type parProgress struct {
	maxRunningDisplayed int
	mode                progressMode
	format              text.Format
	// maxWidth is the maximum width of the progress string in chars, 0 for
	// none.
	maxWidth int
	throttle throttle
}

func newParProgress(maxRunningDisplayed int, mode progressMode, colorStderr bool, maxWidth int) *parProgress {
	format := text.FormatPlain
	if colorStderr {
		format = text.FormatANSI
	}
	return &parProgress{
		maxRunningDisplayed: maxRunningDisplayed,
		mode:                mode,
		format:              format,
		maxWidth:            maxWidth,
		throttle:            throttle{start: time.Now(), interval: updateInterval, firstThrottle: firstThrottle},
	}
}

func (p *parProgress) clearProgressBar(stderr *term.Stderr) {
	if p.mode != modeTestWithProgress {
		return
	}
	stderr.ClearProgressBar()
}

// updateProgressBar displays the progression of workers on stderr.
func (p *parProgress) updateProgressBar(workers []workerState, completed int, total ast.Count, stderr *term.Stderr) {
	if p.mode != modeTestWithProgress {
		return
	}
	p.throttle.update()
	progress, ok := buildProgress(workers, completed, total, p.maxRunningDisplayed, p.format, p.maxWidth)
	if !ok {
		return
	}
	stderr.SetProgressBar(progress)
}

// printCompleted displays the completion of a job.
func (p *parProgress) printCompleted(result *JobResult, stderr *term.Stderr) {
	if p.mode == modeDefault {
		return
	}
	count := 0
	for _, e := range result.HurlResult.Entries {
		count += len(e.Calls)
	}
	var message text.StyledString
	if result.HurlResult.Success {
		message.PushWith("Success", text.NewStyle().Green().Bold())
	} else {
		message.PushWith("Failure", text.NewStyle().Red().Bold())
	}
	message.Push(" ")
	message.PushWith(result.Job.Filename.String(), text.NewStyle().Bold())
	message.Push(fmt.Sprintf(" (%d request(s) in %d ms)", count, result.HurlResult.Duration.Milliseconds()))
	stderr.Eprintln(message.Render(p.format))
}

func (p *parProgress) canUpdate() bool { return p.throttle.allowed() }

func (p *parProgress) forceNextUpdate() { p.throttle.reset() }

// throttle records when the progress bar is refreshed: we don't want to
// update it too often, nor draw things that go away too quickly.
type throttle struct {
	start         time.Time
	lastUpdate    *time.Time
	interval      time.Duration
	firstThrottle time.Duration
}

func (t *throttle) allowed() bool {
	return t.lastUpdate == nil || time.Since(*t.lastUpdate) >= t.interval
}

func (t *throttle) update() {
	if time.Since(t.start) < t.firstThrottle {
		return
	}
	now := time.Now()
	t.lastUpdate = &now
}

func (t *throttle) reset() { t.lastUpdate = nil }

// buildProgress returns the progress string of the running workers, with
// completed jobs of total. At most maxRunningDisplayed running jobs are
// displayed. Each line is wrapped at maxWidth (if not 0).
func buildProgress(workers []workerState, completed int, total ast.Count, maxRunningDisplayed int, format text.Format, maxWidth int) (string, bool) {
	var running []workerState
	for _, w := range workers {
		if w.job != nil {
			running = append(running, w)
		}
	}
	if len(running) == 0 {
		return "", false
	}
	// Sort by job sequence, so a given job is displayed on the same line
	// independently of the worker.
	slices.SortFunc(running, func(a, b workerState) int { return a.job.Seq - b.job.Seq })
	runningCount := len(running)
	running = running[:min(len(running), maxRunningDisplayed)]

	maxLast := 0
	for _, w := range running {
		maxLast = max(maxLast, w.lastEntry.Get())
	}
	maxCompletedWidth := 2*(int(math.Log10(float64(maxLast)))+1) + 1

	var b strings.Builder
	if n, ok := total.(ast.CountFinite); ok {
		percent := 0
		if n.Value > 0 {
			percent = int(float64(completed) * 100 / float64(n.Value))
		}
		fmt.Fprintf(&b, "Executed files: %d/%d (%d%%)\n", completed, n.Value, percent)
	} else {
		fmt.Fprintf(&b, "Executed files: %d\n", completed)
	}

	for _, w := range running {
		requests := fmt.Sprintf("%s/%s", w.currentEntry, w.lastEntry)
		padding := strings.Repeat(" ", max(maxCompletedWidth-len(requests), 0))
		var progress text.StyledString
		progress.Push(progressBar(w.currentEntry, w.lastEntry))
		progress.Push(padding)
		progress.Push(" ")
		progress.PushWith("Running", text.NewStyle().Cyan().Bold())
		progress.Push(" ")
		progress.PushWith(w.job.Filename.String(), text.NewStyle().Bold())
		if w.retryCount > 0 {
			progress.Push(" ")
			progress.PushWith(fmt.Sprintf("(retry %d)", w.retryCount), text.NewStyle().Yellow())
		}
		progress.Push("\n")
		if maxWidth > 0 && progress.Len() >= maxWidth {
			progress = progress.Wrap(maxWidth)
		}
		b.WriteString(progress.Render(format))
	}
	if runningCount > maxRunningDisplayed {
		fmt.Fprintf(&b, "...%d more\n", runningCount-maxRunningDisplayed)
	}
	return b.String(), true
}

// progressBar returns the progress bar of a single job at entry current.
func progressBar(current, last ast.Index) string {
	const width = 24
	progress := float64(current.ToZeroBased()) / float64(last.Get())
	col := int(progress * width)
	return fmt.Sprintf("[%s>%s] %s/%s", strings.Repeat("=", col), strings.Repeat(" ", max(width-col-1, 0)), current, last)
}
