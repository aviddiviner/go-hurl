// Package curlimport converts curl command lines to Hurl, ported from
// hurlfmt's curl module.
package curlimport

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Parse converts curl command lines (one per line, with optional backslash
// continuations) to Hurl entries.
func Parse(s string) (string, error) {
	s = strings.ReplaceAll(s, "\\\n", "")
	s = strings.ReplaceAll(s, "\\\r\n", "")
	var b strings.Builder
	i := 0
	for line := range strings.SplitSeq(s, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		i++
		hurl, err := parseLine(line)
		if err != nil {
			return "", fmt.Errorf("Can not parse curl command at line %d: %s", i, err)
		}
		b.WriteString(hurl + "\n")
	}
	return b.String(), nil
}

const curlHelp = `Usage: curl [OPTIONS] [url_param]

Arguments:
  [url_param]  Sets the url to use

Options:
      --compressed
  -d, --data <data>
      --data-raw <data>
      --digest
  -H, --header <NAME:VALUE>
  -b, --cookie <NAME1=VALUE1; NAME2=VALUE2>
  -k, --insecure
  -v, --verbose
      --negotiate
      --ntlm
  -L, --location
      --max-redirs <NUM>
  -X, --request <METHOD>
      --retry <seconds>
  -u, --user <user>
      --url <url>
  -h, --help                                 Print help
`

var curlCommand = &command{
	name:       "curl",
	positional: "url_param",
	help:       curlHelp,
	args: []*arg{
		{id: "compressed", long: "compressed"},
		{id: "data", long: "data", short: 'd', valueName: "data", overrides: "data_raw"},
		{id: "data_raw", long: "data-raw", valueName: "data", overrides: "data"},
		{id: "digest", long: "digest"},
		{id: "headers", long: "header", short: 'H', valueName: "NAME:VALUE", appendVals: true, parse: parseHeader},
		{id: "cookies", long: "cookie", short: 'b', valueName: "NAME1=VALUE1; NAME2=VALUE2", appendVals: true, parse: parseCookies},
		{id: "insecure", long: "insecure", short: 'k'},
		{id: "verbose", long: "verbose", short: 'v'},
		{id: "negotiate", long: "negotiate"},
		{id: "ntlm", long: "ntlm"},
		{id: "location", long: "location", short: 'L'},
		{id: "max_redirects", long: "max-redirs", valueName: "NUM", allowHyphen: true, parse: parseInt32(-1)},
		{id: "method", long: "request", short: 'X', valueName: "METHOD"},
		{id: "retry", long: "retry", valueName: "seconds", parse: parseInt32(-1 << 31)},
		{id: "user", long: "user", short: 'u', valueName: "user"},
		{id: "url", long: "url", valueName: "url"},
	},
}

func parseHeader(value string) (string, error) {
	// A basic format check on headers, accepting either "NAME: VALUE" or
	// "NAME;" for an empty header. See <https://curl.se/docs/manpage.html#-H>.
	if strings.Contains(value, ":") || strings.HasSuffix(value, ";") {
		return value, nil
	}
	return "", errors.New("headers must be formatted as '<NAME:VALUE>' or '<NAME>;'")
}

func parseCookies(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", errors.New("empty value provided")
	}
	var valid, invalid []string
	for c := range strings.SplitSeq(value, ";") {
		c = strings.TrimSpace(c)
		switch {
		case c == "":
		case strings.Contains(c, "="):
			valid = append(valid, c)
		default:
			invalid = append(invalid, c)
		}
	}
	switch len(invalid) {
	case 0:
		return strings.Join(valid, "; "), nil
	case 1:
		return "", errors.New("invalid cookie pair provided")
	}
	return "", fmt.Errorf("invalid cookie pairs provided: [%s]", strings.Join(invalid, ", "))
}

// parseInt32 parses an i32 in min..=i32::MAX, with Rust's error messages.
func parseInt32(min int64) func(string) (string, error) {
	const maxInt32 = 1<<31 - 1
	return func(value string) (string, error) {
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			switch {
			case value == "":
				return "", errors.New("cannot parse integer from empty string")
			case errors.Is(err, strconv.ErrRange) && strings.HasPrefix(value, "-"):
				return "", errors.New("number too small to fit in target type")
			case errors.Is(err, strconv.ErrRange):
				return "", errors.New("number too large to fit in target type")
			}
			return "", errors.New("invalid digit found in string")
		}
		if n < min || n > maxInt32 {
			return "", fmt.Errorf("%d is not in %d..=%d", n, min, maxInt32)
		}
		return strconv.FormatInt(n, 10), nil
	}
}

type hurlOption struct{ name, value string }

func parseLine(s string) (string, error) {
	params, err := split(s)
	if err != nil {
		return "", err
	}
	m, err := curlCommand.parse(params)
	if err != nil {
		return "", err
	}
	url, err := urlFrom(m)
	if err != nil {
		return "", err
	}
	return format(method(m), url, headers(m), m.values["cookies"], options(m), body(m)), nil
}

func format(method, url string, headers, cookies []string, options []hurlOption, body *string) string {
	s := method + " " + url
	for _, h := range headers {
		if stripped, ok := strings.CutSuffix(h, ";"); ok {
			s += "\n" + stripped + ":"
		} else {
			s += "\n" + h
		}
	}
	if len(cookies) > 0 {
		s += "\ncookie: " + strings.Join(cookies, "; ")
	}
	if len(options) > 0 {
		s += "\n[Options]"
		for _, o := range options {
			s += "\n" + o.name + ": " + o.value
		}
	}
	if body != nil {
		s += "\n" + *body
	}
	if asserts := additionalAsserts(options); len(asserts) > 0 {
		s += "\nHTTP *\n[Asserts]"
		for _, a := range asserts {
			s += "\n" + a
		}
	}
	return s + "\n"
}

func additionalAsserts(options []hurlOption) []string {
	var asserts []string
	for _, o := range options {
		if o.name == "retry" {
			asserts = append(asserts, "status < 500")
			break
		}
	}
	return asserts
}

func body(m *matches) *string {
	if v, ok := m.get("data_raw"); ok {
		s := "```\n" + v + "\n```"
		return &s
	}
	v, ok := m.get("data")
	if !ok {
		return nil
	}
	var s string
	if filename, ok := strings.CutPrefix(v, "@"); ok {
		s = "file, " + filename + ";"
	} else {
		s = "```\n" + v + "\n```"
	}
	return &s
}

func method(m *matches) string {
	if v, ok := m.get("method"); ok {
		return v
	}
	if m.has("data_raw") || m.has("data") {
		return "POST"
	}
	return "GET"
}

func urlFrom(m *matches) (string, error) {
	s, ok := m.get("url")
	if !ok {
		if s, ok = m.get("url_param"); !ok {
			return "", errors.New("URL is missing")
		}
	}
	if !strings.HasPrefix(s, "http") {
		s = "https://" + s
	}
	return s, nil
}

func headers(m *matches) []string {
	headers := m.values["headers"]
	hasContentType := false
	for _, h := range headers {
		if strings.HasPrefix(h, "Content-Type") {
			hasContentType = true
		}
	}
	if !hasContentType {
		if data, ok := m.get("data"); m.has("data_raw") || (ok && !strings.HasPrefix(data, "@")) {
			headers = append(headers, "Content-Type: application/x-www-form-urlencoded")
		}
	}
	return headers
}

func options(m *matches) []hurlOption {
	var options []hurlOption
	flag := func(id, name string) {
		if m.has(id) {
			options = append(options, hurlOption{name, "true"})
		}
	}
	value := func(id, name string) {
		if v, ok := m.get(id); ok {
			options = append(options, hurlOption{name, v})
		}
	}
	flag("compressed", "compressed")
	flag("digest", "digest")
	flag("location", "location")
	flag("insecure", "insecure")
	value("max_redirects", "max-redirs")
	flag("negotiate", "negotiate")
	flag("ntlm", "ntlm")
	value("retry", "retry")
	value("user", "user")
	flag("verbose", "verbose")
	return options
}
