package cli

import (
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
)

// Rust's \d is Unicode aware.
var durationRegex = regexp.MustCompile(`^(\p{Nd}+)([a-zA-Z]*)$`)

// durationFromStr parses a duration with or without a time unit (32s, 10m,
// 20000). Without a unit, defaultUnit is used.
func durationFromStr(value string, defaultUnit ast.DurationUnit) (time.Duration, error) {
	caps := durationRegex.FindStringSubmatch(value)
	if caps == nil {
		return 0, optionsErrorf("Invalid duration")
	}
	n, err := strconv.ParseUint(caps[1], 10, 64)
	if err != nil {
		return 0, optionsErrorf("Duration value too large")
	}
	unit := defaultUnit
	if caps[2] != "" {
		unit, err = ast.ParseDurationUnit(caps[2])
		if err != nil {
			return 0, optionsErrorf("%s", err)
		}
	}
	var factor uint64
	switch unit {
	case ast.DurationUnitMilliSecond:
		factor = 1
	case ast.DurationUnitSecond:
		factor = 1000
	case ast.DurationUnitMinute:
		factor = 1000 * 60
	case ast.DurationUnitHour:
		factor = 1000 * 60 * 60
	}
	// A time.Duration holds about 292 years, less than Rust's Duration: we
	// also reject durations that don't fit in it.
	const maxMillis = math.MaxInt64 / uint64(time.Millisecond)
	if n > math.MaxUint64/factor || n*factor > maxMillis {
		return 0, optionsErrorf("Duration value too large")
	}
	return time.Duration(n*factor) * time.Millisecond, nil
}
