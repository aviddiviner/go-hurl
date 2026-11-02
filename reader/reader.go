// Package reader provides a rune-oriented cursor over a text buffer, tracking
// line and column positions for error reporting.
package reader

// Pos is a human-oriented position in a text buffer. Line and Column are 1-based.
type Pos struct {
	Line   int
	Column int
}

// Cursor is a position in a text buffer: a rune Index into the reader's buffer
// and a display Pos. Pos may be offset (see WithPos) so that a sub-reader can
// report positions relative to a parent reader.
type Cursor struct {
	Index int
	Pos   Pos
}

// Reader reads runes from a buffer, keeping track of the current Cursor.
type Reader struct {
	buf    []rune
	cursor Cursor
}

// New creates a reader positioned at the first rune of s.
func New(s string) *Reader {
	return WithPos(s, Pos{Line: 1, Column: 1})
}

// WithPos creates a reader over s whose display position starts at pos.
// The buffer index still starts at 0.
func WithPos(s string, pos Pos) *Reader {
	return &Reader{buf: []rune(s), cursor: Cursor{Index: 0, Pos: pos}}
}

func (r *Reader) Cursor() Cursor { return r.cursor }

func (r *Reader) Seek(to Cursor) { r.cursor = to }

func (r *Reader) IsEOF() bool { return r.cursor.Index == len(r.buf) }

// Read returns the next rune and advances the cursor. ok is false at EOF.
func (r *Reader) Read() (c rune, ok bool) {
	if r.cursor.Index >= len(r.buf) {
		return 0, false
	}
	c = r.buf[r.cursor.Index]
	r.cursor.Index++
	if !isCombiningCharacter(c) {
		r.cursor.Pos.Column++
	}
	if c == '\n' {
		r.cursor.Pos.Column = 1
		r.cursor.Pos.Line++
	}
	return c, true
}

// ReadN reads up to count runes, returning fewer if the buffer runs out.
func (r *Reader) ReadN(count int) string {
	s := make([]rune, 0, count)
	for range count {
		c, ok := r.Read()
		if !ok {
			break
		}
		s = append(s, c)
	}
	return string(s)
}

// ReadWhile reads runes while predicate holds.
func (r *Reader) ReadWhile(predicate func(rune) bool) string {
	start := r.cursor.Index
	for {
		c, ok := r.Peek()
		if !ok || !predicate(c) {
			return string(r.buf[start:r.cursor.Index])
		}
		r.Read()
	}
}

// ReadFrom returns the runes from start up to (excluding) the current index,
// without moving the cursor.
func (r *Reader) ReadFrom(start int) string {
	return string(r.buf[start:r.cursor.Index])
}

// Peek returns the next rune without advancing. ok is false at EOF.
func (r *Reader) Peek() (rune, bool) {
	if r.cursor.Index >= len(r.buf) {
		return 0, false
	}
	return r.buf[r.cursor.Index], true
}

// PeekIf returns the first upcoming rune satisfying predicate, without advancing.
func (r *Reader) PeekIf(predicate func(rune) bool) (rune, bool) {
	for i := r.cursor.Index; i < len(r.buf); i++ {
		if predicate(r.buf[i]) {
			return r.buf[i], true
		}
	}
	return 0, false
}

// PeekN returns up to count upcoming runes without advancing.
func (r *Reader) PeekN(count int) string {
	end := min(r.cursor.Index+count, len(r.buf))
	return string(r.buf[r.cursor.Index:end])
}

// isCombiningCharacter matches Hurl's (exclusive) Combining Diacritical Marks range.
func isCombiningCharacter(c rune) bool {
	return c > '\u0300' && c < '\u036F'
}
