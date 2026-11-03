// Package json writes JSON reports, ported from hurl/src/report/json.
//
// The JSON report is organised as follows:
//
//   - report.json: list of each file of a run exported to JSON
//   - store/foo_response.{json,xml,html}: an HTTP response referenced in
//     report.json
package json

import (
	"os"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/report"
	"github.com/aviddiviner/go-hurl/internal/runner"
)

// Testcase is a run exported to the JSON report.
type Testcase struct {
	result   *runner.HurlResult
	content  string
	filename input.Input
}

// NewTestcase creates a JSON testcase.
func NewTestcase(result *runner.HurlResult, content string, filename input.Input) Testcase {
	return Testcase{result: result, content: content, filename: filename}
}

// toJSON serializes this testcase to JSON. secrets are redacted from the JSON
// fields.
func (t Testcase) toJSON(responseDir string, secrets []string) (any, error) {
	return t.result.ToJSON(t.content, t.filename, responseDir, secrets)
}

// WriteReport exports a list of testcases to the JSON file indexPath,
// appending them to any existing report. Response files are saved under
// storeDir and referenced by path in the report. secrets are redacted from
// the report fields.
func WriteReport(indexPath string, testcases []Testcase, storeDir string, secrets []string) error {
	runs, err := parseJSONReport(indexPath)
	if err != nil {
		return err
	}
	for _, t := range testcases {
		v, err := t.toJSON(storeDir, secrets)
		if err != nil {
			return report.NewIOError(err, indexPath, "Issue creating JSON report")
		}
		runs = append(runs, v)
	}

	data := append(runner.EncodeJSON(runs), '\n')
	file, err := os.Create(indexPath)
	if err != nil {
		return report.NewIOError(err, indexPath, "Issue creating JSON report")
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return report.NewIOError(err, indexPath, "Issue writing JSON report")
	}
	return nil
}

// parseJSONReport returns the runs of an existing JSON report. If the
// existing report is not valid, it is considered as empty, without error.
func parseJSONReport(filename string) ([]any, error) {
	if _, err := os.Stat(filename); err != nil {
		return []any{}, nil
	}
	b, err := os.ReadFile(filename)
	if err == nil && !utf8.Valid(b) {
		err = report.ErrInvalidData
	}
	if err != nil {
		return nil, report.NewIOError(err, filename, "Issue reading JSON report")
	}
	v, err := runner.DecodeJSON(b)
	if err != nil {
		return []any{}, nil
	}
	values, ok := v.([]any)
	if !ok {
		return []any{}, nil
	}
	for _, v := range values {
		if !runner.IsDeserializable(v) {
			return []any{}, nil
		}
	}
	return values, nil
}
