package text

import "strings"

// Format is the rendering format of a StyledString.
type Format int

const (
	FormatPlain Format = iota
	FormatANSI
)

// StyledString is a string made of styled parts (tokens), rendered as plain
// text or with ANSI escape codes. Consecutive pushes with the same style are
// merged into one token.
//
// The zero value is an empty string. A StyledString shares storage when
// copied by assignment; use Clone to get an independent copy.
type StyledString struct {
	tokens []token
}

type token struct {
	content string
	style   Style
}

func NewStyledString() StyledString { return StyledString{} }

// Push appends content without any style.
func (s *StyledString) Push(content string) { s.PushWith(content, NewStyle()) }

// PushWith appends content with a specific style.
func (s *StyledString) PushWith(content string, style Style) {
	s.pushToken(token{content: content, style: style})
}

func (s *StyledString) pushToken(t token) {
	if n := len(s.tokens); n > 0 && s.tokens[n-1].style == t.style {
		s.tokens[n-1].content += t.content
		return
	}
	s.tokens = append(s.tokens, t)
}

// Render renders this styled string in the given format (Rust's
// StyledString::to_string(format)).
func (s StyledString) Render(format Format) string {
	var b strings.Builder
	for _, t := range s.tokens {
		if format == FormatANSI {
			b.WriteString(t.ansi())
		} else {
			b.WriteString(t.content)
		}
	}
	return b.String()
}

// Append appends other to this styled string.
func (s *StyledString) Append(other StyledString) {
	for _, t := range other.tokens {
		s.pushToken(t)
	}
}

// Clone returns an independent copy of s.
func (s StyledString) Clone() StyledString {
	return StyledString{tokens: append([]token(nil), s.tokens...)}
}

// Split splits this styled string around each delimiter.
func (s StyledString) Split(delimiter rune) []StyledString {
	var items []StyledString
	var item StyledString
	for _, t := range s.tokens {
		substrings := strings.Split(t.content, string(delimiter))
		if substrings[0] != "" {
			item.PushWith(substrings[0], t.style)
		}
		for _, sub := range substrings[1:] {
			items = append(items, item)
			item = StyledString{}
			if sub != "" {
				item.PushWith(sub, t.style)
			}
		}
	}
	return append(items, item)
}

// EndsWith reports whether the plain rendering ends with value.
func (s StyledString) EndsWith(value string) bool {
	return strings.HasSuffix(s.Render(FormatPlain), value)
}

// Len returns the length in bytes of the visible content.
func (s StyledString) Len() int {
	n := 0
	for _, t := range s.tokens {
		n += len(t.content)
	}
	return n
}

func (s StyledString) IsEmpty() bool { return s.Len() == 0 }

// Wrap inserts newlines so that each line has at most maxWidth runes.
func (s StyledString) Wrap(maxWidth int) StyledString {
	var out StyledString
	width := 0
	for _, t := range s.tokens {
		runes := []rune(t.content)
		start := 0
		for i := range runes {
			width++
			if width >= maxWidth {
				out.pushToken(token{content: string(runes[start : i+1]), style: t.style})
				if i+1 < len(runes) {
					out.pushToken(token{content: "\n", style: NewStyle()})
				}
				start = i + 1
				width = 0
			}
		}
		if start < len(runes) {
			out.pushToken(token{content: string(runes[start:]), style: t.style})
		}
	}
	return out
}

// Clear removes all contents.
func (s *StyledString) Clear() { s.tokens = nil }

// ansi renders the token like the colored crate: "\x1b[" + [1;]fg + "m" +
// content + "\x1b[0m", re-applying the style after any reset sequence
// embedded in the content.
func (t token) ansi() string {
	if t.style.fg == ColorNone && !t.style.bold {
		return t.content
	}
	var codes []string
	if t.style.bold {
		codes = append(codes, "1")
	}
	if t.style.fg != ColorNone {
		codes = append(codes, t.style.fg.ansiCode())
	}
	const reset = "\x1b[0m"
	style := "\x1b[" + strings.Join(codes, ";") + "m"
	content := strings.ReplaceAll(t.content, reset, reset+style)
	return style + content + reset
}
