// Package report holds the shared report error type. The report formats live in
// subpackages (curl, html, json, junit, tap), each ported from hurl/src/report.
//
// API contract (consumed by cmd/hurl, mirroring hurl/src/main.rs):
//
//	curl.WriteCurl(results []*runner.HurlResult, filename string, secrets []string) error
//	junit.NewTestcase(result *runner.HurlResult, content string, filename input.Input) junit.Testcase
//	junit.WriteReport(filename string, testcases []junit.Testcase, secrets []string) error
//	tap.NewTestcase(result *runner.HurlResult, filename input.Input) tap.Testcase
//	tap.WriteReport(filename string, testcases []tap.Testcase) error
//	html.NewTestcase(result *runner.HurlResult, filename input.Input) html.Testcase
//	(html.Testcase).WriteHTML(content string, entries []runner.EntryResult, storeDir string, secrets []string) error
//	html.WriteReport(dir string, testcases []html.Testcase) error
//	json.NewTestcase(result *runner.HurlResult, content string, filename input.Input) json.Testcase
//	json.WriteReport(indexPath string, testcases []json.Testcase, storeDir string, secrets []string) error
//
// Errors returned by these functions are *report.Error (Rust's ReportError).
package report
