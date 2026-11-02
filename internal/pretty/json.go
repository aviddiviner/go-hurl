package pretty

import (
	"fmt"
	"io"
	"strings"
)

// Color selects whether JSON is prettified with ANSI escape codes or not.
type Color int

const (
	NoColor Color = iota
	Ansi
)

const maxIndentLevel = 100

// FormatError is an error raised by [Format]. Its concrete type is one of [EOFError],
// [InvalidByteError], [InvalidUTF8Error], [InvalidEscapeError], [MaxIndentLevelError] or
// [WriteError]. Positions are 0-based byte offsets in the input.
type FormatError interface {
	error
	formatError()
}

// EOFError is an unexpected end of input.
type EOFError struct{}

// InvalidByteError is an invalid byte at a position.
type InvalidByteError struct {
	Byte byte
	Pos  int
}

// InvalidUTF8Error is an invalid UTF-8 sequence of Len bytes.
type InvalidUTF8Error struct {
	Bytes [4]byte
	Len   int
	Pos   int
}

// InvalidEscapeError is an invalid escaped byte in a string.
type InvalidEscapeError struct {
	Byte byte
	Pos  int
}

// MaxIndentLevelError is returned when the input is nested too deeply.
type MaxIndentLevelError struct {
	Level int
	Pos   int
}

// WriteError wraps an error returned by the output writer.
type WriteError struct {
	Err error
}

func (EOFError) formatError()            {}
func (InvalidByteError) formatError()    {}
func (InvalidUTF8Error) formatError()    {}
func (InvalidEscapeError) formatError()  {}
func (MaxIndentLevelError) formatError() {}
func (WriteError) formatError()          {}

func debugStrU8(b byte) string {
	if (b >= 0x21 && b <= 0x7e) || b == ' ' {
		return fmt.Sprintf("0x%02x/'%c'", b, b)
	}
	return fmt.Sprintf("0x%02x", b)
}

func (EOFError) Error() string { return "unexpected end of file" }

func (e InvalidByteError) Error() string {
	return fmt.Sprintf("invalid byte %s at offset %d", debugStrU8(e.Byte), e.Pos)
}

func (e InvalidUTF8Error) Error() string {
	hex := make([]string, e.Len)
	for i, b := range e.Bytes[:e.Len] {
		hex[i] = fmt.Sprintf("0x%02x", b)
	}
	return fmt.Sprintf("invalid UTF-8 %d bytes %s at offset %d", e.Len, strings.Join(hex, " "), e.Pos)
}

func (e InvalidEscapeError) Error() string {
	return fmt.Sprintf("invalid escaped byte %s at offset %d", debugStrU8(e.Byte), e.Pos)
}

func (e MaxIndentLevelError) Error() string {
	return fmt.Sprintf("maximum indent level %d at offset %d", e.Level, e.Pos)
}

func (e WriteError) Error() string { return "error writing " + e.Err.Error() }

func (e WriteError) Unwrap() error { return e.Err }

// Format prettifies the JSON input to out, optionally with colour. The input is not
// normalised: escapes and numbers are written as is, and strings are validated as UTF-8 on the
// fly. On error, out may already contain partial output; it's up to the caller to discard it.
func Format(input []byte, color Color, out io.Writer) error {
	f := &formatter{input: input, color: color, out: out}
	return f.format()
}

type formatter struct {
	input []byte
	pos   int
	level int
	color Color
	out   io.Writer
}

func (f *formatter) peekByte() (byte, bool) {
	if f.pos < len(f.input) {
		return f.input[f.pos], true
	}
	return 0, false
}

func (f *formatter) nextByte() (byte, bool) {
	b, ok := f.peekByte()
	if ok {
		f.pos++
	}
	return b, ok
}

func (f *formatter) peekIs(expected byte) bool {
	b, ok := f.peekByte()
	return ok && b == expected
}

func (f *formatter) peekDigit() bool {
	b, ok := f.peekByte()
	return ok && b >= '0' && b <= '9'
}

func (f *formatter) expectByte(expected byte) error {
	b, ok := f.nextByte()
	if !ok {
		return EOFError{}
	}
	if b != expected {
		return InvalidByteError{b, f.pos - 1}
	}
	return nil
}

func (f *formatter) incLevel() error {
	if f.level >= maxIndentLevel {
		return MaxIndentLevelError{f.level, f.pos}
	}
	f.level++
	return nil
}

func (f *formatter) format() error {
	f.skipStartBOM()

	f.skipWhitespace()
	if err := f.parseValue(); err != nil {
		return err
	}
	f.skipWhitespace()

	if err := f.write("\n"); err != nil {
		return err
	}

	if b, ok := f.peekByte(); ok {
		return InvalidByteError{b, f.pos}
	}
	return nil
}

func (f *formatter) skipStartBOM() {
	if len(f.input) >= 3 && f.input[0] == 0xEF && f.input[1] == 0xBB && f.input[2] == 0xBF {
		f.pos = 3
	}
}

func (f *formatter) skipWhitespace() {
	for {
		b, ok := f.peekByte()
		if !ok || (b != ' ' && b != '\n' && b != '\r' && b != '\t') {
			return
		}
		f.pos++
	}
}

func (f *formatter) parseValue() error {
	b, ok := f.peekByte()
	if !ok {
		return EOFError{}
	}
	switch {
	case b == '"':
		return f.parseString(false)
	case b == '-' || (b >= '0' && b <= '9'):
		return f.parseNumber()
	case b == '{':
		return f.parseObject()
	case b == '[':
		return f.parseArray()
	case b == 't':
		return f.parseLiteral("true")
	case b == 'f':
		return f.parseLiteral("false")
	case b == 'n':
		return f.parseLiteral("null")
	}
	return InvalidByteError{b, f.pos}
}

func (f *formatter) parseObject() error {
	if err := f.expectByte('{'); err != nil {
		return err
	}

	f.skipWhitespace()
	if f.peekIs('}') {
		f.nextByte()
		return f.writeDelim("{}")
	}

	if err := f.writeDelim("{"); err != nil {
		return err
	}
	if err := f.write("\n"); err != nil {
		return err
	}
	if err := f.incLevel(); err != nil {
		return err
	}

	for first := true; ; first = false {
		f.skipWhitespace()
		if f.peekIs('}') {
			f.nextByte()
			f.level--
			return f.writeEnd("}")
		}

		if !first {
			if err := f.expectByte(','); err != nil {
				return err
			}
			f.skipWhitespace()
			if err := f.writeValueSep(); err != nil {
				return err
			}
		}

		if err := f.writeIndent(); err != nil {
			return err
		}
		if err := f.parseString(true); err != nil {
			return err
		}

		f.skipWhitespace()
		if err := f.expectByte(':'); err != nil {
			return err
		}
		if err := f.writeDelim(":"); err != nil {
			return err
		}
		if err := f.write(" "); err != nil {
			return err
		}

		f.skipWhitespace()
		if err := f.parseValue(); err != nil {
			return err
		}
	}
}

func (f *formatter) parseArray() error {
	if err := f.expectByte('['); err != nil {
		return err
	}

	f.skipWhitespace()
	if f.peekIs(']') {
		f.nextByte()
		return f.writeDelim("[]")
	}

	if err := f.writeDelim("["); err != nil {
		return err
	}
	if err := f.write("\n"); err != nil {
		return err
	}
	if err := f.incLevel(); err != nil {
		return err
	}

	for first := true; ; first = false {
		f.skipWhitespace()
		if f.peekIs(']') {
			f.nextByte()
			f.level--
			return f.writeEnd("]")
		}

		if !first {
			if err := f.expectByte(','); err != nil {
				return err
			}
			f.skipWhitespace()
			if err := f.writeValueSep(); err != nil {
				return err
			}
		}

		if err := f.writeIndent(); err != nil {
			return err
		}
		if err := f.parseValue(); err != nil {
			return err
		}
	}
}

// parseString processes a JSON string. The string is not normalised: escapes are preserved
// and the bytes are validated as UTF-8 on the fly.
func (f *formatter) parseString(key bool) error {
	start := f.pos
	if err := f.expectByte('"'); err != nil {
		return err
	}

	for {
		b, ok := f.peekByte()
		if !ok {
			return EOFError{}
		}
		switch {
		case b == '"':
			f.nextByte()
			s := f.input[start:f.pos]
			if key {
				return f.writeToken("\x1b[1;34m", s)
			}
			return f.writeToken("\x1b[0;32m", s)
		case b == '\\':
			f.nextByte()
			e, ok := f.nextByte()
			if !ok {
				return EOFError{}
			}
			switch e {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for range 4 {
					h, ok := f.nextByte()
					if !ok {
						return EOFError{}
					}
					if !isHexDigit(h) {
						return InvalidByteError{h, f.pos - 1}
					}
				}
			default:
				return InvalidEscapeError{e, f.pos}
			}
		case b <= 0x1F:
			return InvalidByteError{b, f.pos}
		default:
			if err := f.nextUTF8Char(); err != nil {
				return err
			}
		}
	}
}

func isHexDigit(b byte) bool {
	return (b >= '0' && b <= '9') || (b >= 'a' && b <= 'f') || (b >= 'A' && b <= 'F')
}

func (f *formatter) parseLiteral(lit string) error {
	for i := range len(lit) {
		if err := f.expectByte(lit[i]); err != nil {
			return err
		}
	}
	color := "\x1b[0;33m"
	if lit == "null" {
		color = "\x1b[0;35m"
	}
	return f.writeToken(color, []byte(lit))
}

func (f *formatter) parseNumber() error {
	start := f.pos

	if f.peekIs('-') {
		f.nextByte()
	}

	if err := f.parseInteger(); err != nil {
		return err
	}
	if err := f.parseFraction(); err != nil {
		return err
	}
	if err := f.parseExponent(); err != nil {
		return err
	}

	return f.writeToken("\x1b[0;36m", f.input[start:f.pos])
}

func (f *formatter) parseInteger() error {
	b, ok := f.peekByte()
	switch {
	case !ok:
		return EOFError{}
	case b == '0':
		f.nextByte()
		return nil
	case b >= '1' && b <= '9':
		f.nextByte()
		f.skipDigits()
		return nil
	}
	return InvalidByteError{b, f.pos}
}

// parseDigits processes one or more digits.
func (f *formatter) parseDigits() error {
	b, ok := f.peekByte()
	switch {
	case !ok:
		return EOFError{}
	case b >= '0' && b <= '9':
		f.skipDigits()
		return nil
	}
	return InvalidByteError{b, f.pos}
}

func (f *formatter) skipDigits() {
	for f.peekDigit() {
		f.pos++
	}
}

func (f *formatter) parseFraction() error {
	if !f.peekIs('.') {
		return nil
	}
	f.nextByte()
	return f.parseDigits()
}

func (f *formatter) parseExponent() error {
	if !f.peekIs('e') && !f.peekIs('E') {
		return nil
	}
	f.nextByte()
	if f.peekIs('+') || f.peekIs('-') {
		f.nextByte()
	}
	return f.parseDigits()
}

func cont(b byte) bool { return b&0xC0 == 0x80 }

// nextUTF8Char reads and advances past the next UTF-8 char (1 to 4 bytes), checking its
// validity. See <https://en.wikipedia.org/wiki/UTF-8#Byte_map>.
func (f *formatter) nextUTF8Char() error {
	startPos := f.pos

	b1, ok := f.nextByte()
	if !ok {
		return EOFError{}
	}
	if b1 < 0x80 {
		return nil
	}

	b2, ok := f.nextByte()
	if !ok {
		return EOFError{}
	}
	if b1 < 0xE0 {
		if b1 >= 0xC2 && b1 <= 0xDF && cont(b2) {
			return nil
		}
		return InvalidUTF8Error{[4]byte{b1, b2}, 2, startPos}
	}

	b3, ok := f.nextByte()
	if !ok {
		return EOFError{}
	}
	if b1 < 0xF0 {
		var valid bool
		switch {
		case b1 == 0xE0: // Overlong encodings
			valid = b2 >= 0xA0 && b2 <= 0xBF && cont(b3)
		case b1 == 0xED: // UTF-16 surrogates
			valid = b2 >= 0x80 && b2 <= 0x9F && cont(b3)
		default:
			valid = cont(b2) && cont(b3)
		}
		if valid {
			return nil
		}
		return InvalidUTF8Error{[4]byte{b1, b2, b3}, 3, f.pos}
	}

	b4, ok := f.nextByte()
	if !ok {
		return EOFError{}
	}
	var valid bool
	switch {
	case b1 == 0xF0: // Overlong encodings
		valid = b2 >= 0x90 && b2 <= 0xBF && cont(b3) && cont(b4)
	case b1 == 0xF4: // Code points above U+10FFFF
		valid = b2 >= 0x80 && b2 <= 0x8F && cont(b3) && cont(b4)
	case b1 >= 0xF1 && b1 <= 0xF3:
		valid = cont(b2) && cont(b3) && cont(b4)
	}
	if valid {
		return nil
	}
	return InvalidUTF8Error{[4]byte{b1, b2, b3, b4}, 4, f.pos}
}

var spaces = strings.Repeat(" ", 2*maxIndentLevel)

func (f *formatter) write(s string) error {
	if _, err := io.WriteString(f.out, s); err != nil {
		return WriteError{err}
	}
	return nil
}

func (f *formatter) writeBytes(s []byte) error {
	if _, err := f.out.Write(s); err != nil {
		return WriteError{err}
	}
	return nil
}

func (f *formatter) writeIndent() error {
	return f.write(spaces[:f.level*2])
}

// writeDelim writes structural characters ({, }, [, ], :, ,).
func (f *formatter) writeDelim(s string) error {
	if f.color == Ansi {
		return f.write("\x1b[1;39m" + s + "\x1b[0m")
	}
	return f.write(s)
}

func (f *formatter) writeEnd(s string) error {
	if err := f.write("\n"); err != nil {
		return err
	}
	if err := f.writeIndent(); err != nil {
		return err
	}
	return f.writeDelim(s)
}

func (f *formatter) writeValueSep() error {
	if err := f.writeDelim(","); err != nil {
		return err
	}
	return f.write("\n")
}

func (f *formatter) writeToken(color string, s []byte) error {
	if f.color != Ansi {
		return f.writeBytes(s)
	}
	if err := f.write(color); err != nil {
		return err
	}
	if err := f.writeBytes(s); err != nil {
		return err
	}
	return f.write("\x1b[0m")
}
