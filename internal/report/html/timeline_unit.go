package html

import (
	"math"
	"time"
)

// Units of the SVG timeline: values are float64, as in Hurl.

// timeUnit is a time in seconds, milliseconds or microseconds.
type timeUnit struct {
	unit  string
	value float64
}

const (
	unitSecond      = "s"
	unitMillisecond = "ms"
	unitMicrosecond = "µs"
)

func (t timeUnit) addRaw(raw float64) timeUnit { return timeUnit{t.unit, t.value + raw} }

// microseconds converts t to microseconds.
func (t timeUnit) microseconds() float64 {
	switch t.unit {
	case unitSecond:
		return t.value * 1_000_000.0
	case unitMillisecond:
		return t.value * 1_000.0
	}
	return t.value
}

// less compares t and other in microseconds, with f64::total_cmp.
func (t timeUnit) less(other timeUnit) bool {
	return totalCmp(t.microseconds(), other.microseconds()) < 0
}

// totalCmp is Rust's f64::total_cmp.
func totalCmp(a, b float64) int {
	key := func(f float64) int64 {
		bits := int64(math.Float64bits(f))
		return bits ^ int64(uint64(bits>>63)>>1)
	}
	ka, kb := key(a), key(b)
	switch {
	case ka < kb:
		return -1
	case ka > kb:
		return 1
	}
	return 0
}

// maxF and minF are Rust's f64::max and f64::min: NaN is ignored.
func maxF(a, b float64) float64 {
	if math.IsNaN(a) {
		return b
	}
	if math.IsNaN(b) {
		return a
	}
	return math.Max(a, b)
}

func minF(a, b float64) float64 {
	if math.IsNaN(a) {
		return b
	}
	if math.IsNaN(b) {
		return a
	}
	return math.Min(a, b)
}

// interval is a start and end value (times or pixels).
type interval[T any] struct{ start, end T }

// scale converts a time to a pixel value.
type scale struct {
	times  interval[float64]
	pixels interval[float64]
}

func newScale(times interval[time.Time], pixels interval[float64]) scale {
	end := float64(times.end.Sub(times.start).Microseconds())
	return scale{times: interval[float64]{0, end}, pixels: pixels}
}

// toPixel returns the pixel value of a time in microseconds.
func (s scale) toPixel(time float64) float64 {
	return (time - s.times.start) * (s.pixels.end - s.pixels.start) / (s.times.end - s.times.start)
}

// niceScale holds "nice" numbers given minimum and maximum values, and a
// number of ticks. Derived from "Graphics Gems, Volume 1" by Andrew S.
// Glassner.
type niceScale struct {
	minValue    float64
	maxValue    float64
	maxTicks    int
	rangeValue  float64
	tickSpacing float64
	niceMin     float64
	niceMax     float64
}

func newNiceScale(minValue, maxValue float64, maxTicks int) niceScale {
	rangeValue := niceNumber(maxValue-minValue, false)
	tickSpacing := niceNumber(rangeValue/float64(maxTicks-1), true)
	return niceScale{
		minValue:    minValue,
		maxValue:    maxValue,
		maxTicks:    maxTicks,
		rangeValue:  rangeValue,
		tickSpacing: tickSpacing,
		niceMin:     math.Floor(minValue/tickSpacing) * tickSpacing,
		niceMax:     math.Ceil(maxValue/tickSpacing) * tickSpacing,
	}
}

// niceNumber returns a "nice" number approximately equal to r. Rounds the
// number if round is true, otherwise takes the ceiling.
func niceNumber(r float64, round bool) float64 {
	exponent := floorLog10(r)
	fraction := r / powi(10, exponent)
	var niceFraction float64
	switch {
	case round && fraction < 1.5:
		niceFraction = 1
	case round && fraction < 3:
		niceFraction = 2
	case round && fraction < 7:
		niceFraction = 5
	case round:
		niceFraction = 10
	case fraction <= 1:
		niceFraction = 1
	case fraction <= 2:
		niceFraction = 2
	case fraction <= 5:
		niceFraction = 5
	default:
		niceFraction = 10
	}
	return niceFraction * powi(10, exponent)
}

// floorLog10 is Rust's `x.log10().floor() as i32`. math.Log10 isn't exact for
// powers of 10, so the result is adjusted.
func floorLog10(x float64) int32 {
	f := math.Floor(math.Log10(x))
	switch {
	case math.IsNaN(f):
		return 0
	case f <= math.MinInt32:
		return math.MinInt32
	case f >= math.MaxInt32:
		return math.MaxInt32
	}
	e := int32(f)
	// Powers of 10 are exact up to 1e22.
	if e < -22 || e >= 22 {
		return e
	}
	if powi(10, e+1) <= x {
		e++
	} else if powi(10, e) > x {
		e--
	}
	return e
}

// powi is Rust's f64::powi (compiler-rt __powidf2).
func powi(a float64, b int32) float64 {
	recip := b < 0
	r := 1.0
	for {
		if b&1 != 0 {
			r *= a
		}
		b /= 2
		if b == 0 {
			break
		}
		a *= a
	}
	if recip {
		return 1 / r
	}
	return r
}
