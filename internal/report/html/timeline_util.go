package html

import (
	"unicode/utf8"
)

// truncStr truncates text if it has more than maxLen bytes (with ellipsis).
func truncStr(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	// Hurl slices bytes and panics in the middle of a char: we cut before.
	for maxLen > 0 && !utf8.RuneStart(text[maxLen]) {
		maxLen--
	}
	return text[:maxLen] + "..."
}

// newStripes returns the stripe background SVG (1 call over 2).
func newStripes(nbStripes int, stripeHeight float64, pixelsX, pixelsY interval[float64], color string) *svgElement {
	group := newSVGGroup()
	group.addAttr("class", "grid-strip")
	x := pixelsX.start
	width := pixelsX.end - pixelsX.start

	// We want to have an odd number of stripes to have a filled strip at the
	// bottom.
	nbCalls := 2*(nbStripes/2) + 1
	for index := 0; index < nbCalls; index += 2 {
		y := float64(float64(index)*stripeHeight) + pixelsY.start
		group.addChild(newSVGRect(x, y, width, stripeHeight, color))
	}
	return group
}

// newSuccessIcon returns the SVG success icon identified by id.
func newSuccessIcon(id string) *svgElement {
	return newIcon(id, 512, 512, "M256 512A256 256 0 1 0 256 0a256 256 0 1 0 0 512zM369 209L241 337c-9.4 9.4-24.6 9.4-33.9 0l-64-64c-9.4-9.4-9.4-24.6 0-33.9s24.6-9.4 33.9 0l47 47L335 175c9.4-9.4 24.6-9.4 33.9 0s9.4 24.6 0 33.9z", "#10bb00")
}

// newFailureIcon returns the SVG failure icon identified by id.
func newFailureIcon(id string) *svgElement {
	return newIcon(id, 512, 512, "M256 512A256 256 0 1 0 256 0a256 256 0 1 0 0 512zm0-384c13.3 0 24 10.7 24 24V264c0 13.3-10.7 24-24 24s-24-10.7-24-24V152c0-13.3 10.7-24 24-24zM224 352a32 32 0 1 1 64 0 32 32 0 1 1 -64 0z", "red")
}

// newRetryIcon returns the SVG retry icon identified by id.
func newRetryIcon(id string) *svgElement {
	return newIcon(id, 512, 512, "M256 512A256 256 0 1 0 256 0a256 256 0 1 0 0 512zm0-384c13.3 0 24 10.7 24 24V264c0 13.3-10.7 24-24 24s-24-10.7-24-24V152c0-13.3 10.7-24 24-24zM224 352a32 32 0 1 1 64 0 32 32 0 1 1 -64 0z", "gold")
}

func newIcon(id string, width, height float64, path, color string) *svgElement {
	symbol := newSVGElement("symbol")
	symbol.addAttr("id", id)
	symbol.addAttr("viewBox", viewBox(0, 0, width, height))
	p := newSVGPath(path)
	p.addAttr("fill", color)
	symbol.addChild(p)
	return symbol
}
