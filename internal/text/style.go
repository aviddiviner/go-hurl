// Package text provides styled strings that render as plain text or with
// ANSI escape codes, ported from hurl_core's text module.
//
// The ANSI output is byte-identical to the Rust colored crate as Hurl uses
// it. Hurl forces colored on at startup (init_crate_colored), so
// FormatANSI always emits escape codes; choosing plain output is up to the
// caller.
package text

// Color is a foreground color. The zero value is no color.
type Color int

const (
	ColorNone Color = iota
	ColorBlue
	ColorBrightBlack
	ColorCyan
	ColorGreen
	ColorMagenta
	ColorPurple
	ColorRed
	ColorYellow
)

// ansiCode returns the SGR foreground code used by the colored crate.
func (c Color) ansiCode() string {
	switch c {
	case ColorBlue:
		return "34"
	case ColorBrightBlack:
		return "90"
	case ColorCyan:
		return "36"
	case ColorGreen:
		return "32"
	case ColorMagenta, ColorPurple:
		return "35"
	case ColorRed:
		return "31"
	case ColorYellow:
		return "33"
	}
	return ""
}

// Style is an optional foreground color with a bold attribute. Build styles
// with the chaining methods: NewStyle().Red().Bold().
type Style struct {
	fg   Color
	bold bool
}

func NewStyle() Style { return Style{} }

func (s Style) Blue() Style        { s.fg = ColorBlue; return s }
func (s Style) BrightBlack() Style { s.fg = ColorBrightBlack; return s }
func (s Style) Cyan() Style        { s.fg = ColorCyan; return s }
func (s Style) Green() Style       { s.fg = ColorGreen; return s }
func (s Style) Magenta() Style     { s.fg = ColorMagenta; return s }
func (s Style) Purple() Style      { s.fg = ColorPurple; return s }
func (s Style) Red() Style         { s.fg = ColorRed; return s }
func (s Style) Yellow() Style      { s.fg = ColorYellow; return s }
func (s Style) Bold() Style        { s.bold = true; return s }

func (s Style) Fg() Color    { return s.fg }
func (s Style) IsBold() bool { return s.bold }
