package reader

import (
	"testing"
	"unicode"
)

func TestBasicReader(t *testing.T) {
	r := New("hi")
	if r.Cursor().Index != 0 || r.IsEOF() {
		t.Fatalf("bad initial state: %+v", r.Cursor())
	}
	if got := r.PeekN(2); got != "hi" {
		t.Fatalf("PeekN = %q", got)
	}
	if c, _ := r.Read(); c != 'h' {
		t.Fatalf("Read = %q", c)
	}
	if c, _ := r.Peek(); c != 'i' || r.Cursor().Index != 1 {
		t.Fatalf("Peek = %q at %d", c, r.Cursor().Index)
	}
	r.Read()
	if !r.IsEOF() {
		t.Fatal("expected EOF")
	}
	if _, ok := r.Read(); ok {
		t.Fatal("Read past EOF")
	}
}

func TestReadFrom(t *testing.T) {
	r := New("abcdefgh")
	r.ReadN(5)
	if got := r.ReadFrom(3); got != "de" {
		t.Fatalf("ReadFrom = %q", got)
	}
}

func TestReadWhile(t *testing.T) {
	tests := []struct {
		in, want string
		index    int
		eof      bool
	}{
		{"123456789", "123456789", 9, true},
		{"123456789abcde", "123456789", 9, false},
		{"abcde123456789", "", 0, false},
	}
	for _, tt := range tests {
		r := New(tt.in)
		if got := r.ReadWhile(unicode.IsDigit); got != tt.want || r.Cursor().Index != tt.index || r.IsEOF() != tt.eof {
			t.Errorf("%q: got %q at %d eof=%v", tt.in, got, r.Cursor().Index, r.IsEOF())
		}
	}
}

func TestWithPos(t *testing.T) {
	main := New("aaabb")
	main.ReadN(3)
	pos := main.Cursor().Pos
	sub := WithPos(main.ReadWhile(func(rune) bool { return true }), pos)
	if want := (Cursor{0, Pos{1, 4}}); sub.Cursor() != want {
		t.Fatalf("got %+v want %+v", sub.Cursor(), want)
	}
	sub.Read()
	if want := (Cursor{1, Pos{1, 5}}); sub.Cursor() != want {
		t.Fatalf("got %+v want %+v", sub.Cursor(), want)
	}
}

func TestLinesAndCombining(t *testing.T) {
	r := New("a\nbe\u0301")
	r.ReadN(4)
	if want := (Pos{2, 3}); r.Cursor().Pos != want {
		t.Fatalf("got %+v want %+v", r.Cursor().Pos, want)
	}
}

func TestPeekIf(t *testing.T) {
	notSpace := func(c rune) bool { return c != ' ' && c != '\t' }
	if c, _ := New("\t\t\tabc").PeekIf(notSpace); c != 'a' {
		t.Fatalf("got %q", c)
	}
	if _, ok := New("  ").PeekIf(notSpace); ok {
		t.Fatal("expected no match")
	}
}
