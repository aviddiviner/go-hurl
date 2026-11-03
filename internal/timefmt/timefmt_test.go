package timefmt

import (
	"testing"
	"time"
)

// Expected values come from chrono 0.4.45's strftime test_strftime_docs (with
// the leap second removed: Go can't represent it) and formatting tests.
func TestFormatDocs(t *testing.T) {
	dt := time.Date(2001, 7, 8, 0, 34, 59, 26_490_708, time.FixedZone("", 34200))
	tests := []struct{ format, want string }{
		{"%Y", "2001"},
		{"%C", "20"},
		{"%y", "01"},
		{"%q", "3"},
		{"%m", "07"},
		{"%b", "Jul"},
		{"%B", "July"},
		{"%h", "Jul"},
		{"%d", "08"},
		{"%e", " 8"},
		{"%_d", " 8"},
		{"%a", "Sun"},
		{"%A", "Sunday"},
		{"%w", "0"},
		{"%u", "7"},
		{"%U", "27"},
		{"%W", "27"},
		{"%G", "2001"},
		{"%g", "01"},
		{"%V", "27"},
		{"%j", "189"},
		{"%D", "07/08/01"},
		{"%x", "07/08/01"},
		{"%F", "2001-07-08"},
		{"%v", " 8-Jul-2001"},
		{"%H", "00"},
		{"%k", " 0"},
		{"%_H", " 0"},
		{"%I", "12"},
		{"%l", "12"},
		{"%_I", "12"},
		{"%P", "am"},
		{"%p", "AM"},
		{"%M", "34"},
		{"%S", "59"},
		{"%f", "026490708"},
		{"%.f", ".026490708"},
		{"%.3f", ".026"},
		{"%.6f", ".026490"},
		{"%.9f", ".026490708"},
		{"%3f", "026"},
		{"%6f", "026490"},
		{"%9f", "026490708"},
		{"%R", "00:34"},
		{"%T", "00:34:59"},
		{"%X", "00:34:59"},
		{"%r", "12:34:59 AM"},
		{"%Z", "+09:30"},
		{"%z", "+0930"},
		{"%:z", "+09:30"},
		{"%::z", "+09:30:00"},
		{"%:::z", "+09"},
		{"%c", "Sun Jul  8 00:34:59 2001"},
		{"%+", "2001-07-08T00:34:59.026490708+09:30"},
		{"%s", "994518299"},
		{"%t", "\t"},
		{"%n", "\n"},
		{"%%", "%"},
		{"  %Y%d%m%%%%%t%H%M%S\t", "  20010807%%\t003459\t"},
		{"  %Y%d%m%%%%%t%H:%P:%M%S%:::z\t", "  20010807%%\t00:am:3459+09\t"},
		// padding modifiers
		{"%-d", "8"},
		{"%0e", "08"},
		{"%-j", "189"},
		{"%_m", " 7"},
		{"%-m/%-d/%Y", "7/8/2001"},
	}
	for _, tt := range tests {
		got, err := Format(dt, tt.format)
		if err != nil || got != tt.want {
			t.Errorf("Format(%q) = %q, %v; want %q", tt.format, got, err, tt.want)
		}
	}

	got, _ := Format(dt.UTC(), "%+")
	if want := "2001-07-07T15:04:59.026490708+00:00"; got != want {
		t.Errorf("Format(UTC, %%+) = %q, want %q", got, want)
	}
	got, _ = Format(time.Date(2001, 7, 8, 0, 34, 59, 26_490_000, time.FixedZone("", 34200)), "%.f")
	if want := ".026490"; got != want {
		t.Errorf("Format(%%.f) = %q, want %q", got, want)
	}
	got, _ = Format(dt.UTC(), "%Z %z")
	if want := "UTC +0000"; got != want {
		t.Errorf("Format(UTC, %%Z %%z) = %q, want %q", got, want)
	}
}

// From chrono's formatting test_date_format.
func TestFormatYears(t *testing.T) {
	tests := []struct {
		y      int
		m, d   int
		format string
		want   string
	}{
		{2012, 3, 4, "%Y,%C,%y,%G,%g", "2012,20,12,2012,12"},
		{2012, 3, 4, "%m,%b,%h,%B", "03,Mar,Mar,March"},
		{2012, 3, 4, "%d,%e", "04, 4"},
		{2012, 3, 4, "%U,%W,%V", "10,09,09"},
		{2012, 3, 4, "%a,%A,%w,%u", "Sun,Sunday,0,7"},
		{2012, 3, 4, "%j", "064"},
		{2012, 3, 4, "%D,%x", "03/04/12,03/04/12"},
		{2012, 3, 4, "%F", "2012-03-04"},
		{2012, 3, 4, "%v", " 4-Mar-2012"},
		{12345, 1, 1, "%Y", "+12345"},
		{1234, 1, 1, "%Y", "1234"},
		{123, 1, 1, "%Y", "0123"},
		{12, 1, 1, "%Y", "0012"},
		{1, 1, 1, "%Y", "0001"},
		{0, 1, 1, "%Y", "0000"},
		{-1, 1, 1, "%Y", "-0001"},
		{-12, 1, 1, "%Y", "-0012"},
		{-123, 1, 1, "%Y", "-0123"},
		{-1234, 1, 1, "%Y", "-1234"},
		{-12345, 1, 1, "%Y", "-12345"},
		{2007, 12, 31, "%G,%g,%U,%W,%V", "2008,08,52,53,01"},
		{2010, 1, 3, "%G,%g,%U,%W,%V", "2009,09,01,00,53"},
	}
	for _, tt := range tests {
		d := time.Date(tt.y, time.Month(tt.m), tt.d, 0, 0, 0, 0, time.UTC)
		got, err := Format(d, tt.format)
		if err != nil || got != tt.want {
			t.Errorf("Format(%d-%d-%d, %q) = %q, %v; want %q", tt.y, tt.m, tt.d, tt.format, got, err, tt.want)
		}
	}
}

func TestFormatErrors(t *testing.T) {
	dt := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, f := range []string{"%%%", "%👻", "%", "%.", "%.4f", "%3x", "%-D", "%-a", "%#d", "%#z", "%:", "%Q", "%Y%"} {
		if got, err := Format(dt, f); err == nil {
			t.Errorf("Format(%q) = %q, want error", f, got)
		}
	}
}

func TestDisplay(t *testing.T) {
	tests := []struct {
		t    time.Time
		want string
	}{
		{time.Date(2000, 1, 1, 12, 0, 0, 123_000_000, time.UTC), "2000-01-01 12:00:00.123 UTC"},
		{time.Date(2000, 1, 1, 12, 0, 0, 0, time.UTC), "2000-01-01 12:00:00 UTC"},
		{time.Date(2000, 1, 1, 12, 0, 0, 123_456_000, time.UTC), "2000-01-01 12:00:00.123456 UTC"},
		{time.Date(2000, 1, 1, 12, 0, 0, 123_456_789, time.UTC), "2000-01-01 12:00:00.123456789 UTC"},
		{time.Date(2000, 1, 1, 12, 0, 0, 0, time.FixedZone("", -9*3600)), "2000-01-01 12:00:00 -09:00"},
	}
	for _, tt := range tests {
		if got := Display(tt.t); got != tt.want {
			t.Errorf("Display() = %q, want %q", got, tt.want)
		}
	}
}

// From chrono's datetime test_datetime_parse_from_str.
func TestParseDateTime(t *testing.T) {
	want := time.Date(2013, 8, 9, 23, 54, 35, 0, time.FixedZone("", -9*3600))
	ok := []struct{ s, f string }{
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09 00", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09:00", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09 : 00", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 --0900", "%b %d %Y %H:%M:%S -%z"},
		{"Aug 09 2013 23:54:35 +-0900", "%b %d %Y %H:%M:%S +%z"},
		{"Aug 09 2013 23:54:35 -09:00 ", "%b %d %Y %H:%M:%S %z "},
		{"Aug 09 2013 23:54:35 -09:00\n", "%b %d %Y %H:%M:%S %z "},
		{"Aug 09 2013 23:54:35 -09::00", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -0900::", "%b %d %Y %H:%M:%S %z::"},
		{"Aug 09 2013 23:54:35 -09:00:00", "%b %d %Y %H:%M:%S %z:00"},
		{"Aug 09 2013 23:54:35  -09:00", "%b %d %Y %H:%M:%S  %:z"},
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %:z"},
		{"Aug 09 2013 23:54:35 -09 : 00:", "%b %d %Y %H:%M:%S %:z:"},
		{"Aug 09 2013 23:54:35 -09:00::", "%b %d %Y %H:%M:%S %:z::"},
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %::z"},
		{"Aug 09 2013 23:54:35 -09001234", "%b %d %Y %H:%M:%S %::z1234"},
		{"Aug 09 2013 23:54:35 -0900\t\n", "%b %d %Y %H:%M:%S %::z\t\n"},
		{"Aug 09 2013 23:54:35 -09", "%b %d %Y %H:%M:%S %#z"},
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %#z"},
		{"2013-08-09T23:54:35-09:00", "%+"},
	}
	for _, tt := range ok {
		got, err := ParseDateTime(tt.s, tt.f)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseDateTime(%q, %q) = %v, %v; want %v", tt.s, tt.f, got, err, want)
		}
		if _, off := got.Zone(); off != -9*3600 {
			t.Errorf("ParseDateTime(%q, %q) offset = %d", tt.s, tt.f, off)
		}
	}
	ko := []struct{ s, f string }{
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %Z"},
		{"Aug 09 2013 23:54:35 PST", "%b %d %Y %H:%M:%S %Z"},
		{"Aug 09 2013 23:54:35 XXXXX", "%b %d %Y %H:%M:%S %Z"},
		{"Aug 09 2013 23:54:35 -09:00\n", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09:00:", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09:00: ", "%b %d %Y %H:%M:%S %z "},
		{"Aug 09 2013 23:54:35 -09", "%b %d %Y %H:%M:%S %z"},
		{"Aug 09 2013 23:54:35 -09", "%b %d %Y %H:%M:%S %:z"},
		{"Aug 09 2013 23:54:35 -09:00:00", "%b %d %Y %H:%M:%S %::z"},
		{"Aug 09 2013 23:54:35", "%b %d %Y %H:%M:%S"},
		{"Aug 09 2013 23:54:35 -0900", "%b %d %Y %H:%M:%S %👻"},
	}
	for _, tt := range ko {
		if got, err := ParseDateTime(tt.s, tt.f); err == nil {
			t.Errorf("ParseDateTime(%q, %q) = %v, want error", tt.s, tt.f, got)
		}
	}

	// From strftime test_strftime_docs (leap second removed).
	want = time.Date(2001, 7, 7, 15, 4, 59, 26_490_708, time.UTC)
	for _, s := range []string{"2001-07-07T15:04:59.026490708Z", "2001-07-07T15:04:59.026490708UTC", "2001-07-07t15:04:59.026490708utc"} {
		got, err := ParseDateTime(s, "%+")
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseDateTime(%q, %%+) = %v, %v; want %v", s, got, err, want)
		}
	}
	// From Hurl's to_date tests.
	got, err := ParseDateTime("Thu Aug 27 09:07:46 2020 +0200", "%a %b %d %H:%M:%S %Y %z")
	if want := time.Date(2020, 8, 27, 7, 7, 46, 0, time.UTC); err != nil || !got.Equal(want) {
		t.Errorf("ParseDateTime() = %v, %v; want %v", got, err, want)
	}
}

// From chrono's naive datetime test_datetime_parse_from_str and
// test_datetime_parse_from_str_with_spaces.
func TestParseNaiveDateTime(t *testing.T) {
	ymdhmsn := func(y, m, d, h, n, s, nano int) time.Time {
		return time.Date(y, time.Month(m), d, h, n, s, nano, time.UTC)
	}
	ok := []struct {
		s, f string
		want time.Time
	}{
		{"2014-5-7T12:34:56+09:30", "%Y-%m-%dT%H:%M:%S%z", ymdhmsn(2014, 5, 7, 12, 34, 56, 0)},
		{"2015-W06-1 000000", "%G-W%V-%u%H%M%S", ymdhmsn(2015, 2, 2, 0, 0, 0, 0)},
		{"Fri, 09 Aug 2013 23:54:35 GMT", "%a, %d %b %Y %H:%M:%S GMT", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"1441497364", "%s", ymdhmsn(2015, 9, 5, 23, 56, 4, 0)},
		{"1283929614.1234", "%s.%f", ymdhmsn(2010, 9, 8, 7, 6, 54, 1234)},
		{"1441497364.649", "%s%.3f", ymdhmsn(2015, 9, 5, 23, 56, 4, 649000000)},
		{"1497854303.087654", "%s%.6f", ymdhmsn(2017, 6, 19, 6, 38, 23, 87654000)},
		{" Aug 09 2013 23:54:35", " %b %d %Y %H:%M:%S", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{" Aug 09 2013  23:54:35 ", " %b %d %Y  %H:%M:%S ", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"\n\tAug 09 2013 23:54:35  ", "\n\t%b %d %Y %H:%M:%S  ", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"Aug 09 2013 23:54:35 ", "%b %d %Y %H:%M:%S\n", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"Aug 09 2013 23:54:35", "%b %d %Y\t%H:%M:%S", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"Aug 09 2013 23:54:35", " %b %d %Y %H:%M:%S", ymdhmsn(2013, 8, 9, 23, 54, 35, 0)},
		{"2020-08-27 09:07:46", "%Y-%m-%d %H:%M:%S", ymdhmsn(2020, 8, 27, 9, 7, 46, 0)},
		{"Wed, 13 Jan 2021 22:23:01 GMT", "%a, %d %b %Y %H:%M:%S GMT%Z", ymdhmsn(2021, 1, 13, 22, 23, 1, 0)},
		{"2022-01-03T10:20:30.123456Z", "%Y-%m-%dT%H:%M:%S%.fZ", ymdhmsn(2022, 1, 3, 10, 20, 30, 123456000)},
		{"03:04PM 2022-01-03", "%I:%M%p %F", ymdhmsn(2022, 1, 3, 15, 4, 0, 0)},
		{"Monday, January 03 2022 1:2", "%A, %B %d %Y %k:%M", ymdhmsn(2022, 1, 3, 1, 2, 0, 0)},
	}
	for _, tt := range ok {
		got, err := ParseNaiveDateTime(tt.s, tt.f)
		if err != nil || !got.Equal(tt.want) || got.Location() != time.UTC {
			t.Errorf("ParseNaiveDateTime(%q, %q) = %v, %v; want %v", tt.s, tt.f, got, err, tt.want)
		}
	}
	ko := []struct{ s, f string }{
		{"Sat, 09 Aug 2013 23:54:35 GMT", "%a, %d %b %Y %H:%M:%S GMT"},
		{"2014-5-7 Q2 12:3456", "%Y-%m-%d Q%q %H:%M:%S"},
		{"12:34:56", "%H:%M:%S"},
		{" Aug 09 2013 23:54:35", "%b %d %Y %H:%M:%S"},
		{"Aug 09 2013 23:54:35 ", "%b %d %Y %H:%M:%S"},
		{"Aug 09 2013 23:54:35\t", "%b %d %Y %H:%M:%S"},
		{"\nAug 09 2013 23:54:35", "%b %d %Y %H:%M:%S\n"},
		{"Aug 09 2013 23:54:35 !!!", "%b %d %Y %H:%M:%S "},
		{"2020-08-27", "%Y-%m-%d"},
	}
	for _, tt := range ko {
		if got, err := ParseNaiveDateTime(tt.s, tt.f); err == nil {
			t.Errorf("ParseNaiveDateTime(%q, %q) = %v, want error", tt.s, tt.f, got)
		}
	}
}

// From chrono's naive date test_date_parse_from_str.
func TestParseNaiveDate(t *testing.T) {
	ymd := func(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	ok := []struct {
		s, f string
		want time.Time
	}{
		{"2014-5-7T12:34:56+09:30", "%Y-%m-%dT%H:%M:%S%z", ymd(2014, 5, 7)},
		{"2015-W06-1=2015-033 Q1", "%G-W%V-%u = %Y-%j Q%q", ymd(2015, 2, 2)},
		{"Fri, 09 Aug 13", "%a, %d %b %y", ymd(2013, 8, 9)},
		{"2020-01-0", "%Y-%W-%w", ymd(2020, 1, 12)},
		{"2019-01-0", "%Y-%W-%w", ymd(2019, 1, 13)},
		{"2020-08-27", "%Y-%m-%d", ymd(2020, 8, 27)},
	}
	for _, tt := range ok {
		got, err := ParseNaiveDate(tt.s, tt.f)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("ParseNaiveDate(%q, %q) = %v, %v; want %v", tt.s, tt.f, got, err, tt.want)
		}
	}
	ko := []struct{ s, f string }{
		{"Sat, 09 Aug 2013", "%a, %d %b %Y"},
		{"2014-57", "%Y-%m-%d"},
		{"2014", "%Y"},
		{"2014-5-7 Q3", "%Y-%m-%d Q%q"},
		{"2020-08-27", "ymd"},
	}
	for _, tt := range ko {
		if got, err := ParseNaiveDate(tt.s, tt.f); err == nil {
			t.Errorf("ParseNaiveDate(%q, %q) = %v, want error", tt.s, tt.f, got)
		}
	}
}

// From Hurl's value_impl test_iso_date (RFC 3339 examples) and chrono's
// parse_from_rfc3339 tests.
func TestParseRFC3339(t *testing.T) {
	ok := []struct {
		s    string
		want time.Time
	}{
		{"1985-04-12T23:20:50.52Z", time.Date(1985, 4, 12, 23, 20, 50, 520_000_000, time.UTC)},
		{"1996-12-19T16:39:57-08:00", time.Date(1996, 12, 20, 0, 39, 57, 0, time.UTC)},
		{"1990-12-31T23:59:60Z", time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"1990-12-31T15:59:60-08:00", time.Date(1991, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"1937-01-01T12:00:27.87+00:20", time.Date(1937, 1, 1, 11, 40, 27, 870_000_000, time.UTC)},
		{"2015-02-18 23:16:09Z", time.Date(2015, 2, 18, 23, 16, 9, 0, time.UTC)},
	}
	for _, tt := range ok {
		got, err := ParseRFC3339(tt.s)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("ParseRFC3339(%q) = %v, %v; want %v", tt.s, got, err, tt.want)
		}
	}
	for _, s := range []string{"1978-01-15", "2015-02-18T23:16:09", "2015-02-18T23:16:09+0100", "2015-02-18T23:16:09Z ", "2015-02-30T23:16:09Z", "2015-02-18T24:16:09Z", "2015-02-18T23:16:09+24:00"} {
		if got, err := ParseRFC3339(s); err == nil {
			t.Errorf("ParseRFC3339(%q) = %v, want error", s, got)
		}
	}
}
