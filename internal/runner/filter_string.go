package runner

import (
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/html"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

// evalHTMLEscape converts the characters &, <, > (and quotes) to HTML-safe
// sequences.
func evalHTMLEscape(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	return ValueString{html.Escape(s.Value)}, nil
}

// evalHTMLUnescape converts named and numeric character references to the
// corresponding characters.
func evalHTMLUnescape(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	return ValueString{html.Unescape(s.Value)}, nil
}

// evalRegex extracts the first capture group of a regex.
func evalRegex(value Value, regex ast.RegexValue, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	re, err := evalRegexValue(regex, variables)
	if err != nil {
		return nil, err
	}
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	m := re.FindStringSubmatchIndex(s.Value)
	if len(m) < 4 || m[2] < 0 {
		return nil, nil
	}
	return ValueString{s.Value[m[2]:m[3]]}, nil
}

// evalReplace replaces all occurrences of a string.
func evalReplace(value Value, variables *VariableSet, si ast.SourceInfo, assert bool, oldValue, newValue ast.Template) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	o, err := evalTemplate(oldValue, variables)
	if err != nil {
		return nil, err
	}
	n, err := evalTemplate(newValue, variables)
	if err != nil {
		return nil, err
	}
	return ValueString{strings.ReplaceAll(s.Value, o, n)}, nil
}

// evalReplaceRegex replaces all matches of a regex ($1, ${name}... expand
// to capture groups).
func evalReplaceRegex(value Value, variables *VariableSet, si ast.SourceInfo, assert bool, pattern ast.RegexValue, newValue ast.Template) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	re, err := evalRegexValue(pattern, variables)
	if err != nil {
		return nil, err
	}
	n, err := evalTemplate(newValue, variables)
	if err != nil {
		return nil, err
	}
	return ValueString{re.ReplaceAllString(s.Value, n)}, nil
}

// evalSplit splits a string around a delimiter.
func evalSplit(value Value, variables *VariableSet, si ast.SourceInfo, assert bool, sep ast.Template) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	delimiter, err := evalTemplate(sep, variables)
	if err != nil {
		return nil, err
	}
	var parts []string
	if delimiter == "" {
		// Rust's str::split("") also yields an empty first and last item.
		parts = append([]string{""}, strings.Split(s.Value, "")...)
		parts = append(parts, "")
		if s.Value == "" {
			parts = []string{"", ""}
		}
	} else {
		parts = strings.Split(s.Value, delimiter)
	}
	values := make([]Value, len(parts))
	for i, p := range parts {
		values[i] = ValueString{p}
	}
	return ValueList{values}, nil
}

// evalURLDecode replaces %xx escapes with the bytes they encode (invalid
// UTF-8 is replaced with U+FFFD).
func evalURLDecode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	return ValueString{utf8Lossy(percentDecode(s.Value))}, nil
}

// percentDecode is percent_encoding::percent_decode_str: invalid escapes
// are kept as is.
func percentDecode(s string) []byte {
	unhex := func(c byte) (byte, bool) {
		switch {
		case c >= '0' && c <= '9':
			return c - '0', true
		case c >= 'a' && c <= 'f':
			return c - 'a' + 10, true
		case c >= 'A' && c <= 'F':
			return c - 'A' + 10, true
		}
		return 0, false
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			h, ok1 := unhex(s[i+1])
			l, ok2 := unhex(s[i+2])
			if ok1 && ok2 {
				out = append(out, h<<4|l)
				i += 2
				continue
			}
		}
		out = append(out, s[i])
	}
	return out
}

// evalURLEncode percent-encodes every character except the RFC 3986
// unreserved characters and '/' (like Jinja's urlencode).
func evalURLEncode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	const upperhex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s.Value); i++ {
		c := s.Value[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || strings.IndexByte("-._~/", c) >= 0 {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(upperhex[c>>4])
			b.WriteByte(upperhex[c&15])
		}
	}
	return ValueString{b.String()}, nil
}

// evalURLQueryParam returns the value of a query parameter of a URL.
func evalURLQueryParam(value Value, param ast.Template, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	name, err := evalTemplate(param, variables)
	if err != nil {
		return nil, err
	}
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	u, uerr := httpclient.ParseUrl(s.Value)
	if uerr != nil {
		return nil, NewRunnerError(si, RunnerErrorInvalidURL{URL: uerr.Url, Message: uerr.Reason}, assert)
	}
	for _, p := range u.QueryParams() {
		if p.Name == name {
			return ValueString{p.Value}, nil
		}
	}
	return nil, nil
}

// evalUTF8Decode decodes bytes as UTF-8, replacing invalid sequences with
// U+FFFD.
func evalUTF8Decode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	b, ok := value.(ValueBytes)
	if !ok {
		return nil, invalidInputType(value, "bytes", si, assert)
	}
	return ValueString{utf8Lossy(b.Value)}, nil
}

// evalUTF8Encode encodes a string to its UTF-8 bytes.
func evalUTF8Encode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	return ValueBytes{[]byte(s.Value)}, nil
}

// utf8Lossy is Rust's String::from_utf8_lossy: each maximal invalid
// subsequence is replaced by a single U+FFFD.
func utf8Lossy(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	var sb strings.Builder
	for i := 0; i < len(b); {
		if r, size := utf8.DecodeRune(b[i:]); r != utf8.RuneError || size > 1 {
			sb.WriteRune(r)
			i += size
			continue
		}
		sb.WriteRune(utf8.RuneError)
		i += invalidPrefixLen(b[i:])
	}
	return sb.String()
}

// invalidPrefixLen returns the length of the maximal prefix of b that starts
// a valid UTF-8 sequence (at least 1), b being invalid at its start.
func invalidPrefixLen(b []byte) int {
	lo, hi := byte(0x80), byte(0xBF)
	var n int
	switch c := b[0]; {
	case c >= 0xC2 && c <= 0xDF:
		n = 2
	case c == 0xE0:
		n, lo = 3, 0xA0
	case c >= 0xE1 && c <= 0xEC, c == 0xEE, c == 0xEF:
		n = 3
	case c == 0xED:
		n, hi = 3, 0x9F
	case c == 0xF0:
		n, lo = 4, 0x90
	case c >= 0xF1 && c <= 0xF3:
		n = 4
	case c == 0xF4:
		n, hi = 4, 0x8F
	default:
		return 1
	}
	i := 1
	for ; i < n && i < len(b); i++ {
		if i > 1 {
			lo, hi = 0x80, 0xBF
		}
		if b[i] < lo || b[i] > hi {
			break
		}
	}
	return i
}
