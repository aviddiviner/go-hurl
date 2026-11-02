package httpclient

import (
	"regexp"
	"strings"
)

// IsKindOfText reports whether contentType is a kind of text (text, JSON or
// XML).
func IsKindOfText(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	return strings.Contains(ct, "text/") || IsJSON(ct) || IsXML(ct)
}

// IsHTML reports whether contentType is an HTML mime type.
func IsHTML(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "text/html")
}

var xmlPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^text/xml`),
	regexp.MustCompile(`^application/xml`),
	regexp.MustCompile(`^application/[a-z0-9-_.]+[+.]xml`),
}

var jsonPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^application/json`),
	regexp.MustCompile(`^application/[a-z0-9-_.]+[+.-]json`),
}

// IsXML reports whether contentType is an XML mime type.
func IsXML(contentType string) bool {
	return matchesAny(xmlPatterns, contentType)
}

// IsJSON reports whether contentType is a JSON mime type.
func IsJSON(contentType string) bool {
	return matchesAny(jsonPatterns, contentType)
}

func matchesAny(patterns []*regexp.Regexp, contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	for _, p := range patterns {
		if p.MatchString(ct) {
			return true
		}
	}
	return false
}

// MimeCharset extracts the charset parameter of a mime type.
func MimeCharset(mimeType string) (string, bool) {
	for part := range strings.SplitSeq(strings.TrimSpace(mimeType), ";") {
		param := strings.Split(strings.TrimSpace(part), "=")
		if len(param) == 2 && strings.EqualFold(strings.TrimSpace(param[0]), "charset") {
			return strings.TrimSpace(param[1]), true
		}
	}
	return "", false
}
