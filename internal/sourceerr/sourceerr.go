// Package sourceerr renders errors annotated with their source: a
// description, the file location, the offending line with a line-number
// gutter and carets. Ported from hurl_core's error module; used by both the
// parser and the runner.
package sourceerr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/text"
)

// DisplaySourceError is an error that can be displayed against the source
// it occurred in.
type DisplaySourceError interface {
	SourceInfo() ast.SourceInfo
	Description() string
	Fixme(content []string) text.StyledString
}

// Messager is implemented by errors that replace the default Message.
type Messager interface {
	Message(content []string) text.StyledString
}

// OutputFormat is either OutputFormatPlain or OutputFormatTerminal.
type OutputFormat interface{ outputFormat() }

type OutputFormatPlain struct{}

// OutputFormatTerminal renders for a terminal: CRLF is replaced by LF, and
// ANSI colors are used if Color is set.
type OutputFormatTerminal struct{ Color bool }

func (OutputFormatPlain) outputFormat()    {}
func (OutputFormatTerminal) outputFormat() {}

// Message returns the message for e: the source line followed by e's fixme,
// with a line-number gutter. For example:
//
//	GET abc
//	    ^ expecting http://, https:// or {{
//
// It uses e's own Message method if e implements Messager.
func Message(e DisplaySourceError, content []string) text.StyledString {
	if m, ok := e.(Messager); ok {
		return m.Message(content)
	}
	return DefaultMessage(e, content)
}

// DefaultMessage is the default implementation of Message.
func DefaultMessage(e DisplaySourceError, content []string) text.StyledString {
	var t text.StyledString
	line := e.SourceInfo().Start.Line
	AddSourceLine(&t, content, line)
	t.Append(e.Fixme(content))
	return AddLineInfoPrefix(t, content, line)
}

// Render returns the representation of e, given the file content and
// filename. entrySrcInfo is the optional source info of the entry where the
// error happened. For example:
//
//	Assert status code                        | > Description()
//	 --> test.hurl:2:10                       | > filename and position
//	  |                                       |
//	  | GET http://foo.com                    | > entry line
//	  | ...                                   |
//	2 | HTTP/1.0 200                          | > Message()
//	  |          ^^^ actual value is <404>    | >
//	  |                                       |
func Render(e DisplaySourceError, filename, content string, entrySrcInfo *ast.SourceInfo, format OutputFormat) string {
	var t text.StyledString
	lines := Lines(content)

	errorLine := e.SourceInfo().Start.Line
	errorColumn := e.SourceInfo().Start.Column

	t.PushWith(e.Description(), text.NewStyle().Bold())
	t.Push("\n")

	spaces := strings.Repeat(" ", gutterWidth(lines))
	var prefix text.StyledString
	prefix.PushWith(spaces+" |", text.NewStyle().Blue().Bold())

	addFilenameWithSourceInfo(&t, spaces, filename, errorLine, errorColumn)

	t.Append(prefix)

	if entrySrcInfo != nil {
		addEntryLine(&t, lines, errorLine, entrySrcInfo.Start.Line, prefix)
	}

	t.Append(Message(e, lines))
	return formatErrorMessage(t, format)
}

func gutterWidth(content []string) int {
	return max(len(strconv.Itoa(len(content))), 2)
}

// AddCarets prefixes message with carets under the source position of
// sourceInfo. Following lines of message are indented to align with the
// first one.
func AddCarets(message string, sourceInfo ast.SourceInfo, content []string) string {
	errorLine := sourceInfo.Start.Line
	errorColumn := sourceInfo.Start.Column
	// Start and end can be on different lines, so we ensure a minimum width.
	width := 1
	if sourceInfo.End.Column > errorColumn {
		width = sourceInfo.End.Column - errorColumn
	}
	prefix := getCarets(lineAt(content, errorLine), errorColumn, width)

	var b strings.Builder
	for i, line := range Lines(message) {
		if i == 0 {
			b.WriteString(prefix + line)
			continue
		}
		b.WriteByte('\n')
		if line != "" {
			b.WriteString(strings.Repeat(" ", len(prefix)))
		}
		b.WriteString(line)
	}
	return b.String()
}

// AddLineInfoPrefix prefixes every line of t with the gutter: the first line
// with errorLine, the following ones with a blank gutter. A trailing empty
// gutter line is added if t doesn't already end with one.
func AddLineInfoPrefix(t text.StyledString, content []string, errorLine int) text.StyledString {
	width := gutterWidth(content)
	style := text.NewStyle().Blue().Bold()
	var prefix, prefixWithNumber text.StyledString
	prefix.PushWith(strings.Repeat(" ", width)+" |", style)
	prefixWithNumber.PushWith(fmt.Sprintf("%*d |", width, errorLine), style)

	var t2 text.StyledString
	for i, line := range t.Split('\n') {
		t2.Push("\n")
		if i == 0 {
			t2.Append(prefixWithNumber)
		} else {
			t2.Append(prefix)
		}
		t2.Append(line)
	}

	if !t2.EndsWith("|") {
		t2.Push("\n")
		t2.Append(prefix)
	}
	return t2
}

// getCarets returns the caret prefix for a column. Tabs before the column
// count for 4 columns, since source lines are displayed with tabs replaced by
// 4 spaces.
func getCarets(lineRaw string, errorColumn, width int) string {
	tabShift := 0
	i := 0
	for _, c := range lineRaw {
		if i >= errorColumn-1 {
			break
		}
		if c == '\t' {
			tabShift++
		}
		i++
	}
	return strings.Repeat(" ", errorColumn+tabShift*3) + strings.Repeat("^", width) + " "
}

// AddSourceLine appends the 1-based line of content to t (with tabs replaced
// by 4 spaces), or an empty line if there is no such line.
func AddSourceLine(t *text.StyledString, content []string, line int) {
	s := strings.ReplaceAll(lineAt(content, line), "\t", "    ")
	t.Push(" ")
	t.Push(s)
	t.Push("\n")
}

func lineAt(content []string, line int) string {
	if line >= 1 && line <= len(content) {
		return content[line-1]
	}
	return ""
}

func addFilenameWithSourceInfo(t *text.StyledString, spaces, filename string, errorLine, errorColumn int) {
	t.Push(spaces)
	t.PushWith("-->", text.NewStyle().Blue().Bold())
	t.Push(fmt.Sprintf(" %s:%d:%d", filename, errorLine, errorColumn))
	t.Push("\n")
}

func addEntryLine(t *text.StyledString, lines []string, errorLine, entryLine int, prefix text.StyledString) {
	if entryLine != errorLine {
		line := strings.ReplaceAll(lines[entryLine-1], "\t", "    ")
		t.Push("\n")
		t.Append(prefix)
		t.Push(" ")
		t.PushWith(line, text.NewStyle().BrightBlack())
	}
	if errorLine-entryLine > 1 {
		t.Push("\n")
		t.Append(prefix)
		t.PushWith(" ...", text.NewStyle().BrightBlack())
	}
}

func formatErrorMessage(message text.StyledString, format OutputFormat) string {
	switch f := format.(type) {
	case OutputFormatTerminal:
		textFormat := text.FormatPlain
		if f.Color {
			textFormat = text.FormatANSI
		}
		// CRLF must be replaced by LF in the terminal.
		return strings.ReplaceAll(message.Render(textFormat), "\r\n", "\n")
	default:
		return message.Render(text.FormatPlain)
	}
}

// Lines splits s into lines like Rust's str::lines: lines end with "\n" or
// "\r\n" (the terminator is removed), and a final empty line is dropped.
func Lines(s string) []string {
	var lines []string
	for line := range strings.SplitAfterSeq(s, "\n") {
		if l, ok := strings.CutSuffix(line, "\n"); ok {
			line = strings.TrimSuffix(l, "\r")
		} else if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}
