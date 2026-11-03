package html

import (
	"fmt"
	"math"
	"time"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// waterfallSVG returns the SVG waterfall of this list of calls.
func waterfallSVG(calls []*httpclient.Call, callCtxs []callContext, secrets []string) string {
	if len(calls) == 0 {
		return ""
	}
	times := interval[time.Time]{calls[0].Timings.BeginCall, calls[len(calls)-1].Timings.EndCall}

	marginTop := 50.0
	marginBottom := 250.0
	width := 1138.0
	height := float64(callHeight*float64(len(calls))) + marginTop + marginBottom
	height = maxF(100, height)

	root := newSVG()
	root.addAttr("viewBox", viewBox(0, 0, width, height))
	root.addAttr("width", fmtF64(width))
	root.addAttr("height", fmtF64(height))

	// Add styles, filters, symbols for success and failure icons.
	root.addChild(newSVGStyle(waterfallCSS))
	root.addChild(newFilters())
	root.addChild(newSuccessIcon("success"))
	root.addChild(newFailureIcon("failure"))
	root.addChild(newRetryIcon("retry"))

	pixelsX := interval[float64]{0, width}
	pixelsY := interval[float64]{marginTop, height}
	scaleX := newScale(times, pixelsX)

	ticksNumber := 10
	root.addChild(newWaterfallGrid(calls, times, ticksNumber, scaleX, pixelsX, pixelsY, callHeight))

	// We construct SVG calls from last to first so the detail of any call is
	// not overridden by the next call.
	for i := min(len(calls), len(callCtxs)) - 1; i >= 0; i-- {
		root.addChild(newCall(calls[i], &callCtxs[i], times, scaleX, pixelsX, pixelsY, secrets))
	}
	return root.String()
}

// newWaterfallGrid returns the grid SVG with ticks on "nice" times and stripes
// background.
func newWaterfallGrid(calls []*httpclient.Call, times interval[time.Time], ticksNumber int, scaleX scale, pixelsX, pixelsY interval[float64], callHeight float64) *svgElement {
	grid := newSVGGroup()
	grid.addChild(newStripes(len(calls), callHeight, pixelsX, pixelsY, "#f5f5f5"))
	grid.addChild(newVertLines(times, ticksNumber, scaleX, pixelsY))
	return grid
}

// newVertLines returns the vertical lines with labels for the time ticks.
func newVertLines(times interval[time.Time], ticksNumber int, scaleX scale, pixelsY interval[float64]) *svgElement {
	group := newSVGGroup()

	// We compute in which unit we're going to draw the grid.
	duration := float64(times.end.Sub(times.start).Microseconds())
	delta := duration / float64(ticksNumber)
	var start, end timeUnit
	switch {
	case delta < 1_000:
		start, end = timeUnit{unitMicrosecond, 0}, timeUnit{unitMicrosecond, duration}
	case delta < 1_000_000:
		start, end = timeUnit{unitMillisecond, 0}, timeUnit{unitMillisecond, duration / 1_000}
	default:
		start, end = timeUnit{unitSecond, 0}, timeUnit{unitSecond, duration / 1_000_000}
	}
	nice := newNiceScale(start.value, end.value, ticksNumber)

	type tick struct {
		x float64
		t timeUnit
	}
	var values []tick
	for t := start; t.less(end); t = t.addRaw(nice.tickSpacing) {
		// We want an integer pixel value.
		x := math.Round(scaleX.toPixel(t.microseconds()))
		values = append(values, tick{x, t})
	}

	// Draw the vertical lines.
	lines := newSVGGroup()
	lines.addAttr("class", "grid-ticks")
	lines.addAttr("stroke", "#ccc")
	for _, v := range values {
		if v.x <= 0 {
			continue
		}
		lines.addChild(newSVGLine(v.x, 0, v.x, pixelsY.end))
	}
	group.addChild(lines)

	// Finally, draw labels.
	labels := newSVGGroup()
	labels.addAttr("font-size", "15px")
	labels.addAttr("font-family", "sans-serif")
	labels.addAttr("fill", "#777")
	for _, v := range values {
		labels.addChild(newSVGText(v.x+5, 20, fmtF64(v.t.value)+" "+v.t.unit))
	}
	group.addChild(labels)
	return group
}

// newCall returns the SVG of a call. times is the time interval of the
// complete run, scaleX converts between times and pixels on the X-axis.
func newCall(call *httpclient.Call, ctx *callContext, times interval[time.Time], scaleX scale, pixelsX, pixelsY interval[float64], secrets []string) *svgElement {
	elt := newSVGGroup()
	elt.addChild(newCallTimings(call, ctx, times, scaleX, pixelsY))
	elt.addChild(newCallTooltip(call, ctx, times, scaleX, pixelsX, pixelsY, secrets))
	return elt
}

// since returns t - start, or 0 if negative.
func since(t, start time.Time) time.Duration { return max(0, t.Sub(start)) }

// checkedSub returns a - b, and false if negative (Rust's
// Duration::checked_sub).
func checkedSub(a, b time.Duration) (time.Duration, bool) {
	if a < b {
		return 0, false
	}
	return a - b, true
}

// newCallTimings returns the SVG timings block of a call.
func newCallTimings(call *httpclient.Call, ctx *callContext, times interval[time.Time], scaleX scale, pixelsY interval[float64]) *svgElement {
	group := newSVGGroup()
	group.addAttr("class", "call-summary")

	timings := call.Timings
	offsetY := float64(callHeight*float64(ctx.callIndex.ToZeroBased())) + pixelsY.start
	y := offsetY + callInset
	height := callHeight - float64(callInset*2)

	// DNS
	dnsX := durationToPixel(since(timings.BeginCall, times.start), scaleX)
	dnsWidth := durationToPixel(timings.NameLookup, scaleX)
	if dnsWidth > 0 {
		group.addChild(newSVGRect(dnsX, y, dnsWidth, height, "#1d9688"))
	}

	addRect := func(from, to time.Duration, color string) {
		x := durationToPixel(from, scaleX) + dnsX
		if d, ok := checkedSub(to, from); ok {
			if w := durationToPixel(d, scaleX); w > 0 {
				group.addChild(newSVGRect(x, y, w, height, color))
			}
		}
	}
	// TCP Handshake
	addRect(timings.NameLookup, timings.Connect, "#fa7f03")
	// SSL
	addRect(timings.Connect, timings.AppConnect, "#9933ff")
	// Wait
	addRect(timings.PreTransfer, timings.StartTransfer, "#18c852")
	// Data transfer
	addRect(timings.StartTransfer, timings.Total, "#36a9f4")
	return group
}

// newCallTooltip returns the SVG detail of a call. Each call timing value is
// displayed under the timings.
func newCallTooltip(call *httpclient.Call, ctx *callContext, times interval[time.Time], scaleX scale, pixelsX, pixelsY interval[float64], secrets []string) *svgElement {
	group := newSVGGroup()
	group.addAttr("class", "call-detail")
	group.addAttr("font-family", "sans-serif")
	group.addAttr("font-size", "17px")

	timings := call.Timings
	width := 600.0
	height := 235.0
	offsetX := durationToPixel(since(timings.BeginCall, times.start), scaleX)
	offsetY := float64(callHeight*float64(ctx.callIndex.ToZeroBased())) + pixelsY.start
	offsetY = offsetY + callHeight - callInset
	maxWidth := pixelsX.end - pixelsX.start
	// We bound the tooltip background to the overall bounding box.
	offsetX = maxF(offsetX, 6)
	offsetX = minF(offsetX, maxWidth-width-6)

	group.addChild(newCallSel(call, ctx, times, scaleX, pixelsY))

	x := offsetX
	y := offsetY

	elt := newSVGRect(x, y, width, height, "white")
	elt.addAttr("class", "call-back")
	elt.addAttr("filter", "url(#shadow)")
	elt.addAttr("stroke", "#ccc")
	elt.addAttrF("stroke-width", 1)
	group.addChild(elt)

	x += 14
	y += 14
	deltaY := 30.0

	legend := newSVGGroup()
	legend.addAttr("class", "call-legend")
	legend.addAttr("fill", "#555")

	// Icon + URL + method
	elt = newSVGElement("use")
	elt.addAttr("href", ctx.kind.icon())
	elt.addAttrF("x", x)
	elt.addAttrF("y", y)
	elt.addAttr("width", "20")
	elt.addAttr("height", "20")
	legend.addChild(elt)

	url := util.Redact(call.Request.Url.String(), secrets)
	text := truncStr(call.Request.Method+" "+url, 54)
	text = fmt.Sprintf("%s  %d", text, call.Response.Status)
	elt = newSVGText(x+30, y+16, text)
	if ctx.kind == callFailure {
		elt.addAttr("fill", "red")
	}
	elt.addAttr("font-weight", "bold")
	legend.addChild(elt)

	x += 12
	y += 32

	// DNS
	legend.addChild(newLegend(x, y, "DNS lookup", "#1d9688", micros(timings.NameLookup)))
	y += deltaY

	addLegend := func(from, to time.Duration, text, color string) {
		if d, ok := checkedSub(to, from); ok {
			legend.addChild(newLegend(x, y, text, color, micros(d)))
			y += deltaY
		}
	}
	addLegend(timings.NameLookup, timings.Connect, "TCP handshake", "#fa7f03")
	addLegend(timings.Connect, timings.AppConnect, "SSL handshake", "#9933ff")
	addLegend(timings.PreTransfer, timings.StartTransfer, "Wait", "#18c852")
	addLegend(timings.StartTransfer, timings.Total, "Data transfer", "#36a9f4")

	// Total
	elt = newLegend(x, y, "Total", "", micros(timings.Total))
	elt.addAttr("font-weight", "bold")
	legend.addChild(elt)

	// Start and stop timestamps
	start := since(timings.BeginCall, times.start)
	end := since(timings.EndCall, times.start)
	x = offsetX + 380
	y = offsetY + 64
	legend.addChild(newValue("Start:", microsHumanString(micros(start)), x, y))
	y += deltaY
	legend.addChild(newValue("Stop:", microsHumanString(micros(end)), x, y))

	y += deltaY
	legend.addChild(newValue("Transferred:", bytesHumanString(float64(len(call.Response.Body))), x, y))

	// Run URL
	y += 56
	legend.addChild(newLink(x, y, "(view run)", ctx.runHref()))

	// Source URL
	href := fmt.Sprintf("%s#l%s", ctx.sourceFilename, ctx.line)
	legend.addChild(newLink(x+90, y, "(view source)", href))

	// Timings explanation
	y += deltaY
	legend.addChild(newLink(x, y, "Explanation", "https://hurl.dev/docs/response.html#timings"))
	group.addChild(legend)
	return group
}

func newLink(x, y float64, text, href string) *svgElement {
	elt := newSVGText(x, y, text)
	elt.addAttr("fill", "royalblue")
	elt.addAttr("text-decoration", "underline")
	a := newSVGA(href)
	a.addChild(elt)
	return a
}

// newCallSel returns the highlighted span time of a call.
func newCallSel(call *httpclient.Call, ctx *callContext, times interval[time.Time], scaleX scale, pixelsY interval[float64]) *svgElement {
	start := durationToPixel(since(call.Timings.BeginCall, times.start), scaleX)
	end := durationToPixel(since(call.Timings.EndCall, times.start), scaleX)
	color := "green"
	if ctx.kind == callFailure {
		color = "red"
	}
	elt := newSVGRect(start, 0, end-start, pixelsY.end, color)
	elt.addAttrF("opacity", 0.05)
	elt.addAttr("class", "call-sel")
	return elt
}

// newLegend returns a legend line; color is optional.
func newLegend(x, y float64, text, color string, duration float64) *svgElement {
	dxLabel := 36.0
	dyLabel := 17.0
	dxDuration := 180.0

	group := newSVGGroup()
	if color != "" {
		group.addChild(newSVGRect(x, y, 20, 20, color))
	}
	group.addChild(newSVGText(x+dxLabel, y+dyLabel, text))
	group.addChild(newSVGText(x+dxDuration, y+dyLabel, microsHumanString(duration)))
	return group
}

func newValue(label, value string, x, y float64) *svgElement {
	group := newSVGGroup()
	group.addChild(newSVGText(x, y, label))
	group.addChild(newSVGText(x+100, y, value))
	return group
}

// micros returns d in microseconds (Rust's Duration::as_micros).
func micros(d time.Duration) float64 { return float64(d.Microseconds()) }

// durationToPixel converts a duration to pixels, using scaleX.
func durationToPixel(d time.Duration, scaleX scale) float64 {
	return scaleX.toPixel(micros(d))
}

// newFilters creates SVG filters for the waterfall (used by drop shadow of
// call tooltip).
func newFilters() *svgElement {
	defs := newSVGElement("defs")
	filter := newSVGElement("filter")
	filter.addAttr("id", "shadow")
	shadow := newSVGElement("feDropShadow")
	shadow.addAttrF("dx", 0)
	shadow.addAttrF("dy", 4)
	shadow.addAttrF("stdDeviation", 4)
	shadow.addAttrF("flood-opacity", 0.25)
	filter.addChild(shadow)
	defs.addChild(filter)
	return defs
}

// microsHumanString returns a human readable string of a duration in
// microseconds.
func microsHumanString(d float64) string {
	switch {
	case d < 0:
		return "_"
	case d < 1_000:
		return fmt.Sprintf("%.1f µs", d)
	case d < 1_000_000:
		return fmt.Sprintf("%.1f ms", d/1_000)
	}
	return fmt.Sprintf("%.1f s", d/1_000_000)
}

// bytesHumanString returns a human readable string of a size in bytes.
func bytesHumanString(d float64) string {
	switch {
	case d < 0:
		return "_"
	case d < 1_000:
		return fmt.Sprintf("%.1f B", d)
	case d < 1_000_000:
		return fmt.Sprintf("%.1f kB", d/1_000)
	}
	return fmt.Sprintf("%.1f MB", d/1_000_000)
}
