package regex

import "testing"

func TestCountedRepetitionError(t *testing.T) {
	tests := []struct{ pattern, want string }{
		{`a{3}`, ""},
		{`a{3,}`, ""},
		{`a{3,5}?b`, ""},
		{`\{a}`, ""},
		{`[{a}]`, ""},
		{`[]{a}]`, ""},
		{`[[:alpha:]{]`, ""},
		{`(?:a){2}`, ""},
		{`(?P<n>a{2})`, ""},
		{`aa{a}`, "repetition quantifier expects a valid decimal"},
		{`a{,5}`, "repetition quantifier expects a valid decimal"},
		{`a{1,x}`, "repetition quantifier expects a valid decimal"},
		{`a{`, "unclosed counted repetition"},
		{`a{1`, "unclosed counted repetition"},
		{`a{1,`, "unclosed counted repetition"},
		{`a{1,2`, "unclosed counted repetition"},
		{`a{1x`, "unclosed counted repetition"},
		{`a{2,1}`, "invalid repetition count range, the start must be <= the end"},
		{`{}`, "repetition operator missing expression"},
		{`x|{2}`, "repetition operator missing expression"},
		{`(?i){2}`, "repetition operator missing expression"},
	}
	for _, tt := range tests {
		if got := countedRepetitionError(tt.pattern); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.pattern, got, tt.want)
		}
	}
}
