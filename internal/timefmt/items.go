// Package timefmt formats and parses dates with chrono's strftime syntax
// (chrono 0.4.45, as used by Hurl's dateFormat, format and toDate filters).
//
// Format is chrono's DateTime::format. ParseDateTime, ParseNaiveDateTime
// and ParseNaiveDate are DateTime::parse_from_str,
// NaiveDateTime::parse_from_str and NaiveDate::parse_from_str. ParseRFC3339
// is DateTime::parse_from_rfc3339, and Display is DateTime's Display.
//
// Every chrono specifier is supported, with the default (non-localized)
// names. Like chrono, an unknown specifier makes the whole format invalid.
//
// Go's time.Time can't represent leap seconds: chrono's second 60 (parsed
// from "%S" or RFC 3339 input) rolls over to the next minute.
package timefmt

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type pad int

const (
	padNone pad = iota
	padZero
	padSpace
)

type numeric int

const (
	numYear numeric = iota
	numYearDiv100
	numYearMod100
	numIsoYear
	numIsoYearDiv100
	numIsoYearMod100
	numQuarter
	numMonth
	numDay
	numWeekFromSun
	numWeekFromMon
	numIsoWeek
	numNumDaysFromSun
	numWeekdayFromMon
	numOrdinal
	numHour
	numHour12
	numMinute
	numSecond
	numNanosecond
	numTimestamp
)

type fixed int

const (
	fixShortMonthName fixed = iota
	fixLongMonthName
	fixShortWeekdayName
	fixLongWeekdayName
	fixLowerAmPm
	fixUpperAmPm
	fixNanosecond
	fixNanosecond3
	fixNanosecond6
	fixNanosecond9
	fixNanosecond3NoDot
	fixNanosecond6NoDot
	fixNanosecond9NoDot
	fixTimezoneName
	fixTimezoneOffset
	fixTimezoneOffsetColon
	fixTimezoneOffsetDoubleColon
	fixTimezoneOffsetTripleColon
	fixTimezoneOffsetPermissive
	fixRFC3339
)

type itemKind int

const (
	itemLiteral itemKind = iota
	itemSpace
	itemNumeric
	itemFixed
	itemError
)

type item struct {
	kind itemKind
	s    string
	num  numeric
	pad  pad
	fix  fixed
}

func lit(s string) item          { return item{kind: itemLiteral, s: s} }
func space(s string) item        { return item{kind: itemSpace, s: s} }
func num(n numeric) item         { return item{kind: itemNumeric, num: n, pad: padNone} }
func num0(n numeric) item        { return item{kind: itemNumeric, num: n, pad: padZero} }
func nums(n numeric) item        { return item{kind: itemNumeric, num: n, pad: padSpace} }
func fix(f fixed) item           { return item{kind: itemFixed, fix: f} }
func errItem() item              { return item{kind: itemError} }
func isSpace(r rune) bool        { return unicode.IsSpace(r) }
func isNotSpace(r rune) bool     { return !unicode.IsSpace(r) }
func trimSpace(s string) string  { return strings.TrimLeftFunc(s, isSpace) }
func hasPrefix(s, p string) bool { return strings.HasPrefix(s, p) }

func lenOr(i int, s string) int {
	if i < 0 {
		return len(s)
	}
	return i
}

var (
	dFmt     = []item{num0(numMonth), lit("/"), num0(numDay), lit("/"), num0(numYearMod100)}
	dTFmt    = []item{fix(fixShortWeekdayName), space(" "), fix(fixShortMonthName), space(" "), nums(numDay), space(" "), num0(numHour), lit(":"), num0(numMinute), lit(":"), num0(numSecond), space(" "), num0(numYear)}
	tFmt     = []item{num0(numHour), lit(":"), num0(numMinute), lit(":"), num0(numSecond)}
	tFmtAmPm = []item{num0(numHour12), lit(":"), num0(numMinute), lit(":"), num0(numSecond), space(" "), fix(fixUpperAmPm)}
)

// parseItems splits a strftime format string into items, like chrono's
// StrftimeItems (non-lenient, without locales).
func parseItems(s string) []item {
	var items []item
	for s != "" {
		var next []item
		s, next = parseNextItem(s)
		items = append(items, next...)
	}
	return items
}

// parseNextItem returns the remainder of s and the item(s) for the next
// specifier, space run or literal.
func parseNextItem(s string) (string, []item) {
	c, _ := utf8.DecodeRuneInString(s)
	if c != '%' {
		if isSpace(c) {
			n := lenOr(strings.IndexFunc(s, isNotSpace), s)
			return s[n:], []item{space(s[:n])}
		}
		n := lenOr(strings.IndexFunc(s, func(r rune) bool { return isSpace(r) || r == '%' }), s)
		return s[n:], []item{lit(s[:n])}
	}
	rem := s[1:]
	next := func() (rune, bool) {
		if rem == "" {
			return 0, false
		}
		r, size := utf8.DecodeRuneInString(rem)
		rem = rem[size:]
		return r, true
	}
	fail := func() (string, []item) { return rem, []item{errItem()} }

	spec, ok := next()
	if !ok {
		return fail()
	}
	var padOverride *pad
	switch spec {
	case '-':
		p := padNone
		padOverride = &p
	case '0':
		p := padZero
		padOverride = &p
	case '_':
		p := padSpace
		padOverride = &p
	}
	isAlternate := spec == '#'
	if padOverride != nil || isAlternate {
		if spec, ok = next(); !ok {
			return fail()
		}
	}
	if isAlternate && spec != 'z' {
		return fail()
	}

	var items []item
	one := func(it item) { items = []item{it} }
	switch spec {
	case 'A':
		one(fix(fixLongWeekdayName))
	case 'B':
		one(fix(fixLongMonthName))
	case 'C':
		one(num0(numYearDiv100))
	case 'D':
		items = []item{num0(numMonth), lit("/"), num0(numDay), lit("/"), num0(numYearMod100)}
	case 'F':
		items = []item{num0(numYear), lit("-"), num0(numMonth), lit("-"), num0(numDay)}
	case 'G':
		one(num0(numIsoYear))
	case 'H':
		one(num0(numHour))
	case 'I':
		one(num0(numHour12))
	case 'M':
		one(num0(numMinute))
	case 'P':
		one(fix(fixLowerAmPm))
	case 'R':
		items = []item{num0(numHour), lit(":"), num0(numMinute)}
	case 'S':
		one(num0(numSecond))
	case 'T':
		items = []item{num0(numHour), lit(":"), num0(numMinute), lit(":"), num0(numSecond)}
	case 'U':
		one(num0(numWeekFromSun))
	case 'V':
		one(num0(numIsoWeek))
	case 'W':
		one(num0(numWeekFromMon))
	case 'X':
		items = tFmt
	case 'Y':
		one(num0(numYear))
	case 'Z':
		one(fix(fixTimezoneName))
	case 'a':
		one(fix(fixShortWeekdayName))
	case 'b', 'h':
		one(fix(fixShortMonthName))
	case 'c':
		items = dTFmt
	case 'd':
		one(num0(numDay))
	case 'e':
		one(nums(numDay))
	case 'f':
		one(num0(numNanosecond))
	case 'g':
		one(num0(numIsoYearMod100))
	case 'j':
		one(num0(numOrdinal))
	case 'k':
		one(nums(numHour))
	case 'l':
		one(nums(numHour12))
	case 'm':
		one(num0(numMonth))
	case 'n':
		one(space("\n"))
	case 'p':
		one(fix(fixUpperAmPm))
	case 'q':
		one(num(numQuarter))
	case 'r':
		items = tFmtAmPm
	case 's':
		one(num(numTimestamp))
	case 't':
		one(space("\t"))
	case 'u':
		one(num(numWeekdayFromMon))
	case 'v':
		items = []item{nums(numDay), lit("-"), fix(fixShortMonthName), lit("-"), num0(numYear)}
	case 'w':
		one(num(numNumDaysFromSun))
	case 'x':
		items = dFmt
	case 'y':
		one(num0(numYearMod100))
	case 'z':
		if isAlternate {
			one(fix(fixTimezoneOffsetPermissive))
		} else {
			one(fix(fixTimezoneOffset))
		}
	case '+':
		one(fix(fixRFC3339))
	case ':':
		switch {
		case hasPrefix(rem, "::z"):
			rem = rem[3:]
			one(fix(fixTimezoneOffsetTripleColon))
		case hasPrefix(rem, ":z"):
			rem = rem[2:]
			one(fix(fixTimezoneOffsetDoubleColon))
		case hasPrefix(rem, "z"):
			rem = rem[1:]
			one(fix(fixTimezoneOffsetColon))
		default:
			one(errItem())
		}
	case '.':
		d, ok := next()
		if !ok {
			return fail()
		}
		switch d {
		case '3', '6', '9':
			f, ok := next()
			if !ok {
				return fail()
			}
			if f != 'f' {
				one(errItem())
				break
			}
			one(fix(map[rune]fixed{'3': fixNanosecond3, '6': fixNanosecond6, '9': fixNanosecond9}[d]))
		case 'f':
			one(fix(fixNanosecond))
		default:
			one(errItem())
		}
	case '3', '6', '9':
		f, ok := next()
		if !ok {
			return fail()
		}
		if f != 'f' {
			one(errItem())
			break
		}
		one(fix(map[rune]fixed{'3': fixNanosecond3NoDot, '6': fixNanosecond6NoDot, '9': fixNanosecond9NoDot}[spec]))
	case '%':
		one(lit("%"))
	default:
		one(errItem())
	}

	if padOverride != nil {
		if len(items) == 1 && items[0].kind == itemNumeric {
			it := items[0]
			it.pad = *padOverride
			return rem, []item{it}
		}
		return fail()
	}
	return rem, items
}
