// Package html escapes and unescapes HTML text.
package html

import "strings"

var escaper = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&#x27;",
)

// Escape replaces the special characters &, <, >, " and ' with HTML-safe sequences.
func Escape(text string) string {
	return escaper.Replace(text)
}
