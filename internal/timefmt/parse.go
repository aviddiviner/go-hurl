package timefmt

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// ParseError is a chrono parse error kind.
type ParseError int

const (
	ErrOutOfRange ParseError = iota
	ErrImpossible
	ErrNotEnough
	ErrInvalid
	ErrTooShort
	ErrTooLong
	ErrBadFormat
)

func (e ParseError) Error() string {
	switch e {
	case ErrOutOfRange:
		return "input is out of range"
	case ErrImpossible:
		return "no possible date and time matching input"
	case ErrNotEnough:
		return "input is not enough for unique date and time"
	case ErrInvalid:
		return "input contains invalid characters"
	case ErrTooShort:
		return "premature end of input"
	case ErrTooLong:
		return "trailing input"
	case ErrBadFormat:
		return "bad or unsupported format string"
	}
	return ""
}

const (
	minYear = -262143
	maxYear = 262142
)

type opt[T comparable] struct {
	v   T
	set bool
}

func some[T comparable](v T) opt[T] { return opt[T]{v, true} }

func (o *opt[T]) setConsistent(v T) error {
	if o.set && o.v != v {
		return ErrImpossible
	}
	o.v, o.set = v, true
	return nil
}

func (o opt[T]) or(d T) T {
	if o.set {
		return o.v
	}
	return d
}

// parsed is chrono's Parsed.
type parsed struct {
	year, yearDiv100, yearMod100          opt[int64]
	isoYear, isoYearDiv100, isoYearMod100 opt[int64]
	quarter, month                        opt[int64]
	weekFromSun, weekFromMon, isoWeek     opt[int64]
	weekday                               opt[time.Weekday]
	ordinal, day                          opt[int64]
	hourDiv12, hourMod12                  opt[int64]
	minute, second, nanosecond            opt[int64]
	timestamp                             opt[int64]
	offset                                opt[int64]
}

func inRange(v, lo, hi int64) bool { return v >= lo && v <= hi }

func setRange(o *opt[int64], v, lo, hi int64) error {
	if !inRange(v, lo, hi) {
		return ErrOutOfRange
	}
	return o.setConsistent(v)
}

func (p *parsed) setNumeric(n numeric, v int64) error {
	switch n {
	case numYear:
		return setRange(&p.year, v, math.MinInt32, math.MaxInt32)
	case numYearDiv100:
		return setRange(&p.yearDiv100, v, 0, math.MaxInt32)
	case numYearMod100:
		return setRange(&p.yearMod100, v, 0, 99)
	case numIsoYear:
		return setRange(&p.isoYear, v, math.MinInt32, math.MaxInt32)
	case numIsoYearDiv100:
		return setRange(&p.isoYearDiv100, v, 0, math.MaxInt32)
	case numIsoYearMod100:
		return setRange(&p.isoYearMod100, v, 0, 99)
	case numQuarter:
		return setRange(&p.quarter, v, 1, 4)
	case numMonth:
		return setRange(&p.month, v, 1, 12)
	case numDay:
		return setRange(&p.day, v, 1, 31)
	case numWeekFromSun:
		return setRange(&p.weekFromSun, v, 0, 53)
	case numWeekFromMon:
		return setRange(&p.weekFromMon, v, 0, 53)
	case numIsoWeek:
		return setRange(&p.isoWeek, v, 1, 53)
	case numNumDaysFromSun:
		if !inRange(v, 0, 6) {
			return ErrOutOfRange
		}
		return p.weekday.setConsistent(time.Weekday(v))
	case numWeekdayFromMon:
		if !inRange(v, 1, 7) {
			return ErrOutOfRange
		}
		return p.weekday.setConsistent(time.Weekday(v % 7))
	case numOrdinal:
		return setRange(&p.ordinal, v, 1, 366)
	case numHour:
		return p.setHour(v)
	case numHour12:
		if !inRange(v, 1, 12) {
			return ErrOutOfRange
		}
		if v == 12 {
			v = 0
		}
		return p.hourMod12.setConsistent(v)
	case numMinute:
		return setRange(&p.minute, v, 0, 59)
	case numSecond:
		return setRange(&p.second, v, 0, 60)
	case numNanosecond:
		return setRange(&p.nanosecond, v, 0, 999_999_999)
	case numTimestamp:
		return p.timestamp.setConsistent(v)
	}
	return ErrBadFormat
}

func (p *parsed) setHour(v int64) error {
	if !inRange(v, 0, 23) {
		return ErrOutOfRange
	}
	if err := p.hourDiv12.setConsistent(v / 12); err != nil {
		return err
	}
	return p.hourMod12.setConsistent(v % 12)
}

func (p *parsed) setOffset(v int64) error {
	return setRange(&p.offset, v, math.MinInt32, math.MaxInt32)
}

// ParseDateTime parses s like chrono's DateTime::<FixedOffset>::parse_from_str:
// an offset (or a timestamp) is required. The result has a fixed zone.
func ParseDateTime(s, layout string) (time.Time, error) {
	var p parsed
	if err := parse(&p, s, parseItems(layout)); err != nil {
		return time.Time{}, err
	}
	return p.toDateTime()
}

// ParseNaiveDateTime parses s like chrono's NaiveDateTime::parse_from_str: any
// parsed offset is ignored. The result is in UTC.
func ParseNaiveDateTime(s, layout string) (time.Time, error) {
	var p parsed
	if err := parse(&p, s, parseItems(layout)); err != nil {
		return time.Time{}, err
	}
	d, t, err := p.toNaiveDateTimeWithOffset(0)
	if err != nil {
		return time.Time{}, err
	}
	return combine(d, t, time.UTC), nil
}

// ParseNaiveDate parses s like chrono's NaiveDate::parse_from_str. The result
// is midnight UTC.
func ParseNaiveDate(s, layout string) (time.Time, error) {
	var p parsed
	if err := parse(&p, s, parseItems(layout)); err != nil {
		return time.Time{}, err
	}
	return p.toNaiveDate()
}

func parse(p *parsed, s string, items []item) error {
	rest, err := parseInternal(p, s, items)
	if err != nil {
		return err
	}
	if rest != "" {
		return ErrTooLong
	}
	return nil
}

func numericWidth(n numeric) (width int, signed bool) {
	switch n {
	case numYear, numIsoYear:
		return 4, true
	case numQuarter, numNumDaysFromSun, numWeekdayFromMon:
		return 1, false
	case numOrdinal:
		return 3, false
	case numNanosecond:
		return 9, false
	case numTimestamp:
		return math.MaxInt, false
	}
	return 2, false
}

func parseInternal(p *parsed, s string, items []item) (string, error) {
	for _, it := range items {
		var err error
		switch it.kind {
		case itemLiteral:
			if len(s) < len(it.s) {
				return "", ErrTooShort
			}
			if !strings.HasPrefix(s, it.s) {
				return "", ErrInvalid
			}
			s = s[len(it.s):]
		case itemSpace:
			s = trimSpace(s)
		case itemNumeric:
			width, signed := numericWidth(it.num)
			s = trimSpace(s)
			var v int64
			switch {
			case signed && strings.HasPrefix(s, "-"):
				if s, v, err = scanNumber(s[1:], 1, math.MaxInt); err != nil {
					return "", err
				}
				v = -v
			case signed && strings.HasPrefix(s, "+"):
				if s, v, err = scanNumber(s[1:], 1, math.MaxInt); err != nil {
					return "", err
				}
			default:
				if s, v, err = scanNumber(s, 1, width); err != nil {
					return "", err
				}
			}
			if err = p.setNumeric(it.num, v); err != nil {
				return "", err
			}
		case itemFixed:
			if s, err = parseFixed(p, s, it.fix); err != nil {
				return "", err
			}
		case itemError:
			return "", ErrBadFormat
		}
	}
	return s, nil
}

func parseFixed(p *parsed, s string, f fixed) (string, error) {
	var err error
	switch f {
	case fixShortMonthName, fixLongMonthName:
		var m int
		if f == fixShortMonthName {
			s, m, err = scanShortMonth0(s)
		} else {
			s, m, err = scanShortOrLongMonth0(s)
		}
		if err != nil {
			return "", err
		}
		return s, p.month.setConsistent(int64(m) + 1)
	case fixShortWeekdayName, fixLongWeekdayName:
		var wd time.Weekday
		if f == fixShortWeekdayName {
			s, wd, err = scanShortWeekday(s)
		} else {
			s, wd, err = scanShortOrLongWeekday(s)
		}
		if err != nil {
			return "", err
		}
		return s, p.weekday.setConsistent(wd)
	case fixLowerAmPm, fixUpperAmPm:
		if len(s) < 2 {
			return "", ErrTooShort
		}
		var pm int64
		switch [2]byte{s[0] | 32, s[1] | 32} {
		case [2]byte{'a', 'm'}:
		case [2]byte{'p', 'm'}:
			pm = 1
		default:
			return "", ErrInvalid
		}
		return s[2:], p.hourDiv12.setConsistent(pm)
	case fixNanosecond:
		if strings.HasPrefix(s, ".") {
			var nano int64
			if s, nano, err = scanNanosecond(s[1:]); err != nil {
				return "", err
			}
			return s, setRange(&p.nanosecond, nano, 0, 999_999_999)
		}
		return s, nil
	case fixNanosecond3, fixNanosecond6, fixNanosecond9:
		if strings.HasPrefix(s, ".") {
			digits := map[fixed]int{fixNanosecond3: 3, fixNanosecond6: 6, fixNanosecond9: 9}[f]
			var nano int64
			if s, nano, err = scanNanosecondFixed(s[1:], digits); err != nil {
				return "", err
			}
			return s, setRange(&p.nanosecond, nano, 0, 999_999_999)
		}
		return s, nil
	case fixNanosecond3NoDot, fixNanosecond6NoDot, fixNanosecond9NoDot:
		digits := map[fixed]int{fixNanosecond3NoDot: 3, fixNanosecond6NoDot: 6, fixNanosecond9NoDot: 9}[f]
		if len(s) < digits {
			return "", ErrTooShort
		}
		var nano int64
		if s, nano, err = scanNanosecondFixed(s, digits); err != nil {
			return "", err
		}
		return s, setRange(&p.nanosecond, nano, 0, 999_999_999)
	case fixTimezoneName:
		return strings.TrimLeftFunc(s, isNotSpace), nil
	case fixTimezoneOffset, fixTimezoneOffsetColon, fixTimezoneOffsetDoubleColon, fixTimezoneOffsetTripleColon, fixTimezoneOffsetPermissive:
		permissive := f == fixTimezoneOffsetPermissive
		var off int
		if s, off, err = scanTimezoneOffset(trimSpace(s), colonOrSpace, permissive, permissive, true); err != nil {
			return "", err
		}
		return s, p.setOffset(int64(off))
	case fixRFC3339:
		return parseRFC3339Relaxed(p, s)
	}
	return "", ErrBadFormat
}

var (
	rfc3339DateItems = []item{num0(numYear), space(""), lit("-"), num0(numMonth), space(""), lit("-"), num0(numDay)}
	rfc3339TimeItems = []item{num0(numHour), space(""), lit(":"), num0(numMinute), space(""), lit(":"), num0(numSecond), fix(fixNanosecond), space("")}
)

func parseRFC3339Relaxed(p *parsed, s string) (string, error) {
	s, err := parseInternal(p, s, rfc3339DateItems)
	if err != nil {
		return "", err
	}
	if s == "" {
		return "", ErrTooShort
	}
	switch s[0] {
	case 't', 'T', ' ':
		s = s[1:]
	default:
		return "", ErrInvalid
	}
	if s, err = parseInternal(p, s, rfc3339TimeItems); err != nil {
		return "", err
	}
	s = trimSpace(s)
	var off int
	if len(s) >= 3 && strings.EqualFold(s[:3], "UTC") {
		s = s[3:]
	} else if s, off, err = scanTimezoneOffset(s, colonOrSpace, true, false, true); err != nil {
		return "", err
	}
	return s, p.setOffset(int64(off))
}

// ParseRFC3339 parses s like chrono's DateTime::parse_from_rfc3339.
func ParseRFC3339(s string) (time.Time, error) {
	if len(s) < 19 {
		return time.Time{}, ErrTooShort
	}
	digit := func(i int) (int, error) {
		if c := s[i]; c >= '0' && c <= '9' {
			return int(c - '0'), nil
		}
		return 0, ErrInvalid
	}
	digits := func(idx ...int) (int, error) {
		n := 0
		for _, i := range idx {
			d, err := digit(i)
			if err != nil {
				return 0, err
			}
			n = n*10 + d
		}
		return n, nil
	}
	year, err := digits(0, 1, 2, 3)
	if err != nil {
		return time.Time{}, err
	}
	if s[4] != '-' {
		return time.Time{}, ErrInvalid
	}
	month, err := digits(5, 6)
	if err != nil {
		return time.Time{}, err
	}
	if s[7] != '-' {
		return time.Time{}, ErrInvalid
	}
	day, err := digits(8, 9)
	if err != nil {
		return time.Time{}, err
	}
	date, ok := fromYMD(year, month, day)
	if !ok {
		return time.Time{}, ErrOutOfRange
	}
	if c := s[10]; c != 't' && c != 'T' && c != ' ' {
		return time.Time{}, ErrInvalid
	}
	hour, err := digits(11, 12)
	if err != nil {
		return time.Time{}, err
	}
	if s[13] != ':' {
		return time.Time{}, ErrInvalid
	}
	minute, err := digits(14, 15)
	if err != nil {
		return time.Time{}, err
	}
	if s[16] != ':' {
		return time.Time{}, ErrInvalid
	}
	sec, err := digits(17, 18)
	if err != nil {
		return time.Time{}, err
	}
	nano := 0
	if sec == 60 {
		sec, nano = 59, 1_000_000_000
	}
	rest := s[19:]
	if strings.HasPrefix(rest, ".") {
		var n int64
		if rest, n, err = scanNanosecond(rest[1:]); err != nil {
			return time.Time{}, err
		}
		nano += int(n)
	}
	t, ok := newNaiveTime(hour, minute, sec, nano)
	if !ok {
		return time.Time{}, ErrOutOfRange
	}
	colon := func(s string) (string, error) {
		if s == "" {
			return "", ErrTooShort
		}
		if s[0] != ':' {
			return "", ErrInvalid
		}
		return s[1:], nil
	}
	rest, off, err := scanTimezoneOffset(rest, colon, true, false, true)
	if err != nil {
		return time.Time{}, err
	}
	if rest != "" {
		return time.Time{}, ErrTooLong
	}
	if off <= -86400 || off >= 86400 {
		return time.Time{}, ErrOutOfRange
	}
	return combine(date, t, time.FixedZone("", off)), nil
}

// scanNumber is chrono's scan::number.
func scanNumber(s string, lo, hi int) (string, int64, error) {
	if len(s) < lo {
		return "", 0, ErrTooShort
	}
	var n int64
	for i := 0; i < len(s) && i < hi; i++ {
		c := s[i]
		if c < '0' || c > '9' {
			if i < lo {
				return "", 0, ErrInvalid
			}
			return s[i:], n, nil
		}
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return "", 0, ErrOutOfRange
		}
		n = n*10 + d
	}
	return s[min(hi, len(s)):], n, nil
}

var nanoScale = [10]int64{0, 100_000_000, 10_000_000, 1_000_000, 100_000, 10_000, 1_000, 100, 10, 1}

func scanNanosecond(s string) (string, int64, error) {
	orig := len(s)
	s, v, err := scanNumber(s, 1, 9)
	if err != nil {
		return "", 0, err
	}
	v *= nanoScale[orig-len(s)]
	return strings.TrimLeft(s, "0123456789"), v, nil
}

func scanNanosecondFixed(s string, digits int) (string, int64, error) {
	s, v, err := scanNumber(s, digits, digits)
	if err != nil {
		return "", 0, err
	}
	return s, v * nanoScale[digits], nil
}

func lower3(s string) string { return string([]byte{s[0] | 32, s[1] | 32, s[2] | 32}) }

func scanShortMonth0(s string) (string, int, error) {
	if len(s) < 3 {
		return "", 0, ErrTooShort
	}
	l := lower3(s)
	for i, m := range shortMonths {
		if strings.ToLower(m) == l {
			return s[3:], i, nil
		}
	}
	return "", 0, ErrInvalid
}

func scanShortWeekday(s string) (string, time.Weekday, error) {
	if len(s) < 3 {
		return "", 0, ErrTooShort
	}
	l := lower3(s)
	for i, d := range shortWeekdays {
		if strings.ToLower(d) == l {
			return s[3:], time.Weekday(i), nil
		}
	}
	return "", 0, ErrInvalid
}

func trimSuffixFold(s, suffix string) string {
	if len(s) >= len(suffix) && strings.EqualFold(s[:len(suffix)], suffix) {
		return s[len(suffix):]
	}
	return s
}

func scanShortOrLongMonth0(s string) (string, int, error) {
	s, m, err := scanShortMonth0(s)
	if err != nil {
		return "", 0, err
	}
	return trimSuffixFold(s, longMonths[m][3:]), m, nil
}

func scanShortOrLongWeekday(s string) (string, time.Weekday, error) {
	s, d, err := scanShortWeekday(s)
	if err != nil {
		return "", 0, err
	}
	return trimSuffixFold(s, longWeekdays[d][3:]), d, nil
}

func colonOrSpace(s string) (string, error) {
	return strings.TrimLeftFunc(s, func(r rune) bool { return r == ':' || isSpace(r) }), nil
}

// scanTimezoneOffset is chrono's scan::timezone_offset.
func scanTimezoneOffset(s string, consumeColon func(string) (string, error), allowZulu, allowMissingMinutes, allowTzMinusSign bool) (string, int, error) {
	if allowZulu && s != "" && (s[0] == 'Z' || s[0] == 'z') {
		return s[1:], 0, nil
	}
	r, size := utf8.DecodeRuneInString(s)
	var negative bool
	switch {
	case s == "":
		return "", 0, ErrTooShort
	case r == '+':
	case r == '-':
		negative = true
	case r == '\u2212':
		if !allowTzMinusSign {
			return "", 0, ErrInvalid
		}
		negative = true
	default:
		return "", 0, ErrInvalid
	}
	s = s[size:]
	if len(s) < 2 {
		return "", 0, ErrTooShort
	}
	if !isDigit(s[0]) || !isDigit(s[1]) {
		return "", 0, ErrInvalid
	}
	hours := int(s[0]-'0')*10 + int(s[1]-'0')
	s, err := consumeColon(s[2:])
	if err != nil {
		return "", 0, err
	}
	var minutes int
	switch {
	case len(s) >= 2:
		switch {
		case s[0] >= '0' && s[0] <= '5' && isDigit(s[1]):
			minutes = int(s[0]-'0')*10 + int(s[1]-'0')
		case s[0] >= '6' && s[0] <= '9' && isDigit(s[1]):
			return "", 0, ErrOutOfRange
		default:
			return "", 0, ErrInvalid
		}
		s = s[2:]
	case allowMissingMinutes:
		if len(s) == 1 {
			return "", 0, ErrTooShort
		}
	default:
		return "", 0, ErrTooShort
	}
	seconds := hours*3600 + minutes*60
	if negative {
		seconds = -seconds
	}
	return s, seconds, nil
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
