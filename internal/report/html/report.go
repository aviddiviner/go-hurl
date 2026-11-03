package html

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	htmlutil "github.com/aviddiviner/go-hurl/internal/html"
	"github.com/aviddiviner/go-hurl/internal/report"
)

const (
	rfc2822 = "Mon, 2 Jan 2006 15:04:05 -0700"
	rfc3339 = "2006-01-02T15:04:05-07:00"
)

// WriteReport creates an HTML report for this list of testcases at
// dir/index.html. If the report already exists, results are merged.
func WriteReport(dir string, testcases []Testcase) error {
	indexPath := joinPath(dir, "index.html")
	results, err := parseHTML(indexPath)
	if err != nil {
		return err
	}
	for _, t := range testcases {
		results = append(results, newHTMLResult(t))
	}
	s := createHTMLIndex(time.Now().Format(rfc2822), results)
	if err := os.WriteFile(indexPath, []byte(s), 0o666); err != nil {
		return report.NewIOError(err, indexPath, "Issue writing HTML report")
	}
	return nil
}

// createHTMLIndex returns a standalone HTML report from the list of results.
func createHTMLIndex(now string, results []htmlResult) string {
	countTotal := len(results)
	countSuccess := 0
	for _, r := range results {
		if r.success {
			countSuccess++
		}
	}
	countFailure := countTotal - countSuccess
	var rows strings.Builder
	for _, r := range results {
		rows.WriteString(createHTMLTableRow(r))
	}
	return formatTemplate(reportTpl, map[string]string{
		"now":                now,
		"css":                reportCSS,
		"count_total":        strconv.Itoa(countTotal),
		"count_success":      strconv.Itoa(countSuccess),
		"count_failure":      strconv.Itoa(countFailure),
		"percentage_success": percentage(countSuccess, countTotal),
		"percentage_failure": percentage(countFailure, countTotal),
		"rows":               rows.String(),
	})
}

func parseHTML(path string) ([]htmlResult, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err == nil && !utf8.Valid(b) {
		err = report.ErrInvalidData
	}
	if err != nil {
		return nil, report.NewIOError(err, path, "Issue reading HTML report")
	}
	return parseHTMLReport(string(b)), nil
}

// ws is Rust's Unicode \s.
const ws = `[\t\n\v\f\r\x{85}\p{Zs}\x{2028}\x{2029}]`

var testRef = regexp.MustCompile(
	`data-duration="(?P<time_in_ms>\d+)"` + ws + `+` +
		`data-status="(?P<status>[a-z]+)"` + ws + `+` +
		`data-filename="(?P<filename>[\p{L}\p{N}\p{M}\p{S}\p{P}\p{Zs}_./-]+)"` + ws + `+` +
		`data-id="(?P<id>[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})"` +
		`(` + ws + `+data-timestamp="(?P<timestamp>[0-9]{1,10})")?`)

// parseHTMLReport parses an HTML report and returns its results. If the
// existing report is not valid, it is considered as empty, without error.
func parseHTMLReport(s string) []htmlResult {
	var results []htmlResult
	for _, m := range testRef.FindAllStringSubmatch(s, -1) {
		group := func(name string) string { return m[testRef.SubexpIndex(name)] }
		timeInMs, err := strconv.ParseInt(group("time_in_ms"), 10, 64)
		if err != nil {
			continue
		}
		var timestamp int64
		if ts := group("timestamp"); ts != "" {
			timestamp, _ = strconv.ParseInt(ts, 10, 64)
		}
		results = append(results, htmlResult{
			// Filenames are HTML escaped when the report is written.
			filename:  htmlutil.Unescape(group("filename")),
			id:        group("id"),
			timeInMs:  timeInMs,
			success:   group("status") == "success",
			timestamp: timestamp,
		})
	}
	return results
}

func createHTMLTableRow(r htmlResult) string {
	status := "failure"
	if r.success {
		status = "success"
	}
	durationInS := strconv.FormatFloat(float64(r.timeInMs)/1000.0, 'f', -1, 64)
	filename := htmlutil.Escape(r.filename)
	displayedFilename := filename
	if r.filename == "-" {
		displayedFilename = "(standard input)"
	}
	displayedTime := "-"
	if r.timestamp != 0 {
		// Hurl displays the UTC date and time in the local timezone.
		u := time.Unix(r.timestamp, 0).UTC()
		local := time.Date(u.Year(), u.Month(), u.Day(), u.Hour(), u.Minute(), u.Second(), 0, time.Local)
		displayedTime = local.Format(rfc3339)
	}
	return fmt.Sprintf(`<tr data-duration="%d" data-status="%s" data-filename="%s" data-id="%s" data-timestamp="%d">
    <td><a href="store/%s-source.html">%s</a></td>
    <td class="%s"><a href="store/%s-timeline.html">%s</a></td>
    <td>%s</td>
    <td>%s</td>
</tr>
`, r.timeInMs, status, filename, r.id, r.timestamp, r.id, displayedFilename, status, r.id, status, displayedTime, durationInS)
}

func percentage(count, total int) string {
	p := float32(float32(count)*100.0) / float32(total)
	return fmt.Sprintf("%.1f%%", p)
}
