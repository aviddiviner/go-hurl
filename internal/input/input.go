// Package input represents the input of a read operation: a file or
// standard input. Ported from hurl_core's input module.
package input

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// Input is a file or standard input.
type Input struct {
	kind Kind
}

// Kind is either a File or Stdin.
type Kind interface {
	inputKind()
	String() string
}

// File is an input read from the file at Path.
type File struct{ Path string }

// Stdin is standard input, read once and cached in Content.
type Stdin struct{ Content string }

func (File) inputKind()  {}
func (Stdin) inputKind() {}

func (f File) String() string { return f.Path }
func (Stdin) String() string  { return "-" }

// New creates an input from a file path.
func New(path string) Input { return Input{kind: File{Path: path}} }

// FromStdin creates an input from standard input. Its content is read once
// and cached, so it can be read multiple times.
func FromStdin() (Input, error) { return fromReader(os.Stdin) }

func fromReader(r io.Reader) (Input, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return Input{}, ioError(err)
	}
	if !utf8.Valid(b) {
		return Input{}, errors.New("stream did not contain valid UTF-8")
	}
	return Input{kind: Stdin{Content: string(b)}}, nil
}

func (i Input) Kind() Kind { return i.kind }

func (i Input) String() string { return i.kind.String() }

// ReadToString reads the content of this input, removing any BOM from files.
// Error messages match Hurl's (Rust io::Error) wording.
func (i Input) ReadToString() (string, error) {
	switch k := i.kind.(type) {
	case File:
		b, err := os.ReadFile(k.Path)
		if err != nil {
			return "", ioError(err)
		}
		return stringFromUTF8(b)
	case Stdin:
		return k.Content, nil
	}
	return "", errors.New("input: no input")
}

func stringFromUTF8(b []byte) (string, error) {
	b = stripBOM(b)
	if err := validateUTF8(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func stripBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
}

// Utf8Error is an invalid UTF-8 error, worded like Rust's Utf8Error.
type Utf8Error struct {
	ValidUpTo int
	// ErrorLen is the length of the invalid sequence, or 0 if the input
	// ended in the middle of a sequence.
	ErrorLen int
}

func (e *Utf8Error) Error() string {
	if e.ErrorLen > 0 {
		return fmt.Sprintf("invalid utf-8 sequence of %d bytes from index %d", e.ErrorLen, e.ValidUpTo)
	}
	return fmt.Sprintf("incomplete utf-8 byte sequence from index %d", e.ValidUpTo)
}

// validateUTF8 follows Rust's core::str::run_utf8_validation, so that the
// reported error length matches.
func validateUTF8(v []byte) error {
	i := 0
	for i < len(v) {
		start := i
		first := v[i]
		if first < 0x80 {
			i++
			continue
		}
		fail := func(n int) error { return &Utf8Error{ValidUpTo: start, ErrorLen: n} }
		next := func() (byte, bool) {
			i++
			if i >= len(v) {
				return 0, false
			}
			return v[i], true
		}
		isCont := func(b byte) bool { return b&0xc0 == 0x80 }
		var w int
		switch {
		case first >= 0xc2 && first <= 0xdf:
			w = 2
		case first >= 0xe0 && first <= 0xef:
			w = 3
		case first >= 0xf0 && first <= 0xf4:
			w = 4
		default:
			return fail(1)
		}
		b, ok := next()
		if !ok {
			return fail(0)
		}
		switch w {
		case 2:
			if !isCont(b) {
				return fail(1)
			}
		case 3:
			valid := (first == 0xe0 && b >= 0xa0 && b <= 0xbf) ||
				(first >= 0xe1 && first <= 0xec && isCont(b)) ||
				(first == 0xed && b >= 0x80 && b <= 0x9f) ||
				(first >= 0xee && isCont(b))
			if !valid {
				return fail(1)
			}
			if b, ok = next(); !ok {
				return fail(0)
			} else if !isCont(b) {
				return fail(2)
			}
		case 4:
			valid := (first == 0xf0 && b >= 0x90 && b <= 0xbf) ||
				(first >= 0xf1 && first <= 0xf3 && isCont(b)) ||
				(first == 0xf4 && b >= 0x80 && b <= 0x8f)
			if !valid {
				return fail(1)
			}
			for n := 2; n <= 3; n++ {
				if b, ok = next(); !ok {
					return fail(0)
				} else if !isCont(b) {
					return fail(n)
				}
			}
		}
		i++
	}
	return nil
}

// ioError rewords OS errors like Rust's io::Error Display, e.g.
// "No such file or directory (os error 2)".
func ioError(err error) error {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err
	}
	msg := errno.Error()
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	return &osError{msg: fmt.Sprintf("%s (os error %d)", msg, int(errno)), err: err}
}

type osError struct {
	msg string
	err error
}

func (e *osError) Error() string { return e.msg }
func (e *osError) Unwrap() error { return e.err }
