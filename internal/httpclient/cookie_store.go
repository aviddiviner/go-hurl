package httpclient

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/aviddiviner/go-hurl/internal/logger"
)

// CookieStore is the storage of cookies of an HTTP client.
type CookieStore struct {
	cookies []Cookie
}

func NewCookieStore() *CookieStore { return &CookieStore{} }

// AddCookie adds a cookie from a Netscape formatted string
// (http://www.cookiecentral.com/faq/#3.5).
func (s *CookieStore) AddCookie(netscape string) error {
	c, err := CookieFromNetscape(netscape)
	if err != nil {
		return err
	}
	s.cookies = append(s.cookies, c)
	return nil
}

// Cookies returns the cookies of this store. The slice must not be modified.
func (s *CookieStore) Cookies() []Cookie { return s.cookies }

func (s *CookieStore) Len() int { return len(s.cookies) }

func (s *CookieStore) IsEmpty() bool { return len(s.cookies) == 0 }

// ToNetscape formats this store using the Netscape cookie format.
func (s *CookieStore) ToNetscape() string {
	var b strings.Builder
	for _, c := range s.cookies {
		b.WriteString(c.ToNetscape())
		b.WriteByte('\n')
	}
	return b.String()
}

// Cookie is a cookie returned by a server with a Set-Cookie header, and saved
// in the cookie storage of the HTTP engine.
type Cookie struct {
	// domain is the host to which the cookie will be sent.
	domain           string
	includeSubdomain bool
	// path must exist in the requested URL for the cookie to be sent.
	path string
	// https indicates that the cookie is only sent over https.
	https bool
	// expires is the maximum lifetime of the cookie as a UNIX timestamp (0
	// for a session cookie, 1 for an expired cookie, as libcurl does).
	expires  uint64
	name     string
	value    string
	httpOnly bool
}

// NewCookie returns a cookie.
func NewCookie(domain string, includeSubdomain bool, path string, https bool, expires uint64, name, value string, httpOnly bool) Cookie {
	return Cookie{domain, includeSubdomain, path, https, expires, name, value, httpOnly}
}

func boolNetscape(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

// ToNetscape formats this cookie using the Netscape cookie format: tab
// separated domain, include subdomain flag, path, secure flag, expiration
// (UNIX time), name and value.
func (c Cookie) ToNetscape() string {
	prefix := ""
	if c.httpOnly {
		prefix = "#HttpOnly_"
	}
	return prefix + c.domain + "\t" + boolNetscape(c.includeSubdomain) + "\t" + c.path + "\t" +
		boolNetscape(c.https) + "\t" + strconv.FormatUint(c.expires, 10) + "\t" + c.name + "\t" + c.value
}

func (c Cookie) String() string { return c.ToNetscape() }

// ParseCookieError is returned when a Netscape cookie string can't be parsed.
type ParseCookieError struct{}

func (ParseCookieError) Error() string { return "can not parse cookie" }

// CookieFromNetscape parses a Netscape formatted cookie. Fields are
// separated by tabs or spaces (for inline use in Hurl files), and the last
// field (the value) can contain whitespace.
func CookieFromNetscape(s string) (Cookie, error) {
	tokens := cookieAttributes(s)
	next := func() (string, bool) {
		if len(tokens) == 0 {
			return "", false
		}
		t := tokens[0]
		tokens = tokens[1:]
		return t, true
	}
	parseBool := func() (bool, bool) {
		switch t, _ := next(); t {
		case "TRUE":
			return true, true
		case "FALSE":
			return false, true
		}
		return false, false
	}
	var c Cookie
	v, ok := next()
	if !ok {
		return Cookie{}, ParseCookieError{}
	}
	if d, ok := strings.CutPrefix(v, "#HttpOnly_"); ok {
		c.httpOnly, c.domain = true, d
	} else {
		c.domain = v
	}
	if c.includeSubdomain, ok = parseBool(); !ok {
		return Cookie{}, ParseCookieError{}
	}
	if c.path, ok = next(); !ok {
		return Cookie{}, ParseCookieError{}
	}
	if c.https, ok = parseBool(); !ok {
		return Cookie{}, ParseCookieError{}
	}
	v, ok = next()
	if !ok {
		return Cookie{}, ParseCookieError{}
	}
	expires, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return Cookie{}, ParseCookieError{}
	}
	c.expires = expires
	if c.name, ok = next(); !ok {
		return Cookie{}, ParseCookieError{}
	}
	c.value, _ = next()
	return c, nil
}

func isCookieWhitespace(b byte) bool { return b == ' ' || b == '\t' || b == '\r' || b == '\n' }

// cookieAttributes splits a Netscape line into at most 7 fields; the 7th is
// the remainder of the line.
func cookieAttributes(line string) []string {
	var fields []string
	pos := 0
	for len(fields) < 7 {
		for pos < len(line) && isCookieWhitespace(line[pos]) {
			pos++
		}
		if pos >= len(line) {
			break
		}
		if len(fields) == 6 {
			fields = append(fields, line[pos:])
			break
		}
		start := pos
		for pos < len(line) && !isCookieWhitespace(line[pos]) {
			pos++
		}
		fields = append(fields, line[start:pos])
	}
	return fields
}

func (c Cookie) Domain() string         { return c.domain }
func (c Cookie) Path() string           { return c.path }
func (c Cookie) Name() string           { return c.name }
func (c Cookie) Value() string          { return c.value }
func (c Cookie) Expires() uint64        { return c.expires }
func (c Cookie) Https() bool            { return c.https }
func (c Cookie) HttpOnly() bool         { return c.httpOnly }
func (c Cookie) IncludeSubdomain() bool { return c.includeSubdomain }

// IsExpired reports whether this cookie has been expired by the engine
// (libcurl sets the expiration to 1).
func (c Cookie) IsExpired() bool { return c.expires == 1 }

// MatchDomain reports whether this cookie matches the domain and path of
// url.
func (c Cookie) MatchDomain(url Url) bool {
	cookieDomain := strings.TrimPrefix(c.domain, ".")
	if urlDomain := url.Domain(); urlDomain != "" {
		if !c.includeSubdomain {
			if urlDomain != cookieDomain {
				return false
			}
		} else if !strings.HasSuffix(urlDomain, cookieDomain) {
			return false
		}
	}
	return strings.HasPrefix(url.Path(), c.path)
}

// CookieStore returns the cookie store of this client.
func (c *Client) CookieStore(l *logger.Logger) *CookieStore {
	store := NewCookieStore()
	for _, line := range c.jar.list() {
		if !utf8.ValidString(line) {
			line = strings.ToValidUTF8(line, "\uFFFD")
			l.Warning("Cookie <" + line + "> can not be parsed as UTF-8")
		}
		if store.AddCookie(line) != nil {
			l.Warning("Line <" + line + "> can not be parsed as cookie")
		}
	}
	return store
}

// AddCookie adds a cookie to the cookie jar (experimental).
func (c *Client) AddCookie(cookie Cookie, l *logger.Logger) {
	l.Debug("Add to cookie store <" + cookie.String() + "> (experimental)")
	c.jar.addNetscape(cookie.ToNetscape())
}

// ClearCookieStorage clears the cookie storage (experimental).
func (c *Client) ClearCookieStorage(l *logger.Logger) {
	l.Debug("Clear cookie storage (experimental)")
	c.jar.clear()
}
