package timefmt

import (
	"errors"
	"math"
	"time"
)

// naiveTime is chrono's NaiveTime; nano may be >= 1e9 for a leap second.
type naiveTime struct{ hour, minute, second, nano int }

func newNaiveTime(hour, minute, second, nano int) (naiveTime, bool) {
	if hour >= 24 || minute >= 60 || second >= 60 || (nano >= 1_000_000_000 && second != 59) || nano >= 2_000_000_000 {
		return naiveTime{}, false
	}
	return naiveTime{hour, minute, second, nano}, true
}

// combine joins a date (midnight UTC) and a time in loc. A leap second rolls
// over to the next second.
func combine(d time.Time, t naiveTime, loc *time.Location) time.Time {
	return time.Date(d.Year(), d.Month(), d.Day(), t.hour, t.minute, t.second, t.nano, loc)
}

func isLeap(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

func daysInYear(y int) int {
	if isLeap(y) {
		return 366
	}
	return 365
}

func daysInMonth(y int, m int) int {
	return time.Date(y, time.Month(m)+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func fromYMD(y, m, d int) (time.Time, bool) {
	if y < minYear || y > maxYear || m < 1 || m > 12 || d < 1 || d > daysInMonth(y, m) {
		return time.Time{}, false
	}
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC), true
}

func fromYO(y, o int) (time.Time, bool) {
	if y < minYear || y > maxYear || o < 1 || o > daysInYear(y) {
		return time.Time{}, false
	}
	return time.Date(y, 1, o, 0, 0, 0, 0, time.UTC), true
}

func isoWeeksInYear(y int) int {
	jan1 := time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC).Weekday()
	if jan1 == time.Thursday || (isLeap(y) && jan1 == time.Wednesday) {
		return 53
	}
	return 52
}

func fromISOYWD(y, week int, wd time.Weekday) (time.Time, bool) {
	if y < minYear-1 || y > maxYear+1 || week < 1 || week > isoWeeksInYear(y) {
		return time.Time{}, false
	}
	jan4 := time.Date(y, 1, 4, 0, 0, 0, 0, time.UTC)
	monday := jan4.AddDate(0, 0, -daysSince(jan4.Weekday(), time.Monday))
	d := monday.AddDate(0, 0, (week-1)*7+daysSince(wd, time.Monday))
	if d.Year() < minYear || d.Year() > maxYear {
		return time.Time{}, false
	}
	return d, true
}

func quarter(d time.Time) int64 { return int64((int(d.Month())-1)/3 + 1) }

// resolveYear combines a full year, a year divided by 100 and a year modulo
// 100, as chrono's Parsed::to_naive_date does.
func resolveYear(y, q, r opt[int64]) (opt[int64], error) {
	switch {
	case !q.set && !r.set:
		return y, nil
	case y.set && (!r.set || inRange(r.v, 0, 99)):
		if y.v < 0 {
			return opt[int64]{}, ErrImpossible
		}
		if q.or(y.v/100) == y.v/100 && r.or(y.v%100) == y.v%100 {
			return y, nil
		}
		return opt[int64]{}, ErrImpossible
	case !y.set && q.set && r.set && inRange(r.v, 0, 99):
		if q.v < 0 {
			return opt[int64]{}, ErrImpossible
		}
		v := q.v*100 + r.v
		if v > math.MaxInt32 {
			return opt[int64]{}, ErrOutOfRange
		}
		return some(v), nil
	case !y.set && !q.set && r.set && inRange(r.v, 0, 99):
		if r.v < 70 {
			return some(r.v + 2000), nil
		}
		return some(r.v + 1900), nil
	case !y.set && q.set && !r.set:
		return opt[int64]{}, ErrNotEnough
	}
	return opt[int64]{}, ErrOutOfRange
}

func optEq(o opt[int64], v int64, valid bool) bool {
	if !o.set {
		return true
	}
	return valid && o.v == v
}

func (p *parsed) toNaiveDate() (time.Time, error) {
	givenYear, err := resolveYear(p.year, p.yearDiv100, p.yearMod100)
	if err != nil {
		return time.Time{}, err
	}
	givenISOYear, err := resolveYear(p.isoYear, p.isoYearDiv100, p.isoYearMod100)
	if err != nil {
		return time.Time{}, err
	}

	verifyYMD := func(d time.Time) bool {
		y := int64(d.Year())
		return p.year.or(y) == y &&
			optEq(p.yearDiv100, y/100, y >= 0) &&
			optEq(p.yearMod100, y%100, y >= 0) &&
			p.month.or(int64(d.Month())) == int64(d.Month()) &&
			p.day.or(int64(d.Day())) == int64(d.Day())
	}
	verifyISOWeekDate := func(d time.Time) bool {
		iy, iw := d.ISOWeek()
		y := int64(iy)
		return p.isoYear.or(y) == y &&
			optEq(p.isoYearDiv100, y/100, y >= 0) &&
			optEq(p.isoYearMod100, y%100, y >= 0) &&
			p.isoWeek.or(int64(iw)) == int64(iw) &&
			p.weekday.or(d.Weekday()) == d.Weekday()
	}
	verifyOrdinal := func(d time.Time) bool {
		return p.ordinal.or(int64(d.YearDay())) == int64(d.YearDay()) &&
			p.weekFromSun.or(int64(weeksFrom(d, time.Sunday))) == int64(weeksFrom(d, time.Sunday)) &&
			p.weekFromMon.or(int64(weeksFrom(d, time.Monday))) == int64(weeksFrom(d, time.Monday))
	}

	var verified bool
	var date time.Time
	switch {
	case givenYear.set && p.month.set && p.day.set:
		d, ok := fromYMD(int(givenYear.v), int(p.month.v), int(p.day.v))
		if !ok {
			return time.Time{}, ErrOutOfRange
		}
		date, verified = d, verifyISOWeekDate(d) && verifyOrdinal(d)
	case givenYear.set && p.ordinal.set:
		d, ok := fromYO(int(givenYear.v), int(p.ordinal.v))
		if !ok {
			return time.Time{}, ErrOutOfRange
		}
		date, verified = d, verifyYMD(d) && verifyISOWeekDate(d) && verifyOrdinal(d)
	case givenYear.set && p.weekFromSun.set && p.weekday.set:
		d, err := resolveWeekDate(int(givenYear.v), int(p.weekFromSun.v), p.weekday.v, time.Sunday)
		if err != nil {
			return time.Time{}, err
		}
		date, verified = d, verifyYMD(d) && verifyISOWeekDate(d) && verifyOrdinal(d)
	case givenYear.set && p.weekFromMon.set && p.weekday.set:
		d, err := resolveWeekDate(int(givenYear.v), int(p.weekFromMon.v), p.weekday.v, time.Monday)
		if err != nil {
			return time.Time{}, err
		}
		date, verified = d, verifyYMD(d) && verifyISOWeekDate(d) && verifyOrdinal(d)
	case givenISOYear.set && p.isoWeek.set && p.weekday.set:
		d, ok := fromISOYWD(int(givenISOYear.v), int(p.isoWeek.v), p.weekday.v)
		if !ok {
			return time.Time{}, ErrOutOfRange
		}
		date, verified = d, verifyYMD(d) && verifyOrdinal(d)
	default:
		return time.Time{}, ErrNotEnough
	}
	if !verified {
		return time.Time{}, ErrImpossible
	}
	if p.quarter.set && p.quarter.v != quarter(date) {
		return time.Time{}, ErrImpossible
	}
	return date, nil
}

func resolveWeekDate(year, week int, wd, weekStart time.Weekday) (time.Time, error) {
	if week > 53 {
		return time.Time{}, ErrOutOfRange
	}
	first, ok := fromYO(year, 1)
	if !ok {
		return time.Time{}, ErrOutOfRange
	}
	firstWeekStart := 1 + daysSince(weekStart, first.Weekday())
	ordinal := firstWeekStart + (week-1)*7 + daysSince(wd, weekStart)
	if ordinal <= 0 {
		return time.Time{}, ErrImpossible
	}
	d, ok := fromYO(year, ordinal)
	if !ok {
		return time.Time{}, ErrImpossible
	}
	return d, nil
}

func (p *parsed) toNaiveTime() (naiveTime, error) {
	if !p.hourDiv12.set || !p.hourMod12.set || !p.minute.set {
		return naiveTime{}, ErrNotEnough
	}
	if !inRange(p.hourDiv12.v, 0, 1) || !inRange(p.hourMod12.v, 0, 11) || !inRange(p.minute.v, 0, 59) {
		return naiveTime{}, ErrOutOfRange
	}
	hour := int(p.hourDiv12.v*12 + p.hourMod12.v)
	var second, nano int
	switch s := p.second.or(0); {
	case inRange(s, 0, 59):
		second = int(s)
	case s == 60:
		second, nano = 59, 1_000_000_000
	default:
		return naiveTime{}, ErrOutOfRange
	}
	if p.nanosecond.set {
		if !inRange(p.nanosecond.v, 0, 999_999_999) {
			return naiveTime{}, ErrOutOfRange
		}
		if !p.second.set {
			return naiveTime{}, ErrNotEnough
		}
		nano += int(p.nanosecond.v)
	}
	t, ok := newNaiveTime(hour, int(p.minute.v), second, nano)
	if !ok {
		return naiveTime{}, ErrOutOfRange
	}
	return t, nil
}

func unixSeconds(d time.Time, t naiveTime) int64 {
	return time.Date(d.Year(), d.Month(), d.Day(), t.hour, t.minute, t.second, 0, time.UTC).Unix()
}

func (p *parsed) toNaiveDateTimeWithOffset(offset int64) (time.Time, naiveTime, error) {
	date, dateErr := p.toNaiveDate()
	tm, timeErr := p.toNaiveTime()
	if dateErr == nil && timeErr == nil {
		ts := unixSeconds(date, tm) - offset
		if p.timestamp.set && p.timestamp.v != ts && !(tm.nano >= 1_000_000_000 && p.timestamp.v == ts+1) {
			return time.Time{}, naiveTime{}, ErrImpossible
		}
		return date, tm, nil
	}
	if !p.timestamp.set {
		if dateErr != nil {
			return time.Time{}, naiveTime{}, dateErr
		}
		return time.Time{}, naiveTime{}, timeErr
	}
	switch {
	case errors.Is(dateErr, ErrOutOfRange) || errors.Is(timeErr, ErrOutOfRange):
		return time.Time{}, naiveTime{}, ErrOutOfRange
	case errors.Is(dateErr, ErrImpossible) || errors.Is(timeErr, ErrImpossible):
		return time.Time{}, naiveTime{}, ErrImpossible
	}
	ts := p.timestamp.v + offset
	if (offset > 0 && ts < p.timestamp.v) || (offset < 0 && ts > p.timestamp.v) {
		return time.Time{}, naiveTime{}, ErrOutOfRange
	}
	maxUnix := time.Date(maxYear, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
	minUnix := time.Date(minYear, 1, 1, 0, 0, 0, 0, time.UTC).Unix()
	if ts > maxUnix || ts < minUnix {
		return time.Time{}, naiveTime{}, ErrOutOfRange
	}
	dt := time.Unix(ts, 0).UTC()
	q := *p
	if q.second.set && q.second.v == 60 {
		switch dt.Second() {
		case 59:
		case 0:
			dt = dt.Add(-time.Second)
		default:
			return time.Time{}, naiveTime{}, ErrImpossible
		}
	} else if err := setRange(&q.second, int64(dt.Second()), 0, 60); err != nil {
		return time.Time{}, naiveTime{}, err
	}
	for _, err := range []error{
		setRange(&q.year, int64(dt.Year()), math.MinInt32, math.MaxInt32),
		setRange(&q.ordinal, int64(dt.YearDay()), 1, 366),
		q.setHour(int64(dt.Hour())),
		setRange(&q.minute, int64(dt.Minute()), 0, 59),
	} {
		if err != nil {
			return time.Time{}, naiveTime{}, err
		}
	}
	date, err := q.toNaiveDate()
	if err != nil {
		return time.Time{}, naiveTime{}, err
	}
	tm, err = q.toNaiveTime()
	if err != nil {
		return time.Time{}, naiveTime{}, err
	}
	return date, tm, nil
}

func (p *parsed) toDateTime() (time.Time, error) {
	var offset int64
	switch {
	case p.offset.set:
		offset = p.offset.v
	case p.timestamp.set:
	default:
		return time.Time{}, ErrNotEnough
	}
	date, tm, err := p.toNaiveDateTimeWithOffset(offset)
	if err != nil {
		return time.Time{}, err
	}
	if offset <= -86400 || offset >= 86400 {
		return time.Time{}, ErrOutOfRange
	}
	return combine(date, tm, time.FixedZone("", int(offset))), nil
}
