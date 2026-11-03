// Package html writes HTML reports, ported from hurl/src/report/html.
//
// The report is an index.html file listing the runs, and for each run three
// files in a store directory: a syntax-colored view of the Hurl source file, a
// timeline view (with an SVG waterfall) and a view of the executed run.
package html

import (
	"crypto/rand"
	_ "embed"
	"fmt"
	"os"
	"strings"

	htmlutil "github.com/aviddiviner/go-hurl/internal/html"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/report"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/parser"
)

var (
	//go:embed resources/calls.css
	callsCSS string
	//go:embed resources/nav.css
	navCSS string
	//go:embed resources/nav.html
	navTpl string
	//go:embed resources/report.css
	reportCSS string
	//go:embed resources/report.html
	reportTpl string
	//go:embed resources/run.css
	runCSS string
	//go:embed resources/run.html
	runTpl string
	//go:embed resources/source.css
	sourceCSS string
	//go:embed resources/source.html
	sourceTpl string
	//go:embed resources/timeline.css
	timelineCSS string
	//go:embed resources/timeline.html
	timelineTpl string
	//go:embed resources/waterfall.css
	waterfallCSS string
)

// Testcase is a run displayed in the HTML report.
type Testcase struct {
	// ID is the unique identifier of this testcase.
	ID string
	// Filename is the source file name.
	Filename string
	Success  bool
	TimeInMs int64
	// Errors are the runtime errors, with the source information of the
	// entry throwing each error.
	Errors    []runner.EntryError
	Timestamp int64
}

// NewTestcase creates an HTML testcase.
func NewTestcase(result *runner.HurlResult, filename input.Input) Testcase {
	return Testcase{
		ID:        newUUIDv4(),
		Filename:  filename.String(),
		Success:   result.Success,
		TimeInMs:  result.Duration.Milliseconds(),
		Errors:    result.Errors(),
		Timestamp: result.Timestamp,
	}
}

// WriteHTML exports this testcase to HTML in the directory storeDir. It
// creates three HTML files:
//   - an HTML view of the Hurl source file (with potential errors and syntax
//     colored),
//   - an HTML timeline view of the executed entries (with potential errors,
//     waterfall),
//   - an HTML view of the executed run (headers, cookies, etc...).
//
// secrets are redacted from the produced HTML.
func (t Testcase) WriteHTML(content string, entries []runner.EntryResult, storeDir string, secrets []string) error {
	hurlFile, perr := parser.ParseHurlFile(content)
	if perr != nil {
		return report.NewError(fmt.Sprintf("Issue writing HTML report: %s is not a valid Hurl file", t.Filename))
	}
	files := []struct{ name, html string }{
		{t.timelineFilename(), t.timelineHTML(hurlFile, content, entries, secrets)},
		{t.runFilename(), t.runHTML(hurlFile, content, entries, secrets)},
		{t.sourceFilename(), t.sourceHTML(hurlFile, content, secrets)},
	}
	for _, f := range files {
		path := joinPath(storeDir, f.name)
		if err := os.WriteFile(path, []byte(f.html), 0o666); err != nil {
			return report.NewIOError(err, path, "Issue writing HTML report")
		}
	}
	return nil
}

// safeFilename returns this testcase's Hurl file name, HTML escaped.
func (t Testcase) safeFilename() string { return htmlutil.Escape(t.Filename) }

func (t Testcase) sourceFilename() string { return t.ID + "-source.html" }

func (t Testcase) timelineFilename() string { return t.ID + "-timeline.html" }

func (t Testcase) runFilename() string { return t.ID + "-run.html" }

// htmlResult is a test result displayed in the index page.
type htmlResult struct {
	// filename is the original filename, as given in the run execution.
	filename string
	// id is the id of the corresponding Testcase.
	id        string
	timeInMs  int64
	success   bool
	timestamp int64
}

func newHTMLResult(t Testcase) htmlResult {
	return htmlResult{filename: t.Filename, id: t.ID, timeInMs: t.TimeInMs, success: t.Success, timestamp: t.Timestamp}
}

// formatTemplate substitutes the named arguments of a Rust format! template: {name}
// is replaced by args[name], {{ and }} by { and }.
func formatTemplate(tpl string, args map[string]string) string {
	var b strings.Builder
	for i := 0; i < len(tpl); {
		switch {
		case strings.HasPrefix(tpl[i:], "{{"):
			b.WriteByte('{')
			i += 2
		case strings.HasPrefix(tpl[i:], "}}"):
			b.WriteByte('}')
			i += 2
		case tpl[i] == '{':
			end := strings.IndexByte(tpl[i:], '}')
			name := tpl[i+1 : i+end]
			v, ok := args[name]
			if !ok {
				panic("missing template argument " + name)
			}
			b.WriteString(v)
			i += end + 1
		default:
			b.WriteByte(tpl[i])
			i++
		}
	}
	return b.String()
}

// joinPath joins like Rust's Path::join, without lexical cleaning.
func joinPath(dir, name string) string {
	if dir == "" || strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}

func newUUIDv4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
