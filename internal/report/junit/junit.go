// Package junit writes XML JUnit reports, ported from hurl/src/report/junit.
//
// One Hurl file results in one JUnit <testcase>, which can include <error>
// (for runtime errors) or <failure> (for assert errors). Each Hurl execution
// generates its own <testsuite> within the root <testsuites>, appended to any
// existing report.
package junit

import (
	"fmt"
	"os"
	"strconv"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/report"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// Testcase is a JUnit <testcase>.
type Testcase struct {
	id       string
	name     string
	timeInMs int64
	failures []string
	errors   []string
}

// NewTestcase creates a JUnit testcase from a Hurl result.
func NewTestcase(result *runner.HurlResult, content string, filename input.Input) Testcase {
	tc := Testcase{
		id:       filename.String(),
		name:     filename.String(),
		timeInMs: result.Duration.Milliseconds(),
	}
	for _, e := range result.Errors() {
		message := sourceerr.Render(e.Error, tc.name, content, &e.SourceInfo, sourceerr.OutputFormatTerminal{Color: false})
		if e.Error.Assert {
			tc.failures = append(tc.failures, message)
		} else {
			tc.errors = append(tc.errors, message)
		}
	}
	return tc
}

// toXML serializes this testcase to XML. secrets are redacted from the
// messages.
func (t Testcase) toXML(secrets []string) *element {
	elt := newElement("testcase").
		attr("id", t.id).
		attr("name", t.name).
		attr("time", fmt.Sprintf("%.3f", float64(t.timeInMs)/1000.0))
	for _, f := range t.failures {
		elt.addChild(newElement("failure").text(util.Redact(f, secrets)))
	}
	for _, e := range t.errors {
		elt.addChild(newElement("error").text(util.Redact(e, secrets)))
	}
	return elt
}

// WriteReport creates a JUnit report from a list of testcases, or appends a
// new testsuite to an existing report. secrets are redacted from the
// messages.
func WriteReport(filename string, testcases []Testcase, secrets []string) error {
	if err := util.CreateDirAll(filename); err != nil {
		return report.NewIOError(err, filename, "Issue writing JUnit report")
	}

	var root *element
	if _, err := os.Stat(filename); err == nil {
		data, err := os.ReadFile(filename)
		if err != nil {
			return report.NewIOError(err, filename, "Issue reading JUnit report")
		}
		doc, err := parseXML(data)
		if err == nil && doc.root == nil {
			err = errInvalidXML("no root element")
		}
		if err != nil {
			return report.NewError(fmt.Sprintf("Issue reading JUnit report %s %s", filename, err))
		}
		root = doc.root
	} else {
		root = newElement("testsuites")
	}
	root.addChild(createTestsuite(testcases, secrets))

	doc := xmlDocument{root: root}
	file, err := os.Create(filename)
	if err != nil {
		return report.NewIOError(err, filename, "Issue writing JUnit report")
	}
	defer file.Close()
	if _, err := file.Write(doc.bytes()); err != nil {
		return report.NewError(fmt.Sprintf("Issue writing JUnit report %s %s", filename, report.OSErrorString(err)))
	}
	return nil
}

// createTestsuite returns a <testsuite> element from a list of testcases.
func createTestsuite(testcases []Testcase, secrets []string) *element {
	errors, failures := 0, 0
	for _, tc := range testcases {
		errors += len(tc.errors)
		failures += len(tc.failures)
	}
	elt := newElement("testsuite").
		attr("tests", strconv.Itoa(len(testcases))).
		attr("errors", strconv.Itoa(errors)).
		attr("failures", strconv.Itoa(failures))
	for _, tc := range testcases {
		elt.addChild(tc.toXML(secrets))
	}
	return elt
}
