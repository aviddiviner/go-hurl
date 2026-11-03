package cli

import (
	"slices"
	"strings"
	"unicode/utf8"
)

const about = "Hurl, run and test HTTP requests with plain text"

// helpStyles are Hurl's clap styles (cli/options/args.rs), as ANSI escapes.
type helpStyles struct {
	header, literal, placeholder, reset string
}

func newHelpStyles(color bool) helpStyles {
	if !color {
		return helpStyles{}
	}
	return helpStyles{
		header:      "\x1b[1m\x1b[32m",
		literal:     "\x1b[1m\x1b[36m",
		placeholder: "\x1b[36m",
		reset:       "\x1b[0m",
	}
}

func (s helpStyles) apply(style, text string) string {
	if style == "" {
		return text
	}
	return style + text + s.reset
}

const (
	tabWidth       = 2
	shortSize      = 4
	nextLineIndent = 8
)

// helpArg is an entry of the help: an option or the positional FILES.
type helpArg struct {
	def        *argDef // nil for FILES
	help       string
	positional bool
}

func (a helpArg) width() int {
	if a.positional {
		return utf8.RuneCountInString("[FILES]...")
	}
	return utf8.RuneCountInString(a.def.display())
}

func (a helpArg) specVals() string {
	if a.def == nil || len(a.def.possibleValues) == 0 {
		return ""
	}
	return "[possible values: " + strings.Join(a.def.possibleValues, ", ") + "]"
}

// renderHelp renders the help as clap does for Hurl's command, wrapped at
// width columns.
func renderHelp(binName string, width int, color bool) string {
	st := newHelpStyles(color)
	var b strings.Builder
	b.WriteString(about + "\n\n")
	b.WriteString(st.apply(st.header, "Usage:") + " " + usageLine(binName, st) + "\n\n")

	b.WriteString(st.apply(st.header, "Arguments:") + "\n")
	writeHelpArgs(&b, []helpArg{{help: "Set the input file to use", positional: true}}, width, st)

	var options []helpArg
	var headings []string
	for _, a := range argDefs {
		if a.heading == "" {
			options = append(options, helpArg{def: a, help: a.help})
		} else if !slices.Contains(headings, a.heading) {
			headings = append(headings, a.heading)
		}
	}
	b.WriteString("\n\n" + st.apply(st.header, "Options:") + "\n")
	writeHelpArgs(&b, options, width, st)

	for _, heading := range headings {
		var args []helpArg
		for _, a := range argDefs {
			if a.heading == heading {
				args = append(args, helpArg{def: a, help: a.help})
			}
		}
		b.WriteString("\n\n" + st.apply(st.header, heading+":") + "\n")
		writeHelpArgs(&b, args, width, st)
	}
	return strings.TrimRight(b.String(), " \n") + "\n"
}

func usageLine(binName string, st helpStyles) string {
	return st.apply(st.literal, binName) + " " + st.apply(st.placeholder, "[OPTIONS]") + " " + st.apply(st.placeholder, "[FILES]...")
}

// writeHelpArgs writes a help section, as clap's HelpTemplate::write_args.
func writeHelpArgs(b *strings.Builder, args []helpArg, termWidth int, st helpStyles) {
	longest := 2
	for _, a := range args {
		w := a.width()
		if !a.positional {
			w += shortSize
		}
		longest = max(longest, w)
	}
	nextLine := false
	for _, a := range args {
		taken := longest + tabWidth*2
		hw := utf8.RuneCountInString(a.help) + utf8.RuneCountInString(a.specVals())
		if termWidth >= taken && float32(taken)/float32(termWidth) > 0.40 && hw > termWidth-taken {
			nextLine = true
		}
	}
	for i, a := range args {
		if i != 0 {
			b.WriteString("\n")
		}
		b.WriteString("  ")
		if a.positional {
			b.WriteString(st.apply(st.placeholder, "[FILES]..."))
		} else {
			if a.def.short != "" {
				b.WriteString(st.apply(st.literal, "-"+a.def.short) + ", ")
			} else {
				b.WriteString("    ")
			}
			b.WriteString(st.apply(st.literal, "--"+a.def.long))
			if !a.def.isFlag() {
				b.WriteString(st.apply(st.placeholder, " ") + st.apply(st.placeholder, "<"+a.def.valueName+">"))
			}
		}
		var spaces int
		if nextLine {
			b.WriteString("\n" + strings.Repeat(" ", tabWidth+nextLineIndent))
			spaces = tabWidth + nextLineIndent
		} else {
			selfLen := a.width()
			if !a.positional {
				selfLen += shortSize
			}
			b.WriteString(strings.Repeat(" ", longest+tabWidth-selfLen))
			spaces = longest + tabWidth*2
		}
		help := a.help
		if sv := a.specVals(); sv != "" {
			if help != "" {
				help += " "
			}
			help += sv
		}
		help = wrapHelp(help, max(termWidth-spaces, 0))
		b.WriteString(strings.ReplaceAll(help, "\n", "\n"+strings.Repeat(" ", spaces)))
	}
}

// wrapHelp wraps a single line of text like clap's LineWrapper: words keep
// their trailing spaces, and the first word is never wrapped.
func wrapHelp(line string, hardWidth int) string {
	var words []string
	start := 0
	inSpace := false
	for i, c := range line {
		isSpace := c == ' '
		if inSpace && !isSpace {
			words = append(words, line[start:i])
			start = i
		}
		inSpace = isSpace
	}
	if start < len(line) {
		words = append(words, line[start:])
	}
	var out []string
	lineWidth := 0
	first := true
	for _, w := range words {
		trimmed := strings.TrimRight(w, " ")
		ww := utf8.RuneCountInString(trimmed)
		if first && ww > 0 {
			first = false
		} else if hardWidth < lineWidth+ww {
			if n := len(out); n > 0 {
				out[n-1] = strings.TrimRight(out[n-1], " ")
			}
			lineWidth = 0
			out = append(out, "\n")
		}
		lineWidth += ww + len(w) - len(trimmed)
		out = append(out, w)
	}
	return strings.TrimRight(strings.Join(out, ""), " \n")
}
