package html

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/parser"
	"github.com/aviddiviner/go-hurl/reader"
)

func srcInfo(line, col, endLine, endCol int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: line, Column: col}, End: reader.Pos{Line: endLine, Column: endCol}}
}

func TestPercentage(t *testing.T) {
	tests := []struct {
		count, total int
		want         string
	}{
		{100, 100, "100.0%"},
		{66, 99, "66.7%"},
		{33, 99, "33.3%"},
		{0, 0, "NaN%"},
	}
	for _, tt := range tests {
		if got := percentage(tt.count, tt.total); got != tt.want {
			t.Errorf("percentage(%d, %d) = %s, want %s", tt.count, tt.total, got, tt.want)
		}
	}
}

func TestParseHTMLReport(t *testing.T) {
	html := `<html>
          <body>
            <h2>Hurl Report</h2>
            <table>
              <tbody>
                <tr class="success" data-duration="100" data-status="success" data-filename="tests/hello.hurl" data-id="08aad14a-8d10-4ecc-892e-a72703c5b494">
                  <td><a href="tests/hello.hurl.html">tests/hello.hurl</a></td>
                  <td>success</td>
                  <td>0.1s</td>
                </tr>
                <tr class="failure" data-duration="200" data-status="failure" data-filename="tests/failure.hurl" data-id="a6641ae3-8ce0-4d9f-80c5-3e23e032e055" data-timestamp="1696473444">
                  <td><a href="tests/failure.hurl.html">tests/failure.hurl</a></td>
                  <td>failure</td>
                  <td>2023-10-05T02:37:24Z</td>
                  <td>0.2s</td>
                </tr>
                <tr class="success" data-duration="50" data-status="success" data-filename="tests/café.hurl" data-id="a151aea6-2b02-465e-be47-45a2fa9cce02" data-timestamp="1796473444">
                  <td><a href="tests/café.hurl.html">tests/café.hurl</a></td>
                  <td>success</td>
                  <td>2023-10-05T02:37:24Z</td>
                  <td>0.4s</td>
                </tr>
                <tr class="failure" data-duration="366" data-status="failure" data-filename="abcd[1234]@2x.hurl" data-id="2008c777-025d-4708-8016-e2928b9ef538" data-timestamp="1796473666">
                  <td><a href="abcd[1234]@2x.hurl.html">abcd[1234]@2x.hurl</a></td>
                  <td>failure</td>
                  <td>2023-10-05T02:37:24Z</td>
                  <td>0.4s</td>
                </tr>
                </tbody>
              <table>
           </body>
        </html>`
	want := []htmlResult{
		{filename: "tests/hello.hurl", id: "08aad14a-8d10-4ecc-892e-a72703c5b494", timeInMs: 100, success: true, timestamp: 0},
		{filename: "tests/failure.hurl", id: "a6641ae3-8ce0-4d9f-80c5-3e23e032e055", timeInMs: 200, success: false, timestamp: 1696473444},
		{filename: "tests/café.hurl", id: "a151aea6-2b02-465e-be47-45a2fa9cce02", timeInMs: 50, success: true, timestamp: 1796473444},
		{filename: "abcd[1234]@2x.hurl", id: "2008c777-025d-4708-8016-e2928b9ef538", timeInMs: 366, success: false, timestamp: 1796473666},
	}
	if got := parseHTMLReport(html); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

// Filenames with chars that must be escaped.
func TestFilenameSurvivesReportMerge(t *testing.T) {
	filenames := []string{
		`tests/a&b.hurl`,
		`tests/c<d>.hurl`,
		`tests/x" onmouseover="alert(1).hurl`,
		`tests/caf&eacute;.hurl`,
		"tests/café.hurl",
		"-",
	}
	for _, filename := range filenames {
		result := htmlResult{filename: filename, id: "08aad14a-8d10-4ecc-892e-a72703c5b494", timeInMs: 100, success: true, timestamp: 1696473444}
		// Merging a report re-writes rows parsed from the previous run, so
		// several cycles must be idempotent.
		for range 3 {
			row := createHTMLTableRow(result)
			if strings.Contains(row, ` onmouseover="`) {
				t.Fatalf("filename escaped its attribute: %s", row)
			}
			parsed := parseHTMLReport(row)
			if len(parsed) != 1 {
				t.Fatalf("row not parsable for %s", filename)
			}
			if parsed[0].filename != filename {
				t.Fatalf("got %s, want %s", parsed[0].filename, filename)
			}
			result = parsed[0]
		}
	}
}

func TestCreateHTMLTableRow(t *testing.T) {
	local := time.Local
	time.Local = time.FixedZone("CEST", 2*3600)
	defer func() { time.Local = local }()

	result := htmlResult{filename: "-", id: "08aad14a-8d10-4ecc-892e-a72703c5b494", timeInMs: 1000, success: false, timestamp: 1696473444}
	want := `<tr data-duration="1000" data-status="failure" data-filename="-" data-id="08aad14a-8d10-4ecc-892e-a72703c5b494" data-timestamp="1696473444">
    <td><a href="store/08aad14a-8d10-4ecc-892e-a72703c5b494-source.html">(standard input)</a></td>
    <td class="failure"><a href="store/08aad14a-8d10-4ecc-892e-a72703c5b494-timeline.html">failure</a></td>
    <td>2023-10-05T02:37:24+02:00</td>
    <td>1</td>
</tr>
`
	if got := createHTMLTableRow(result); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	result = htmlResult{filename: "a.hurl", id: "08aad14a-8d10-4ecc-892e-a72703c5b494", timeInMs: 124, success: true}
	row := createHTMLTableRow(result)
	if !strings.Contains(row, "<td>-</td>\n    <td>0.124</td>") {
		t.Errorf("got:\n%s", row)
	}
}

func TestErrorHTML(t *testing.T) {
	entrySrcInfo := srcInfo(1, 1, 1, 39)
	err := runner.NewRunnerError(srcInfo(4, 1, 4, 9), runner.RunnerErrorAssertFailure{
		Actual:   "<script>alert('Hi')</script>",
		Expected: "Hello world",
	}, true)
	content := "GET http://localhost:8000/inline-script\nHTTP 200\n[Asserts]\n`Hello World`\n"
	got := errorToHTML(err, entrySrcInfo, content, "a/b/c/foo.hurl", "abc-source.hurl", nil)
	want := `<pre><code>Assert failure
  --&gt; <a href="abc-source.hurl#l4">a/b/c/foo.hurl:4:1</a>
   |
   | GET http://localhost:8000/inline-script
   | ...
 4 | ` + "`Hello World`" + `
   |   actual:   &lt;script&gt;alert(&#x27;Hi&#x27;)&lt;/script&gt;
   |   expected: Hello world
   |</code></pre>`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTruncates(t *testing.T) {
	tests := []struct {
		in   string
		max  int
		want string
	}{
		{"foo", 32, "foo"},
		{"abcdefgh", 3, "abc..."},
		{"aébc", 2, "a..."},
	}
	for _, tt := range tests {
		if got := truncStr(tt.in, tt.max); got != tt.want {
			t.Errorf("truncStr(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
		}
	}
}

func TestCreateStripes(t *testing.T) {
	elt := newStripes(10, 1, interval[float64]{0, 10}, interval[float64]{0, 10}, "green")
	want := `<g class="grid-strip">` +
		`<rect x="0" y="0" width="10" height="1" fill="green" />` +
		`<rect x="0" y="2" width="10" height="1" fill="green" />` +
		`<rect x="0" y="4" width="10" height="1" fill="green" />` +
		`<rect x="0" y="6" width="10" height="1" fill="green" />` +
		`<rect x="0" y="8" width="10" height="1" fill="green" />` +
		`<rect x="0" y="10" width="10" height="1" fill="green" />` +
		`</g>`
	if got := elt.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestNiceScale(t *testing.T) {
	tests := []struct {
		min, max float64
		ticks    int
		want     niceScale
	}{
		{0, 500, 20, niceScale{minValue: 0, maxValue: 500, maxTicks: 20, rangeValue: 500, tickSpacing: 20, niceMin: 0, niceMax: 500}},
		{0, 1700, 20, niceScale{minValue: 0, maxValue: 1700, maxTicks: 20, rangeValue: 2000, tickSpacing: 100, niceMin: 0, niceMax: 1700}},
		{0, 1000, 10, niceScale{minValue: 0, maxValue: 1000, maxTicks: 10, rangeValue: 1000, tickSpacing: 100, niceMin: 0, niceMax: 1000}},
	}
	for _, tt := range tests {
		if got := newNiceScale(tt.min, tt.max, tt.ticks); got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
		}
	}
}

func TestFloorLog10(t *testing.T) {
	for e := int32(-22); e <= 22; e++ {
		if got := floorLog10(powi(10, e)); got != e {
			t.Errorf("floorLog10(1e%d) = %d", e, got)
		}
	}
	if got := floorLog10(0); got != -2147483648 {
		t.Errorf("floorLog10(0) = %d", got)
	}
}

func TestSimpleLineSVG(t *testing.T) {
	elt := newSVGLine(0, 80, 100, 20)
	elt.addAttr("stroke", "black")
	if got, want := elt.String(), `<line x1="0" y1="80" x2="100" y2="20" stroke="black" />`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestGroupSVG(t *testing.T) {
	root := newSVG()
	root.addAttr("viewBox", viewBox(0, 0, 100, 100))
	group := newSVGGroup()
	group.addAttr("fill", "white")
	group.addAttr("stroke", "green")
	group.addAttrF("stroke-width", 5)
	group.addChild(newSVGRect(0, 0, 40, 60, "#fff"))
	group.addChild(newSVGRect(20, 10, 3.5, 15, "red"))
	root.addChild(group)
	want := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">` +
		`<g fill="white" stroke="green" stroke-width="5">` +
		`<rect x="0" y="0" width="40" height="60" fill="#fff" />` +
		`<rect x="20" y="10" width="3.5" height="15" fill="red" />` +
		`</g>` +
		`</svg>`
	if got := root.String(); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestLegendSVG(t *testing.T) {
	elt := newLegend(20, 30, "Hello world", "red", 2000)
	want := `<g>` +
		`<rect x="20" y="30" width="20" height="20" fill="red" />` +
		`<text x="56" y="47">Hello world</text>` +
		`<text x="200" y="47">2.0 ms</text>` +
		`</g>`
	if got := elt.String(); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestGridVertLinesSVG(t *testing.T) {
	start := time.Date(2022, 1, 1, 8, 0, 0, 0, time.UTC)
	times := interval[time.Time]{start, start.Add(time.Second)}
	scaleX := newScale(times, interval[float64]{0, 1000})
	elt := newVertLines(times, 10, scaleX, interval[float64]{0, 100})
	want := `<g>` +
		`<g class="grid-ticks" stroke="#ccc">` +
		`<line x1="100" y1="0" x2="100" y2="100" />` +
		`<line x1="200" y1="0" x2="200" y2="100" />` +
		`<line x1="300" y1="0" x2="300" y2="100" />` +
		`<line x1="400" y1="0" x2="400" y2="100" />` +
		`<line x1="500" y1="0" x2="500" y2="100" />` +
		`<line x1="600" y1="0" x2="600" y2="100" />` +
		`<line x1="700" y1="0" x2="700" y2="100" />` +
		`<line x1="800" y1="0" x2="800" y2="100" />` +
		`<line x1="900" y1="0" x2="900" y2="100" />` +
		`</g>` +
		`<g font-size="15px" font-family="sans-serif" fill="#777">` +
		`<text x="5" y="20">0 ms</text>` +
		`<text x="105" y="20">100 ms</text>` +
		`<text x="205" y="20">200 ms</text>` +
		`<text x="305" y="20">300 ms</text>` +
		`<text x="405" y="20">400 ms</text>` +
		`<text x="505" y="20">500 ms</text>` +
		`<text x="605" y="20">600 ms</text>` +
		`<text x="705" y="20">700 ms</text>` +
		`<text x="805" y="20">800 ms</text>` +
		`<text x="905" y="20">900 ms</text>` +
		`</g>` +
		`</g>`
	if got := elt.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatTemplate(t *testing.T) {
	got := formatTemplate("{{a}} {a}{b} }}", map[string]string{"a": "{b}", "b": "x"})
	if want := "{a} {b}x }"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteReportMerge(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "build", "a", "b", "c")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	run1 := []Testcase{
		NewTestcase(&runner.HurlResult{Success: true, Duration: 100 * time.Millisecond, Timestamp: 1696473444}, input.New("tests/a&b.hurl")),
		NewTestcase(&runner.HurlResult{Success: false, Duration: 1500 * time.Millisecond}, input.New("tests/b.hurl")),
	}
	run2 := []Testcase{NewTestcase(&runner.HurlResult{Success: true, Duration: 5 * time.Millisecond}, input.New("tests/c.hurl"))}
	if err := WriteReport(dir, run1); err != nil {
		t.Fatal(err)
	}
	if err := WriteReport(dir, run2); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	got := parseHTMLReport(string(b))
	var want []htmlResult
	for _, tc := range append(run1, run2...) {
		want = append(want, newHTMLResult(tc))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	for _, s := range []string{
		`<div class="count">Executed: 3 (100%)</div>`,
		`<div class="count">Succeeded: 2 (66.7%)</div>`,
		`<div class="count">Failed: 1 (33.3%)</div>`,
		`<a href="store/` + run1[0].ID + `-source.html">tests/a&amp;b.hurl</a>`,
		"<td>1.5</td>",
	} {
		if !strings.Contains(string(b), s) {
			t.Errorf("index.html doesn't contain %s", s)
		}
	}
	date := regexp.MustCompile(`<div class="date">[A-Z][a-z]{2}, \d{1,2} [A-Z][a-z]{2} \d{4} \d{2}:\d{2}:\d{2} [+-]\d{4}</div>`)
	if !date.Match(b) {
		t.Errorf("index.html has no RFC 2822 date:\n%s", b)
	}
}

// evilEntry follows integration/hurl/tests_ok/html_report_injection.
func evilEntry(t *testing.T) runner.EntryResult {
	t.Helper()
	url, uerr := httpclient.ParseUrl("http://localhost:8000/evil-headers")
	if uerr != nil {
		t.Fatal(uerr)
	}
	spec := httpclient.NewRequestSpec()
	spec.Method = "GET"
	spec.Url = url
	spec.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("X-Bar", "<script>alert('Hello')</script>"))
	begin := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	return runner.EntryResult{
		EntryIndex: ast.NewIndex(1),
		SourceInfo: srcInfo(1, 1, 1, 36),
		Calls: []httpclient.Call{{
			Request: httpclient.NewRequest("GET", url, httpclient.NewHeaderVec(
				httpclient.NewHeader("Host", "localhost:8000"),
				httpclient.NewHeader("X-Bar", "<script>alert('Hello')</script>"),
			), nil),
			Response: httpclient.Response{
				Version: httpclient.HttpVersionHttp11,
				Status:  200,
				Headers: httpclient.NewHeaderVec(
					httpclient.NewHeader("X-Foo", "<script>alert('Hello')</script>"),
					httpclient.NewHeader("<script>alert('Hello')</script>", "foo"),
				),
				Body: []byte("Hello world!"),
			},
			Timings: httpclient.Timings{
				BeginCall:     begin,
				EndCall:       begin.Add(3 * time.Millisecond),
				NameLookup:    100 * time.Microsecond,
				Connect:       300 * time.Microsecond,
				PreTransfer:   400 * time.Microsecond,
				StartTransfer: 2 * time.Millisecond,
				Total:         2500 * time.Microsecond,
			},
		}},
		Captures: []runner.CaptureResult{{Name: `<script>alert("Hello")</script>`, Value: runner.NewInteger(200)}},
		CurlCmd:  httpclient.NewCurlCmd(spec, httpclient.NewCookieStore(), util.ContextDir{}, nil, httpclient.DefaultClientOptions()),
	}
}

func TestWriteHTMLInjection(t *testing.T) {
	content, err := os.ReadFile("../../../integration/hurl/tests_ok/html_report_injection/html_report_injection.hurl")
	if err != nil {
		t.Fatal(err)
	}
	result := &runner.HurlResult{Entries: []runner.EntryResult{evilEntry(t)}, Success: true, Duration: 4 * time.Millisecond}
	tc := NewTestcase(result, input.New("tests_ok/html_report_injection/html_report_injection.hurl"))
	store := filepath.Join(t.TempDir(), "store")
	if err := os.MkdirAll(store, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := tc.WriteHTML(string(content), result.Entries, store, nil); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(store)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("got %d files, want 3", len(files))
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(store, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "<script>") {
			t.Errorf("found unescaped <script> in %s", f.Name())
		}
	}
}

func TestWriteHTMLFailedInjection(t *testing.T) {
	content, err := os.ReadFile("../../../integration/hurl/tests_failed/html_report_injection/html_report_injection.hurl")
	if err != nil {
		t.Fatal(err)
	}
	entry := evilEntry(t)
	entry.SourceInfo = srcInfo(1, 1, 1, 40)
	entry.Errors = []*runner.RunnerError{runner.NewRunnerError(srcInfo(4, 1, 4, 14), runner.RunnerErrorAssertBodyValueError{
		Actual:   "<script>alert('Hi')</script>",
		Expected: "Hello World",
	}, true)}
	result := &runner.HurlResult{Entries: []runner.EntryResult{entry}, Success: false}
	tc := NewTestcase(result, input.New("tests_failed/html_report_injection/html_report_injection.hurl"))
	store := t.TempDir()
	if err := tc.WriteHTML(string(content), result.Entries, store, nil); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{tc.sourceFilename(), tc.timelineFilename(), tc.runFilename()} {
		b, err := os.ReadFile(filepath.Join(store, name))
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		if strings.Contains(s, "<script>") {
			t.Errorf("found unescaped <script> in %s", name)
		}
		if !strings.Contains(s, `<div class="item-name">Errors:</div><div>1</div>`) {
			t.Errorf("%s has no error count", name)
		}
		if !strings.Contains(s, `actual value is &lt;&lt;script&gt;alert(&#x27;Hi&#x27;)&lt;/script&gt;&gt;`) {
			t.Errorf("%s has no escaped error", name)
		}
	}
}

func mustParse(t *testing.T, content string) *ast.HurlFile {
	t.Helper()
	f, err := parser.ParseHurlFile(content)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestTimelineHTML(t *testing.T) {
	content, err := os.ReadFile("../../../integration/hurl/tests_ok/html_report_injection/html_report_injection.hurl")
	if err != nil {
		t.Fatal(err)
	}
	result := &runner.HurlResult{Entries: []runner.EntryResult{evilEntry(t)}, Success: true}
	tc := NewTestcase(result, input.New("a.hurl"))
	tc.ID = "id"
	calls := []*httpclient.Call{&result.Entries[0].Calls[0]}
	ctxs := tc.callContexts(mustParse(t, string(content)), result.Entries)
	want := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 260 324" width="260" height="324"><style>` + callsCSS + `</style>` +
		newSuccessIcon("success").String() + newFailureIcon("failure").String() + newRetryIcon("retry").String() +
		`<rect class="calls-back" x="0" y="0" width="100%" height="100%" fill="#fbfafd" />` +
		`<g class="calls-grid"><rect x="0" y="50" width="260" height="1" fill="#ddd" /><rect x="0" y="73" width="260" height="1" fill="#ddd" /><rect x="259" y="0" width="1" height="324" fill="#ddd" /></g>` +
		`<g class="calls-list" font-size="13px" font-family="sans-serif" fill="#777">` +
		`<use href="#success" x="7" y="56" width="13" height="13" />` +
		`<text x="25" y="67"><title>GET localhost:8000/evil-headers</title>GET localhost:8000/evil-...</text>` +
		`<text x="205" y="67">200</text>` +
		`<a href="id-run.html#e1:c1"><text x="233" y="67" fill="royalblue" text-decoration="underline">run</text></a>` +
		`</g></svg>`
	if got := callsSVG(calls, ctxs, nil); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	waterfall := waterfallSVG(calls, ctxs, nil)
	for _, s := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1138 324" width="1138" height="324"><style>`,
		`<defs><filter id="shadow"><feDropShadow dx="0" dy="4" stdDeviation="4" flood-opacity="0.25" /></filter></defs>`,
		`<g class="call-summary"><rect x="0" y="53" width="37.93333333333333" height="18" fill="#1d9688" /><rect x="37.93333333333333" y="53" width="75.86666666666666" height="18" fill="#fa7f03" /><rect x="151.73333333333332" y="53" width="606.9333333333333" height="18" fill="#18c852" /><rect x="758.6666666666666" y="53" width="189.66666666666666" height="18" fill="#36a9f4" /></g>`,
		`<rect x="0" y="0" width="1138" height="324" fill="green" opacity="0.05" class="call-sel" />`,
		`<rect x="6" y="71" width="600" height="235" fill="white" class="call-back" filter="url(#shadow)" stroke="#ccc" stroke-width="1" />`,
		`<text x="50" y="101" font-weight="bold">GET http://localhost:8000/evil-headers  200</text>`,
		`<g><rect x="32" y="147" width="20" height="20" fill="#fa7f03" /><text x="68" y="164">TCP handshake</text><text x="212" y="164">200.0 µs</text></g>`,
		`<text x="386" y="135">Start:</text><text x="486" y="135">0.0 µs</text>`,
		`<text x="486" y="165">3.0 ms</text>`,
		`<text x="486" y="195">12.0 B</text>`,
		`<a href="id-source.html#l1">`,
	} {
		if !strings.Contains(waterfall, s) {
			t.Errorf("waterfall doesn't contain %s\n%s", s, waterfall)
		}
	}
}
