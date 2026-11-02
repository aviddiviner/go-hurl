package httpclient

import (
	"bufio"
	"errors"
	"math"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// cookieJar is the cookie engine of the client. libcurl's cookie engine
// stores and matches cookies for Hurl; this is a port of its rules
// (lib/cookie.c): domain tail matching, path matching, secure cookies,
// expiration, replacement of cookies with the same name, domain and path,
// and Netscape format import/export.
type cookieJar struct {
	mu       sync.Mutex
	cookies  []*jarCookie
	creation int
	now      func() time.Time
}

type jarCookie struct {
	// domain is stored without a leading dot.
	domain    string
	tailmatch bool
	// path is the path as stored (and exported); spath is the sanitized
	// path used for matching.
	path     string
	spath    string
	secure   bool
	expires  int64
	name     string
	value    string
	httpOnly bool
	// live is false for cookies loaded from a cookie file: live cookies are
	// preferred over file cookies.
	live     bool
	creation int
}

func newCookieJar() *cookieJar { return &cookieJar{now: time.Now} }

// netscape formats a cookie like libcurl's get_netscape_format.
func (c *jarCookie) netscape() string {
	var b strings.Builder
	if c.httpOnly {
		b.WriteString("#HttpOnly_")
	}
	if c.tailmatch && !strings.HasPrefix(c.domain, ".") {
		b.WriteByte('.')
	}
	b.WriteString(c.domain)
	b.WriteString("\t" + boolNetscape(c.tailmatch) + "\t")
	if c.path == "" {
		b.WriteString("/")
	} else {
		b.WriteString(c.path)
	}
	b.WriteString("\t" + boolNetscape(c.secure) + "\t" + strconv.FormatInt(c.expires, 10))
	b.WriteString("\t" + c.name + "\t" + c.value)
	return b.String()
}

func (j *cookieJar) list() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	var lines []string
	for _, c := range j.cookies {
		lines = append(lines, c.netscape())
	}
	return lines
}

func (j *cookieJar) clear() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.cookies = nil
}

func (j *cookieJar) removeExpired() {
	now := j.now().Unix()
	j.cookies = slices.DeleteFunc(j.cookies, func(c *jarCookie) bool {
		return c.expires != 0 && c.expires < now
	})
}

// add adds or replaces a cookie, like libcurl's Curl_cookie_add.
func (j *cookieJar) add(co *jarCookie, noexpire bool) {
	if !noexpire {
		j.removeExpired()
	}
	for i, old := range j.cookies {
		if !strings.EqualFold(old.name, co.name) {
			continue
		}
		if !strings.EqualFold(old.domain, co.domain) || old.tailmatch != co.tailmatch {
			continue
		}
		if !strings.EqualFold(old.spath, co.spath) {
			continue
		}
		if !co.live && old.live {
			return
		}
		co.creation = old.creation
		j.cookies[i] = co
		return
	}
	j.creation++
	co.creation = j.creation
	j.cookies = append(j.cookies, co)
}

// addNetscape adds a cookie from a Netscape line (CURLOPT_COOKIELIST).
func (j *cookieJar) addNetscape(line string) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if co := parseNetscapeCookie(line); co != nil {
		co.live = true
		j.add(co, true)
	}
}

// loadFile loads a Netscape cookie file. A missing file is ignored, as
// libcurl does.
func (j *cookieJar) loadFile(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	j.mu.Lock()
	defer j.mu.Unlock()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		if co := parseNetscapeCookie(scanner.Text()); co != nil {
			j.add(co, false)
		}
	}
}

func parseNetscapeCookie(line string) *jarCookie {
	line = strings.TrimRight(line, "\r\n")
	if strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#HttpOnly_") {
		return nil
	}
	c, err := CookieFromNetscape(line)
	if err != nil {
		return nil
	}
	expires := int64(min(c.expires, math.MaxInt64))
	return &jarCookie{
		domain:    strings.TrimPrefix(c.domain, "."),
		tailmatch: c.includeSubdomain,
		path:      c.path,
		spath:     sanitizeCookiePath(c.path),
		secure:    c.https,
		expires:   expires,
		name:      c.name,
		value:     c.value,
		httpOnly:  c.httpOnly,
	}
}

// sanitizeCookiePath removes surrounding quotes and a trailing slash, and
// defaults to "/" for a path not starting with "/".
func sanitizeCookiePath(path string) string {
	path = strings.TrimPrefix(path, "\"")
	path = strings.TrimSuffix(path, "\"")
	if !strings.HasPrefix(path, "/") {
		return "/"
	}
	if len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

// isSecureContext reports whether cookies marked Secure can be used: for
// https, and for localhost like libcurl.
func isSecureContext(scheme, host string) bool {
	return scheme == "https" || strings.EqualFold(host, "localhost") || host == "127.0.0.1" || host == "::1"
}

func isIPHost(host string) bool { return net.ParseIP(host) != nil }

// badDomain reports whether a Domain attribute is too broad: it must
// contain a dot that is not the last char, except for "localhost".
func badDomain(domain string) bool {
	if strings.EqualFold(domain, "localhost") {
		return false
	}
	i := strings.IndexByte(domain, '.')
	return i < 0 || i == len(domain)-1
}

// tailMatch reports whether hostname is cookieDomain or a subdomain of it.
func tailMatch(cookieDomain, hostname string) bool {
	if len(hostname) < len(cookieDomain) || !strings.EqualFold(hostname[len(hostname)-len(cookieDomain):], cookieDomain) {
		return false
	}
	return len(hostname) == len(cookieDomain) || hostname[len(hostname)-len(cookieDomain)-1] == '.'
}

// invalidOctets reports whether s contains control chars (except tab).
func invalidOctets(s string) bool {
	for i := range len(s) {
		b := s[i]
		if (b < 0x20 && b != '\t') || b == 0x7f {
			return true
		}
	}
	return false
}

// setCookie parses a Set-Cookie header received for host and request path,
// and stores it.
func (j *cookieJar) setCookie(header, host, reqPath string, secure bool) {
	co := j.parseSetCookie(header, host, reqPath, secure)
	if co == nil {
		return
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	j.add(co, false)
}

func (j *cookieJar) parseSetCookie(header, host, reqPath string, secure bool) *jarCookie {
	parts := strings.Split(header, ";")
	name, value, ok := strings.Cut(parts[0], "=")
	if !ok {
		return nil
	}
	name, value = strings.TrimSpace(name), strings.TrimSpace(value)
	if name == "" || invalidOctets(name) || invalidOctets(value) {
		return nil
	}
	co := &jarCookie{name: name, value: value, live: true}
	now := j.now().Unix()
	var domainAttr *string
	pathSet, maxAgeSet := false, false
	for _, p := range parts[1:] {
		k, v, _ := strings.Cut(p, "=")
		k, v = strings.TrimSpace(k), strings.TrimSpace(v)
		switch strings.ToLower(k) {
		case "secure":
			if !secure {
				return nil
			}
			co.secure = true
		case "httponly":
			co.httpOnly = true
		case "path":
			co.path = v
			co.spath = sanitizeCookiePath(v)
			pathSet = true
		case "domain":
			if v == "" {
				continue
			}
			v = strings.TrimPrefix(v, ".")
			domainAttr = &v
		case "max-age":
			maxAgeSet = true
			v = strings.Trim(v, "\"")
			n, err := strconv.ParseInt(v, 10, 64)
			switch {
			case errors.Is(err, strconv.ErrRange) && n > 0:
				co.expires = math.MaxInt64
			case err != nil || n <= 0:
				co.expires = 1
			case math.MaxInt64-now < n:
				co.expires = math.MaxInt64
			default:
				co.expires = now + n
			}
		case "expires":
			if maxAgeSet || v == "" {
				continue
			}
			t, ok := parseCookieDate(v)
			switch {
			case !ok:
				co.expires = 0
			case t.Unix() == 0:
				co.expires = 1
			case t.Unix() < 0:
				co.expires = 0
			default:
				co.expires = t.Unix()
			}
		}
	}

	if domainAttr != nil {
		d := *domainAttr
		ip := isIPHost(host)
		switch {
		case ip && d == host:
			co.domain = d
		case !ip && !badDomain(d) && tailMatch(d, host):
			co.domain = d
			co.tailmatch = true
		default:
			return nil
		}
	} else {
		co.domain = host
	}

	if !pathSet {
		endslash := strings.LastIndexByte(reqPath, '/')
		if endslash < 0 {
			co.path = "/"
		} else {
			co.path = reqPath[:endslash+1]
		}
		co.spath = sanitizeCookiePath(co.path)
	}

	// Cookie prefixes (RFC 6265bis).
	if strings.HasPrefix(name, "__Secure-") && !co.secure {
		return nil
	}
	if strings.HasPrefix(name, "__Host-") && (!co.secure || domainAttr != nil || co.spath != "/") {
		return nil
	}
	return co
}

// pathMatch implements RFC 6265 5.1.4 path matching, case sensitive.
func pathMatch(cookiePath, uriPath string) bool {
	if len(cookiePath) == 1 {
		return true
	}
	if uriPath == "" || uriPath[0] != '/' {
		uriPath = "/"
	}
	if !strings.HasPrefix(uriPath, cookiePath) {
		return false
	}
	return len(cookiePath) == len(uriPath) || uriPath[len(cookiePath)] == '/'
}

// cookiesFor returns the cookies to send to host for reqPath, sorted like
// libcurl: longest path first, then longest domain, then name, then
// creation order.
func (j *cookieJar) cookiesFor(host, reqPath string, secure bool) []*jarCookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.removeExpired()
	ip := isIPHost(host)
	var matches []*jarCookie
	for _, c := range j.cookies {
		if c.secure && !secure {
			continue
		}
		var domainOK bool
		if c.tailmatch && !ip {
			domainOK = tailMatch(c.domain, host)
		} else {
			domainOK = strings.EqualFold(c.domain, host)
		}
		if !domainOK || !pathMatch(c.spath, reqPath) {
			continue
		}
		matches = append(matches, c)
	}
	slices.SortStableFunc(matches, func(a, b *jarCookie) int {
		if d := len(b.spath) - len(a.spath); d != 0 {
			return d
		}
		if d := len(b.domain) - len(a.domain); d != 0 {
			return d
		}
		if d := strings.Compare(a.name, b.name); d != 0 {
			return d
		}
		return a.creation - b.creation
	})
	return matches
}

var cookieMonths = map[string]time.Month{
	"jan": time.January, "feb": time.February, "mar": time.March, "apr": time.April,
	"may": time.May, "jun": time.June, "jul": time.July, "aug": time.August,
	"sep": time.September, "oct": time.October, "nov": time.November, "dec": time.December,
}

// parseCookieDate is a lenient date parser in the spirit of curl_getdate:
// it finds a time (hh:mm:ss), a month name, a day and a year in any order,
// so RFC 1123, RFC 850 ("13-Jan-21") and asctime formats are accepted.
func parseCookieDate(s string) (time.Time, bool) {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !(r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == ':')
	})
	hour, minute, sec := -1, -1, -1
	day, year := -1, -1
	month := time.Month(0)
	for _, f := range fields {
		switch {
		case strings.Contains(f, ":") && hour < 0:
			t := strings.Split(f, ":")
			if len(t) < 2 || len(t) > 3 {
				return time.Time{}, false
			}
			var nums [3]int
			for i, p := range t {
				n, err := strconv.Atoi(p)
				if err != nil {
					return time.Time{}, false
				}
				nums[i] = n
			}
			hour, minute, sec = nums[0], nums[1], nums[2]
		case len(f) >= 3 && month == 0 && cookieMonths[strings.ToLower(f[:3])] != 0 && !isDigits(f):
			month = cookieMonths[strings.ToLower(f[:3])]
		case isDigits(f):
			n, _ := strconv.Atoi(f)
			switch {
			case day < 0 && len(f) <= 2 && n >= 1 && n <= 31:
				day = n
			case year < 0 && len(f) == 4:
				year = n
			case year < 0 && len(f) == 2:
				if n < 70 {
					year = 2000 + n
				} else {
					year = 1900 + n
				}
			}
		}
	}
	if day < 0 || month == 0 || year < 0 {
		return time.Time{}, false
	}
	if hour < 0 {
		hour, minute, sec = 0, 0, 0
	}
	if hour > 23 || minute > 59 || sec > 60 {
		return time.Time{}, false
	}
	return time.Date(year, month, day, hour, minute, sec, 0, time.UTC), true
}

func isDigits(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}
