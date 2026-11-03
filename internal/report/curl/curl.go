// Package curl writes a curl export of Hurl runs, ported from
// hurl/src/report/curl.rs.
package curl

import (
	"os"
	"strings"

	"github.com/aviddiviner/go-hurl/internal/report"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// WriteCurl creates a curl export from a list of results. secrets are
// redacted from this export.
func WriteCurl(results []*runner.HurlResult, filename string, secrets []string) error {
	if err := util.CreateDirAll(filename); err != nil {
		return report.NewIOError(err, filename, "Issue creating curl export")
	}
	file, err := os.Create(filename)
	if err != nil {
		return report.NewIOError(err, filename, "Issue creating curl export")
	}
	defer file.Close()

	var cmds []string
	for _, r := range results {
		for _, e := range r.Entries {
			cmds = append(cmds, e.CurlCmd.ToRedactedString(secrets))
		}
	}
	if _, err := file.WriteString(strings.Join(cmds, "\n") + "\n"); err != nil {
		return report.NewIOError(err, filename, "Issue writing curl export")
	}
	return nil
}
