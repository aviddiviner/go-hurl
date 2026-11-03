package timefmt

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrFormat is returned by Format when the format string is invalid (an
// unknown specifier, or a specifier that can't be formatted, like %#z).
var ErrFormat = errors.New("invalid format string")

var (
	shortMonths   = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	longMonths    = []string{"January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	shortWeekdays = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	longWeekdays  = []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
)

// Format formats t with a chrono strftime format string. The offset is t's
// zone offset; the zone name (%Z) is "UTC" for time.UTC, else the offset
// (chrono FixedOffset's Display, e.g. "+09:30").
func Format(t time.Time, layout string) (string, error) {
	var b strings.Builder
	for _, it := range parseItems(layout) {
		var ok bool
		switch it.kind {
		case itemLiteral, itemSpace:
			b.WriteString(it.s)
			ok = true
		case itemNumeric:
			ok = formatNumeric(&b, t, it.num, it.pad)
		case itemFixed:
			ok = formatFixed(&b, t, it.fix)
		}
		if !ok {
			return "", ErrFormat
		}
	}
	return b.String(), nil
}

func writeTwo(b *strings.Builder, v uint8, p pad) {
	ones := '0' + v%10
	switch tens := v / 10; {
	case tens == 0 && p == padNone:
	case tens == 0 && p == padSpace:
		b.WriteByte(' ')
	default:
		b.WriteByte('0' + tens)
	}
	b.WriteByte(ones)
}

func writeN(b *strings.Builder, n int, v int64, p pad, alwaysSign bool) {
	if alwaysSign {
		switch p {
		case padNone:
			fmt.Fprintf(b, "%+d", v)
		case padZero:
			fmt.Fprintf(b, "%+0*d", n+1, v)
		case padSpace:
			fmt.Fprintf(b, "%+*d", n+1, v)
		}
		return
	}
	switch p {
	case padNone:
		fmt.Fprintf(b, "%d", v)
	case padZero:
		fmt.Fprintf(b, "%0*d", n, v)
	case padSpace:
		fmt.Fprintf(b, "%*d", n, v)
	}
}

func writeYear(b *strings.Builder, year int, p pad) {
	if year >= 1000 && year <= 9999 {
		fmt.Fprintf(b, "%04d", year)
		return
	}
	writeN(b, 4, int64(year), p, year < 0 || year >= 10000)
}

func writeHundreds(b *strings.Builder, n int) bool {
	if n < 0 || n >= 100 {
		return false
	}
	fmt.Fprintf(b, "%02d", n)
	return true
}

func divEuclid(a, b int) int {
	q := a / b
	if a%b < 0 {
		q--
	}
	return q
}

func remEuclid(a, b int) int {
	r := a % b
	if r < 0 {
		r += b
	}
	return r
}

// daysSince returns the number of days from weekday b to weekday a.
func daysSince(a, b time.Weekday) int { return (int(a) - int(b) + 7) % 7 }

func weeksFrom(t time.Time, day time.Weekday) int {
	return (t.YearDay() - daysSince(t.Weekday(), day) + 6) / 7
}

func hour12(h int) int {
	h %= 12
	if h == 0 {
		return 12
	}
	return h
}

func formatNumeric(b *strings.Builder, t time.Time, n numeric, p pad) bool {
	isoYear, isoWeek := t.ISOWeek()
	switch n {
	case numYear:
		writeYear(b, t.Year(), p)
	case numYearDiv100:
		writeTwo(b, uint8(divEuclid(t.Year(), 100)), p)
	case numYearMod100:
		writeTwo(b, uint8(remEuclid(t.Year(), 100)), p)
	case numIsoYear:
		writeYear(b, isoYear, p)
	case numIsoYearDiv100:
		writeTwo(b, uint8(divEuclid(isoYear, 100)), p)
	case numIsoYearMod100:
		writeTwo(b, uint8(remEuclid(isoYear, 100)), p)
	case numQuarter:
		b.WriteByte('0' + byte((int(t.Month())-1)/3+1))
	case numMonth:
		writeTwo(b, uint8(t.Month()), p)
	case numDay:
		writeTwo(b, uint8(t.Day()), p)
	case numWeekFromSun:
		writeTwo(b, uint8(weeksFrom(t, time.Sunday)), p)
	case numWeekFromMon:
		writeTwo(b, uint8(weeksFrom(t, time.Monday)), p)
	case numIsoWeek:
		writeTwo(b, uint8(isoWeek), p)
	case numNumDaysFromSun:
		b.WriteByte('0' + byte(t.Weekday()))
	case numWeekdayFromMon:
		b.WriteByte('0' + byte(daysSince(t.Weekday(), time.Monday)+1))
	case numOrdinal:
		writeN(b, 3, int64(t.YearDay()), p, false)
	case numHour:
		writeTwo(b, uint8(t.Hour()), p)
	case numHour12:
		writeTwo(b, uint8(hour12(t.Hour())), p)
	case numMinute:
		writeTwo(b, uint8(t.Minute()), p)
	case numSecond:
		writeTwo(b, uint8(t.Second()), p)
	case numNanosecond:
		writeN(b, 9, int64(t.Nanosecond()), p, false)
	case numTimestamp:
		writeN(b, 9, t.Unix(), p, false)
	default:
		return false
	}
	return true
}

func writeNanoAuto(b *strings.Builder, nano int) {
	switch {
	case nano == 0:
	case nano%1_000_000 == 0:
		fmt.Fprintf(b, ".%03d", nano/1_000_000)
	case nano%1_000 == 0:
		fmt.Fprintf(b, ".%06d", nano/1_000)
	default:
		fmt.Fprintf(b, ".%09d", nano)
	}
}

func formatFixed(b *strings.Builder, t time.Time, f fixed) bool {
	_, off := t.Zone()
	nano := t.Nanosecond()
	switch f {
	case fixShortMonthName:
		b.WriteString(shortMonths[t.Month()-1])
	case fixLongMonthName:
		b.WriteString(longMonths[t.Month()-1])
	case fixShortWeekdayName:
		b.WriteString(shortWeekdays[t.Weekday()])
	case fixLongWeekdayName:
		b.WriteString(longWeekdays[t.Weekday()])
	case fixLowerAmPm, fixUpperAmPm:
		ampm := "AM"
		if t.Hour() >= 12 {
			ampm = "PM"
		}
		if f == fixLowerAmPm {
			ampm = strings.ToLower(ampm)
		}
		b.WriteString(ampm)
	case fixNanosecond:
		writeNanoAuto(b, nano)
	case fixNanosecond3:
		fmt.Fprintf(b, ".%03d", nano/1_000_000%1000)
	case fixNanosecond6:
		fmt.Fprintf(b, ".%06d", nano/1_000%1_000_000)
	case fixNanosecond9:
		fmt.Fprintf(b, ".%09d", nano)
	case fixNanosecond3NoDot:
		fmt.Fprintf(b, "%03d", nano/1_000_000%1000)
	case fixNanosecond6NoDot:
		fmt.Fprintf(b, "%06d", nano/1_000%1_000_000)
	case fixNanosecond9NoDot:
		fmt.Fprintf(b, "%09d", nano)
	case fixTimezoneName:
		b.WriteString(zoneName(t))
	case fixTimezoneOffset:
		return formatOffset(b, off, precMinutes, false, false)
	case fixTimezoneOffsetColon:
		return formatOffset(b, off, precMinutes, true, false)
	case fixTimezoneOffsetDoubleColon:
		return formatOffset(b, off, precSeconds, true, false)
	case fixTimezoneOffsetTripleColon:
		return formatOffset(b, off, precHours, false, false)
	case fixRFC3339:
		return writeRFC3339(b, t)
	default:
		return false
	}
	return true
}

type offsetPrecision int

const (
	precHours offsetPrecision = iota
	precMinutes
	precSeconds
)

// formatOffset is chrono's OffsetFormat::format with zero padding.
func formatOffset(b *strings.Builder, off int, prec offsetPrecision, colons, allowZulu bool) bool {
	if allowZulu && off == 0 {
		b.WriteByte('Z')
		return true
	}
	sign := byte('+')
	if off < 0 {
		sign, off = '-', -off
	}
	var hours, mins, secs int
	switch prec {
	case precHours:
		hours = off / 3600
	case precMinutes:
		minutes := (off + 30) / 60
		mins, hours = minutes%60, minutes/60
	case precSeconds:
		minutes := off / 60
		secs, mins, hours = off%60, minutes%60, minutes/60
	}
	b.WriteByte(sign)
	if !writeHundreds(b, hours) {
		return false
	}
	if prec >= precMinutes {
		if colons {
			b.WriteByte(':')
		}
		writeHundreds(b, mins)
	}
	if prec == precSeconds {
		if colons {
			b.WriteByte(':')
		}
		writeHundreds(b, secs)
	}
	return true
}

func writeDate(b *strings.Builder, t time.Time) {
	if y := t.Year(); y >= 0 && y <= 9999 {
		fmt.Fprintf(b, "%04d", y)
	} else {
		fmt.Fprintf(b, "%+05d", y)
	}
	fmt.Fprintf(b, "-%02d-%02d", int(t.Month()), t.Day())
}

// writeRFC3339 is chrono's write_rfc3339 with SecondsFormat::AutoSi and
// without Z.
func writeRFC3339(b *strings.Builder, t time.Time) bool {
	writeDate(b, t)
	fmt.Fprintf(b, "T%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
	writeNanoAuto(b, t.Nanosecond())
	_, off := t.Zone()
	return formatOffset(b, off, precMinutes, true, false)
}

// zoneName is the chrono offset Display: "UTC" for Utc, else the FixedOffset
// Display (e.g. "+09:30", "-05:00:30").
func zoneName(t time.Time) string {
	if t.Location() == time.UTC {
		return "UTC"
	}
	_, off := t.Zone()
	sign := '+'
	if off < 0 {
		sign, off = '-', -off
	}
	sec, mins := off%60, off/60
	if sec == 0 {
		return fmt.Sprintf("%c%02d:%02d", sign, mins/60, mins%60)
	}
	return fmt.Sprintf("%c%02d:%02d:%02d", sign, mins/60, mins%60, sec)
}

// Display formats t like chrono's Display for DateTime: the local date and
// time (with 0, 3, 6 or 9 fractional digits), a space and the offset name,
// e.g. "2000-01-01 12:00:00.123 UTC".
func Display(t time.Time) string {
	var b strings.Builder
	writeDate(&b, t)
	fmt.Fprintf(&b, " %02d:%02d:%02d", t.Hour(), t.Minute(), t.Second())
	writeNanoAuto(&b, t.Nanosecond())
	b.WriteByte(' ')
	b.WriteString(zoneName(t))
	return b.String()
}
