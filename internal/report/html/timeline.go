package html

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// Common constants used to construct the SVG timeline.
const (
	callHeight = 24.0
	callInset  = 3.0
)

type callContextKind int

const (
	// callSuccess is a call whose parent entry is successful.
	callSuccess callContextKind = iota
	// callFailure is a call whose parent entry is in error and has not been
	// retried.
	callFailure
	// callRetry is a call whose parent entry is in error and has been
	// retried.
	callRetry
)

func (k callContextKind) icon() string {
	switch k {
	case callFailure:
		return "#failure"
	case callRetry:
		return "#retry"
	}
	return "#success"
}

// callContext holds information to construct an SVG view of a call.
type callContext struct {
	kind callContextKind
	// line is the line number of the source entry.
	line ast.Index
	// entryIndex is the index of the runtime entry result.
	entryIndex ast.Index
	// callEntryIndex is the index of the call in its entry.
	callEntryIndex ast.Index
	// callIndex is the index of the call in the whole run.
	callIndex      ast.Index
	sourceFilename string
	runFilename    string
}

// runHref returns the link to this call in the run view.
func (c *callContext) runHref() string {
	return fmt.Sprintf("%s#e%s:c%s", c.runFilename, c.entryIndex, c.callEntryIndex)
}

// timelineHTML returns the HTML timeline of these entries. The AST hurlFile
// is used to construct URL with line numbers to the corresponding entry in
// the colored HTML source file.
func (t Testcase) timelineHTML(hurlFile *ast.HurlFile, content string, entries []runner.EntryResult, secrets []string) string {
	var calls []*httpclient.Call
	for i := range entries {
		for j := range entries[i].Calls {
			calls = append(calls, &entries[i].Calls[j])
		}
	}
	callCtxs := t.callContexts(hurlFile, entries)
	return formatTemplate(timelineTpl, map[string]string{
		"calls":        callsSVG(calls, callCtxs, secrets),
		"filename":     t.safeFilename(),
		"nav":          t.navHTML(content, tabTimeline, secrets),
		"nav_css":      navCSS,
		"timeline_css": timelineCSS,
		"waterfall":    waterfallSVG(calls, callCtxs, secrets),
	})
}

// callContexts returns a list of call contexts to record source line code,
// runtime entry and call indices.
func (t Testcase) callContexts(hurlFile *ast.HurlFile, entries []runner.EntryResult) []callContext {
	var ctxs []callContext
	for entryIndex, e := range entries {
		// The last entry of the whole run can't be retried.
		retry := entryIndex+1 < len(entries) && e.EntryIndex == entries[entryIndex+1].EntryIndex
		kind := callSuccess
		if len(e.Errors) > 0 {
			kind = callFailure
			if retry {
				kind = callRetry
			}
		}
		for callEntryIndex := range e.Calls {
			entrySrc := hurlFile.Entries[e.EntryIndex.ToZeroBased()]
			ctxs = append(ctxs, callContext{
				kind:           kind,
				line:           ast.NewIndex(entrySrc.SourceInfo().Start.Line),
				entryIndex:     ast.IndexFromZeroBased(entryIndex),
				callEntryIndex: ast.IndexFromZeroBased(callEntryIndex),
				callIndex:      ast.IndexFromZeroBased(len(ctxs)),
				sourceFilename: t.sourceFilename(),
				runFilename:    t.runFilename(),
			})
		}
	}
	return ctxs
}

// callsSVG returns an SVG view of calls using contexts callCtxs.
func callsSVG(calls []*httpclient.Call, callCtxs []callContext, secrets []string) string {
	marginTop := 50.0
	marginBottom := 250.0
	width := 260.0
	height := float64(24.0*float64(len(calls))) + marginTop + marginBottom
	height = maxF(100, height)

	root := newSVG()
	root.addAttr("viewBox", viewBox(0, 0, width, height))
	root.addAttr("width", fmtF64(width))
	root.addAttr("height", fmtF64(height))

	// Add styles, symbols for success and failure icons.
	root.addChild(newSVGStyle(callsCSS))
	root.addChild(newSuccessIcon("success"))
	root.addChild(newFailureIcon("failure"))
	root.addChild(newRetryIcon("retry"))

	// Add a flat background.
	elt := newSVGElement("rect")
	elt.addAttr("class", "calls-back")
	elt.addAttrF("x", 0)
	elt.addAttrF("y", 0)
	elt.addAttr("width", "100%")
	elt.addAttr("height", "100%")
	elt.addAttr("fill", "#fbfafd")
	root.addChild(elt)

	if len(calls) > 0 {
		// Add horizontal lines.
		root.addChild(newCallsGrid(calls, marginTop, width, height))
		// Add calls info.
		root.addChild(newCalls(calls, callCtxs, 0, marginTop, secrets))
	}
	return root.String()
}

// newCalls returns an SVG view of a list of calls, like:
//
//	✅ GET www.google.fr 303 <run>
func newCalls(calls []*httpclient.Call, callCtxs []callContext, offsetX, offsetY float64, secrets []string) *svgElement {
	group := newSVGGroup()
	group.addAttr("class", "calls-list")
	group.addAttr("font-size", "13px")
	group.addAttr("font-family", "sans-serif")
	group.addAttr("fill", "#777")

	marginLeft := 13.0

	for index := range min(len(calls), len(callCtxs)) {
		call, ctx := calls[index], &callCtxs[index]
		x := offsetX + marginLeft
		y := float64(offsetY+float64(callHeight*float64(index))) + callHeight - 7

		// Icon success / failure.
		elt := newSVGElement("use")
		elt.addAttr("href", ctx.kind.icon())
		elt.addAttrF("x", x-6)
		elt.addAttrF("y", y-11)
		elt.addAttr("width", "13")
		elt.addAttr("height", "13")
		group.addChild(elt)

		x += 12

		// URL.
		url := util.Redact(call.Request.Url.String(), secrets)
		url = strings.TrimPrefix(url, "http://")
		url = strings.TrimPrefix(url, "https://")
		fullText := call.Request.Method + " " + url
		elt = newSVGText(x, y, truncStr(fullText, 24))
		if ctx.kind == callFailure {
			elt.addAttr("fill", "red")
		}
		elt.addChild(newSVGTitle(fullText))
		group.addChild(elt)

		// Status code.
		x += 180
		elt = newSVGText(x, y, strconv.FormatUint(uint64(call.Response.Status), 10))
		if ctx.kind == callFailure {
			elt.addAttr("fill", "red")
		}
		group.addChild(elt)

		// Source.
		x += 28
		a := newSVGA(ctx.runHref())
		text := newSVGText(x, y, "run")
		text.addAttr("fill", "royalblue")
		text.addAttr("text-decoration", "underline")
		a.addChild(text)
		group.addChild(a)
	}
	return group
}

// newCallsGrid returns an SVG view of the calls grid.
func newCallsGrid(calls []*httpclient.Call, offsetY, width, height float64) *svgElement {
	group := newSVGGroup()
	group.addAttr("class", "calls-grid")
	nbLines := 2*(len(calls)/2) + 2
	for index := range nbLines {
		y := float64(callHeight*float64(index)) + offsetY - float64(index%2)
		group.addChild(newSVGRect(0, y, width, 1, "#ddd"))
	}
	// Right borders.
	group.addChild(newSVGRect(width-1, 0, 1, height, "#ddd"))
	return group
}
