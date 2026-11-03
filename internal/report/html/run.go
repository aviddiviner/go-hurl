package html

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	htmlutil "github.com/aviddiviner/go-hurl/internal/html"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// runHTML returns an HTML view of a run (HTTP status code, response headers
// etc...).
func (t Testcase) runHTML(hurlFile *ast.HurlFile, content string, entries []runner.EntryResult, secrets []string) string {
	var run strings.Builder
	for entryIndex, e := range entries {
		entrySrc := hurlFile.Entries[e.EntryIndex.ToZeroBased()]
		line := entrySrc.SourceInfo().Start.Line

		run.WriteString("<details open>")
		run.WriteString(entryHTML(&e, entryIndex+1, secrets))
		for callIndex := range e.Calls {
			run.WriteString(callHTML(&e.Calls[callIndex], entryIndex+1, callIndex+1, t.safeFilename(), t.sourceFilename(), line, secrets))
		}
		run.WriteString("</details>")
	}
	return formatTemplate(runTpl, map[string]string{
		"filename": t.safeFilename(),
		"nav":      t.navHTML(content, tabRun, secrets),
		"nav_css":  navCSS,
		"run":      run.String(),
		"run_css":  runCSS,
	})
}

type nameValue struct{ name, value string }

// entryHTML returns an HTML view of an entry (title, index and captures).
func entryHTML(entry *runner.EntryResult, entryIndex int, secrets []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<summary>Entry %d</summary>", entryIndex)

	cmd := htmlutil.Escape(entry.CurlCmd.ToRedactedString(secrets))
	b.WriteString(newTable("Debug", []nameValue{{"Command", cmd}}))

	if len(entry.Captures) > 0 {
		var values []nameValue
		for _, c := range entry.Captures {
			values = append(values, nameValue{htmlutil.Escape(c.Name), htmlutil.Escape(util.Redact(c.Value.String(), secrets))})
		}
		sortByLowerName(values)
		b.WriteString(newTable("Captures", values))
	}
	return b.String()
}

// callHTML returns an HTML view of a call (source file, request and response
// headers, certificate etc...).
func callHTML(call *httpclient.Call, entryIndex, callIndex int, filename, source string, line int, secrets []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<h4 id=\"e%d:c%d\">Call %d</h3>", entryIndex, callIndex, callIndex)

	// General
	url := htmlutil.Escape(util.Redact(call.Request.Url.String(), secrets))
	b.WriteString(newTable("General", []nameValue{
		{"Start Date", call.Timings.BeginCall.UTC().Format(rfc2822)},
		{"Request URL", fmt.Sprintf(`<a href="%s">%s</a>`, url, url)},
		{"Request Method", call.Request.Method},
		{"Version", call.Response.Version.String()},
		{"Status code", strconv.FormatUint(uint64(call.Response.Status), 10)},
		{"Source", fmt.Sprintf(`<a href="%s#l%d">%s:%d</a>`, source, line, filename, line)},
	}))

	// Certificate
	if cert := call.Response.Certificate; cert != nil {
		var values []nameValue
		if v := cert.Subject(); v != nil {
			values = append(values, nameValue{"Subject", htmlutil.Escape(*v)})
		}
		if v := cert.Issuer(); v != nil {
			values = append(values, nameValue{"Issuer", htmlutil.Escape(*v)})
		}
		if v := cert.StartDate(); v != nil {
			values = append(values, nameValue{"Start Date", httpclient.FormatUtc(*v)})
		}
		if v := cert.ExpireDate(); v != nil {
			values = append(values, nameValue{"Expire Date", httpclient.FormatUtc(*v)})
		}
		if v := cert.SerialNumber(); v != nil {
			values = append(values, nameValue{"Serial Number", htmlutil.Escape(*v)})
		}
		if v := cert.SubjectAltName(); v != nil {
			values = append(values, nameValue{"Subject Alt Name", htmlutil.Escape(*v)})
		}
		if v := cert.Value(); v != nil {
			// Truncate PEM for display: first 50 chars + "..." + last 50 chars.
			pem := *v
			if len(pem) > 100 {
				pem = pem[:50] + "..." + pem[len(pem)-50:]
			}
			values = append(values, nameValue{"Value", pem})
		}
		b.WriteString(newTable("Certificate", values))
	}

	b.WriteString(newTable("Request Headers", headersValues(call.Request.Headers, secrets)))
	b.WriteString(newTable("Response Headers", headersValues(call.Response.Headers, secrets)))
	return b.String()
}

func headersValues(headers httpclient.HeaderVec, secrets []string) []nameValue {
	var values []nameValue
	for _, h := range headers.All() {
		values = append(values, nameValue{htmlutil.Escape(h.Name), htmlutil.Escape(util.Redact(h.Value, secrets))})
	}
	sortByLowerName(values)
	return values
}

func sortByLowerName(values []nameValue) {
	slices.SortStableFunc(values, func(a, b nameValue) int {
		return strings.Compare(strings.ToLower(a.name), strings.ToLower(b.name))
	})
}

// newTable returns an HTML table with a title and a list of name/values.
func newTable(title string, data []nameValue) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<table><thead><tr><th colspan="2">%s</tr></th></thead><tbody>`, title)
	for _, d := range data {
		fmt.Fprintf(&b, `<tr><td class="name">%s</td><td class="value">%s</td></tr>`, d.name, d.value)
	}
	b.WriteString("</tbody></table>")
	return b.String()
}
