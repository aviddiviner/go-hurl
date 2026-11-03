package html

import (
	"fmt"
	"slices"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/format"
	"github.com/aviddiviner/go-hurl/internal/runner"
)

// sourceHTML returns the HTML of the Hurl source file (syntax colored and
// errors).
func (t Testcase) sourceHTML(hurlFile *ast.HurlFile, content string, secrets []string) string {
	return formatTemplate(sourceTpl, map[string]string{
		"filename":   t.safeFilename(),
		"hurl_css":   format.HurlCSS(),
		"lines_div":  numberedLines(content, t.Errors),
		"nav":        t.navHTML(content, tabSource, secrets),
		"nav_css":    navCSS,
		"source_div": format.HTML(hurlFile, false),
		"source_css": sourceCSS,
	})
}

// numberedLines returns the line numbers of content in HTML.
func numberedLines(content string, errors []runner.EntryError) string {
	var errorLines []int
	for _, e := range errors {
		errorLines = append(errorLines, e.Error.SourceInfo().Start.Line)
	}
	var b strings.Builder
	b.WriteString("<pre><code>")
	for line := 1; line <= linesCount(content); line++ {
		if slices.Contains(errorLines, line) {
			fmt.Fprintf(&b, "<a id=\"l%d\" href=\"#l%d\" class=\"line-error\">%d</a>\n", line, line, line)
		} else {
			fmt.Fprintf(&b, "<a id=\"l%d\" href=\"#l%d\">%d</a>\n", line, line, line)
		}
	}
	b.WriteString("</pre></code>")
	return b.String()
}

// linesCount returns the number of lines of s, like Rust's str::lines.
func linesCount(s string) int {
	if s == "" {
		return 0
	}
	n := strings.Count(s, "\n")
	if !strings.HasSuffix(s, "\n") {
		n++
	}
	return n
}
