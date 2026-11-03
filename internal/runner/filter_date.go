package runner

import (
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/timefmt"
)

const day = 24 * time.Hour

// evalDaysAfterNow returns the number of days between now and a date in
// the future.
func evalDaysAfterNow(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	d, ok := value.(ValueDate)
	if !ok {
		return nil, invalidInputType(value, "date", si, assert)
	}
	return NewInteger(int64(d.Value.Sub(time.Now()) / day)), nil
}

// evalDaysBeforeNow returns the number of days between now and a date in
// the past.
func evalDaysBeforeNow(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	d, ok := value.(ValueDate)
	if !ok {
		return nil, invalidInputType(value, "date", si, assert)
	}
	return NewInteger(int64(time.Since(d.Value) / day)), nil
}

// evalDateFormat formats a date with a chrono strftime format.
func evalDateFormat(value Value, format ast.Template, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	f, err := evalTemplate(format, variables)
	if err != nil {
		return nil, err
	}
	d, ok := value.(ValueDate)
	if !ok {
		return nil, invalidInputType(value, "date", si, assert)
	}
	s, ferr := timefmt.Format(d.Value, f)
	if ferr != nil {
		return nil, NewRunnerError(si, RunnerErrorFilterInvalidFormatSpecifier{Value: f}, assert)
	}
	return ValueString{s}, nil
}

// evalToDate parses a string to a date with a chrono strftime format. As
// chrono is strict about missing fields, it tries date + time + offset, then
// date + time, then date only.
func evalToDate(value Value, format ast.Template, variables *VariableSet, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	f, err := evalTemplate(format, variables)
	if err != nil {
		return nil, err
	}
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	if t, err := timefmt.ParseDateTime(s.Value, f); err == nil {
		return ValueDate{t.UTC()}, nil
	}
	if t, err := timefmt.ParseNaiveDateTime(s.Value, f); err == nil {
		return ValueDate{t}, nil
	}
	if t, err := timefmt.ParseNaiveDate(s.Value, f); err == nil {
		return ValueDate{t}, nil
	}
	return nil, NewRunnerError(si, RunnerErrorFilterDateParsingError{Date: s.Value, Format: f}, assert)
}
