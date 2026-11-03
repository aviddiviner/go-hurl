package html

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	htmlutil "github.com/aviddiviner/go-hurl/internal/html"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/util"
)

type tab int

const (
	tabTimeline tab = iota
	tabRun
	tabSource
)

// navHTML returns the HTML navigation component for a tab. This common
// component is used to get source information and errors.
func (t Testcase) navHTML(content string, tab tab, secrets []string) string {
	errorsCount := "-"
	if len(t.Errors) > 0 {
		errorsCount = strconv.Itoa(len(t.Errors))
	}
	return formatTemplate(navTpl, map[string]string{
		"duration":          strconv.FormatInt(t.TimeInMs, 10),
		"errors":            t.errorsHTML(content, secrets),
		"errors_count":      errorsCount,
		"filename":          t.safeFilename(),
		"href_run":          t.runFilename(),
		"href_source":       t.sourceFilename(),
		"href_timeline":     t.timelineFilename(),
		"run_selected":      strconv.FormatBool(tab == tabRun),
		"source_selected":   strconv.FormatBool(tab == tabSource),
		"status":            statusHTML(t.Success, t.ID),
		"timeline_selected": strconv.FormatBool(tab == tabTimeline),
	})
}

// errorsHTML formats the errors of this testcase to an HTML snippet.
func (t Testcase) errorsHTML(content string, secrets []string) string {
	var b strings.Builder
	for _, e := range t.Errors {
		err := errorToHTML(e.Error, e.SourceInfo, content, t.Filename, t.sourceFilename(), secrets)
		b.WriteString(`<div class="error"><div class="error-desc">` + err + `</div></div>`)
	}
	return b.String()
}

func statusHTML(success bool, id string) string {
	class := "failure"
	if success {
		class = "success"
	}
	return fmt.Sprintf(`<span class="%s"><a href="%s-timeline.html">%s</a></span>`, class, id, class)
}

// errorToHTML returns an HTML <pre> tag representing err.
func errorToHTML(err *runner.RunnerError, entrySrcInfo ast.SourceInfo, content, filename, sourceFilename string, secrets []string) string {
	line := err.SourceInfo().Start.Line
	column := err.SourceInfo().Start.Column
	message := sourceerr.Render(err, filename, content, &entrySrcInfo, sourceerr.OutputFormatTerminal{Color: false})
	message = htmlutil.Escape(util.Redact(message, secrets))
	// We override the first part of the error string to add an anchor to the
	// error context.
	filename = htmlutil.Escape(filename)
	old := fmt.Sprintf("%s:%d:%d", filename, line, column)
	repl := fmt.Sprintf(`<a href="%s#l%d">%s:%d:%d</a>`, sourceFilename, line, filename, line, column)
	message = strings.ReplaceAll(message, old, repl)
	return "<pre><code>" + message + "</code></pre>"
}
