// Package tap writes Test Anything Protocol (https://testanything.org)
// reports, ported from hurl/src/report/tap.
package tap

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/report"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// See https://testanything.org/tap-version-13-specification.html
const tapReportVersionMarker = "TAP version 13"

// Testcase is a TAP test line.
type Testcase struct {
	description string
	success     bool
}

// NewTestcase creates a TAP testcase from a Hurl result.
func NewTestcase(result *runner.HurlResult, filename input.Input) Testcase {
	return Testcase{description: filename.String(), success: len(result.Errors()) == 0}
}

// parseTestcase parses a TAP line, like "ok 1 - this is the first test" or
// "not ok 2 - this is the second test".
func parseTestcase(line string) (Testcase, error) {
	var success bool
	switch {
	case strings.HasPrefix(line, "ok"):
		line = line[2:]
		success = true
	case strings.HasPrefix(line, "not ok"):
		line = line[6:]
	default:
		return Testcase{}, report.NewError(fmt.Sprintf("Invalid TAP line <%s> - must start with ok or nok", line))
	}
	index := strings.IndexByte(line, '-')
	if index < 0 {
		return Testcase{}, report.NewError(fmt.Sprintf("Invalid TAP line <%s> - missing '-' separator", line))
	}
	if !isUsize(strings.TrimSpace(line[:index])) {
		return Testcase{}, report.NewError(fmt.Sprintf("Invalid TAP line <%s> - missing test number", line))
	}
	return Testcase{description: strings.TrimSpace(line[index+1:]), success: success}, nil
}

// isUsize reports whether s parses as a Rust usize.
func isUsize(s string) bool {
	s = strings.TrimPrefix(s, "+")
	if s == "" || s[0] == '+' || s[0] == '-' {
		return false
	}
	_, err := strconv.ParseUint(s, 10, strconv.IntSize)
	return err == nil
}

// WriteReport creates or appends to a TAP report from a list of testcases.
func WriteReport(filename string, testcases []Testcase) error {
	existing, err := parseTapFile(filename)
	if err != nil {
		return err
	}
	return writeTapFile(filename, append(existing, testcases...))
}

func writeTapFile(filename string, testcases []Testcase) error {
	if err := util.CreateDirAll(filename); err != nil {
		return report.NewIOError(err, filename, "Issue writing TAP report")
	}
	var b strings.Builder
	b.WriteString(tapReportVersionMarker + "\n")
	fmt.Fprintf(&b, "1..%d\n", len(testcases))
	for i, tc := range testcases {
		state := "not ok"
		if tc.success {
			state = "ok"
		}
		fmt.Fprintf(&b, "%s %d - %s\n", state, i+1, tc.description)
	}
	if err := os.WriteFile(filename, []byte(b.String()), 0o666); err != nil {
		return report.NewIOError(err, filename, "Issue writing TAP report")
	}
	return nil
}

func parseTapFile(filename string) ([]Testcase, error) {
	if _, err := os.Stat(filename); err != nil {
		return nil, nil
	}
	b, err := os.ReadFile(filename)
	if err == nil && !utf8.Valid(b) {
		err = report.ErrInvalidData
	}
	if err != nil {
		return nil, report.NewIOError(err, filename, "Issue reading TAP report")
	}
	return parseTapReport(string(b))
}

var tapHeader = regexp.MustCompile(`^1\.\.\d+.*$`)

func parseTapReport(s string) ([]Testcase, error) {
	lines := lines(s)
	if len(lines) == 0 {
		return nil, nil
	}
	header, lines := lines[0], lines[1:]
	// A TAP report may have a protocol version header.
	if strings.EqualFold(header, tapReportVersionMarker) {
		header = ""
		if len(lines) > 0 {
			header, lines = lines[0], lines[1:]
		}
	}
	if !tapHeader.MatchString(header) {
		return nil, report.NewError(fmt.Sprintf("Invalid TAP Header <%s>", header))
	}
	var testcases []Testcase
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tc, err := parseTestcase(line)
		if err != nil {
			return nil, err
		}
		testcases = append(testcases, tc)
	}
	return testcases, nil
}

// lines splits s like Rust's str::lines.
func lines(s string) []string {
	if s == "" {
		return nil
	}
	ls := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimSuffix(l, "\r")
	}
	return ls
}
