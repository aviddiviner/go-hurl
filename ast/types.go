package ast

import (
	"fmt"
	"math"
	"strconv"
)

// Count is a count, either finite or infinite: CountFinite or CountInfinite.
type Count interface {
	count()
	String() string
}

type CountFinite struct{ Value int }

type CountInfinite struct{}

func (CountFinite) count()   {}
func (CountInfinite) count() {}

func (c CountFinite) String() string { return strconv.Itoa(c.Value) }
func (CountInfinite) String() string { return "-1" }

// CountFromInt32 converts n to a Count, where -1 means infinite.
func CountFromInt32(n int32) (Count, error) {
	switch {
	case n < -1:
		return nil, fmt.Errorf("%d is not in -1..=%d", n, math.MaxInt32)
	case n == -1:
		return CountInfinite{}, nil
	default:
		return CountFinite{Value: int(n)}, nil
	}
}

type DurationUnit int

const (
	DurationUnitMilliSecond DurationUnit = iota
	DurationUnitSecond
	DurationUnitMinute
	DurationUnitHour
)

func (u DurationUnit) String() string {
	switch u {
	case DurationUnitMilliSecond:
		return "ms"
	case DurationUnitSecond:
		return "s"
	case DurationUnitMinute:
		return "m"
	case DurationUnitHour:
		return "h"
	}
	return ""
}

// ParseDurationUnit parses "ms", "s", "m" or "h".
func ParseDurationUnit(s string) (DurationUnit, error) {
	switch s {
	case "ms":
		return DurationUnitMilliSecond, nil
	case "s":
		return DurationUnitSecond, nil
	case "m":
		return DurationUnitMinute, nil
	case "h":
		return DurationUnitHour, nil
	}
	return 0, fmt.Errorf("Invalid duration unit %s", s)
}

// BytesPerSec is a bit rate.
type BytesPerSec uint64

func (b BytesPerSec) String() string { return strconv.FormatUint(uint64(b), 10) }

// Index is a 1-based index. The zero value is invalid; use NewIndex or
// IndexFromZeroBased.
type Index struct{ value int }

// NewIndex creates an index from a 1-based value. It panics if value < 1.
func NewIndex(value int) Index {
	if value < 1 {
		panic(fmt.Sprintf("ast: invalid 1-based index %d", value))
	}
	return Index{value: value}
}

// IndexFromZeroBased creates an index from a 0-based value.
func IndexFromZeroBased(value int) Index { return NewIndex(value + 1) }

func (i Index) Get() int { return i.value }

func (i Index) ToZeroBased() int { return i.value - 1 }

// Add returns i advanced by n.
func (i Index) Add(n int) Index { return NewIndex(i.value + n) }

func (i Index) String() string { return strconv.Itoa(i.value) }
