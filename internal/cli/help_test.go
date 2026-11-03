package cli

import (
	"os"
	"strings"
	"testing"
)

func TestHelpMatchesUpstream(t *testing.T) {
	want, err := os.ReadFile("../../integration/hurl/tests_pty/help/help.out")
	if err != nil {
		t.Fatal(err)
	}
	got := renderHelp("hurl", 100, false)
	if got != string(want) {
		gl, wl := strings.Split(got, "\n"), strings.Split(string(want), "\n")
		for i := range min(len(gl), len(wl)) {
			if gl[i] != wl[i] {
				t.Fatalf("line %d:\ngot:  %q\nwant: %q", i+1, gl[i], wl[i])
			}
		}
		t.Fatalf("got %d lines, want %d", len(gl), len(wl))
	}
}

func TestHelpColor(t *testing.T) {
	got := renderHelp("hurl", 100, true)
	if !strings.Contains(got, "\x1b[1m\x1b[32mHTTP options:\x1b[0m\n") ||
		!strings.Contains(got, "  \x1b[1m\x1b[36m-h\x1b[0m, \x1b[1m\x1b[36m--help\x1b[0m     Print help") {
		t.Errorf("unexpected colored help:\n%s", got)
	}
}

func TestWrapHelp(t *testing.T) {
	tests := []struct {
		line  string
		width int
		want  string
	}{
		{"Control the format of error messages [default: short] [possible values: short, long]", 69,
			"Control the format of error messages [default: short] [possible\nvalues: short, long]"},
		{"a b", 0, "a\nb"},
		{"word", 2, "word"},
	}
	for _, tt := range tests {
		if got := wrapHelp(tt.line, tt.width); got != tt.want {
			t.Errorf("wrapHelp(%q, %d) = %q, want %q", tt.line, tt.width, got, tt.want)
		}
	}
}
