package parser

import "github.com/aviddiviner/go-hurl/reader"

// Combinators ported from hurl_core's combinator module. They backtrack by
// saving and restoring the reader cursor.

// optional tries to consume one instance of f, returning nil (and resetting
// the cursor) on a recoverable error.
func optional[T any](f func(*reader.Reader) (T, *ParseError), r *reader.Reader) (*T, *ParseError) {
	start := r.Cursor()
	v, err := f(r)
	if err != nil {
		if err.Recoverable {
			r.Seek(start)
			return nil, nil
		}
		return nil, err
	}
	return &v, nil
}

// recover makes an error recoverable, without resetting the cursor.
func recover[T any](f func(*reader.Reader) (T, *ParseError), r *reader.Reader) (T, *ParseError) {
	v, err := f(r)
	if err != nil {
		return v, err.toRecoverable()
	}
	return v, nil
}

// nonRecover makes an error non-recoverable.
func nonRecover[T any](f func(*reader.Reader) (T, *ParseError), r *reader.Reader) (T, *ParseError) {
	v, err := f(r)
	if err != nil {
		return v, err.toNonRecoverable()
	}
	return v, nil
}

// zeroOrMore consumes zero or more instances of f.
func zeroOrMore[T any](f func(*reader.Reader) (T, *ParseError), r *reader.Reader) ([]T, *ParseError) {
	var vs []T
	for {
		initial := r.Cursor()
		if r.IsEOF() {
			return vs, nil
		}
		v, err := f(r)
		if err != nil {
			if err.Recoverable {
				r.Seek(initial)
				return vs, nil
			}
			return nil, err
		}
		vs = append(vs, v)
	}
}

// oneOrMore consumes one or more instances of f. Failing to parse the first
// one is a non-recoverable error.
func oneOrMore[T any](f func(*reader.Reader) (T, *ParseError), r *reader.Reader) ([]T, *ParseError) {
	first, err := f(r)
	if err != nil {
		return nil, err.toNonRecoverable()
	}
	vs := []T{first}
	for {
		initial := r.Cursor()
		v, err := f(r)
		if err != nil {
			if err.Recoverable {
				r.Seek(initial)
				return vs, nil
			}
			return nil, err
		}
		vs = append(vs, v)
	}
}

// choice tries each parser in turn until one succeeds or fails with a
// non-recoverable error. The last parser's result is returned as is.
func choice[T any](fs []func(*reader.Reader) (T, *ParseError), r *reader.Reader) (T, *ParseError) {
	for i, f := range fs {
		start := r.Cursor()
		if i == len(fs)-1 {
			return f(r)
		}
		v, err := f(r)
		if err != nil && err.Recoverable {
			r.Seek(start)
			continue
		}
		return v, err
	}
	panic("parser: choice called with no parsers")
}
