package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func TestBase64Parse(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
		index int
	}{
		{"", nil, 0},
		{"AA==;", []byte{0}, 4},
		{"AA", []byte{0}, 2},
		{"AA;", []byte{0}, 2},
		{"TWE=;", []byte{77, 97}, 4},
		{"TWFu;", []byte{77, 97, 110}, 4},
		{"TW E=\n;", []byte{77, 97}, 5},
		{"YW55IGNhcm5hbCBwbGVhcw==;", []byte("any carnal pleas"), -1},
		{"YW55IGNhcm5hbCBwbGVhc3U=;", []byte("any carnal pleasu"), -1},
		{"YW55IGNhcm5hbCBwbGVhc3Vy;", []byte("any carnal pleasur"), -1},
		{"YW55IGNhcm5hbCBwbGVhcw;", []byte("any carnal pleas"), -1},
		{"YW55IGNhcm5hbCBwbGVhc3U;", []byte("any carnal pleasu"), -1},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, base64Parse(r), tt.want)
		if tt.index >= 0 {
			assertCursor(t, r, tt.index)
		}
	}
}

func TestBase64Decode(t *testing.T) {
	v := func(c rune) int {
		n, ok := base64Value(c)
		if !ok {
			t.Fatalf("invalid base64 char %q", c)
		}
		return n
	}
	assertEqual(t, decodeTwoChars(v('A'), v('A')), []byte{0})
	assertEqual(t, decodeTwoChars(v('A'), v('Q')), []byte{1})
	assertEqual(t, decodeTwoChars(v('T'), v('Q')), []byte{77})
	assertEqual(t, decodeThreeChars(v('T'), v('W'), v('E')), []byte{77, 97})
	assertEqual(t, decodeFourChars(v('Y'), v('W'), v('5'), v('5')), []byte{97, 110, 121})
	assertEqual(t, decodeFourChars(v('T'), v('W'), v('F'), v('u')), []byte{77, 97, 110})
}
