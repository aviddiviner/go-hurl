package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/internal/sourceerr"
)

func TestLevenshtein(t *testing.T) {
	assertEqual(t, levenshteinDistance("kitten", "sitting"), 3)
	assertEqual(t, levenshteinDistance("Saturday", "Sunday"), 3)
}

func TestSuggestion(t *testing.T) {
	validValues := []string{"Captures", "Asserts"}
	tests := []struct {
		actual string
		want   string
		ok     bool
	}{
		{"Asserts", "Asserts", true},
		{"Assert", "Asserts", true},
		{"assert", "Asserts", true},
		{"asser", "", false},
	}
	for _, tt := range tests {
		got, ok := suggestion(validValues, tt.actual)
		if got != tt.want || ok != tt.ok {
			t.Errorf("suggestion(%q) = %q, %v; want %q, %v", tt.actual, got, ok, tt.want, tt.ok)
		}
	}
}

func TestParsingError(t *testing.T) {
	err := &ParseError{Pos: pos(1, 5), Recoverable: false, Kind: ParseErrorURLInvalidStart{}}
	got := sourceerr.Render(err, "test.hurl", "GET abc", nil, sourceerr.OutputFormatTerminal{Color: false})
	want := `Parsing URL
  --> test.hurl:1:5
   |
 1 | GET abc
   |     ^ expecting http://, https:// or {{
   |`
	assertEqual(t, got, want)
}
