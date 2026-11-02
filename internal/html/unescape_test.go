package html

import (
	"fmt"
	"strings"
	"testing"
)

// Extracts from Python test suites: https://github.com/python/cpython/blob/main/Lib/test/test_html.py
func TestUnescape(t *testing.T) {
	check := func(text, want string) {
		t.Helper()
		if got := Unescape(text); got != want {
			t.Errorf("Unescape(%q) = %q, want %q", text, got, want)
		}
	}
	checkNum := func(num uint64, want string) {
		t.Helper()
		check(fmt.Sprintf("&#%d", num), want)
		check(fmt.Sprintf("&#%d;", num), want)
		check(fmt.Sprintf("&#x%x", num), want)
		check(fmt.Sprintf("&#x%x;", num), want)
	}

	check("Hurl&rlarr;", "Hurl⇄")

	// Check simple
	check("Foo &#xA9; bar &#x1D306; baz &#x2603; qux", "Foo © bar 𝌆 baz ☃ qux")

	// Check text with no character references
	check("no character references", "no character references")

	// Check & followed by invalid chars
	check("&\n&\t& &&", "&\n&\t& &&")

	// Check & followed by numbers and letters
	check("&0 &9 &a &0; &9; &a;", "&0 &9 &a &0; &9; &a;")

	// Check incomplete entities at the end of the string
	for _, x := range []string{"&", "&#", "&#x", "&#X", "&#y", "&#xy", "&#Xy"} {
		check(x, x)
		check(x+";", x+";")
	}

	// Check several combinations of numeric character references,
	// possibly followed by different characters
	nums := []struct {
		num uint64
		ch  rune
	}{
		{65, 'A'},
		{97, 'a'},
		{34, '"'},
		{38, '&'},
		{0x2603, '\u2603'},
		{0x101234, '\U00101234'},
	}
	formats := []string{
		"&#%d",     // Format &#1234 (without ending semi-colon)
		"&#%07d",   // Format &#0001234 (without ending semi-colon)
		"&#%d;",    // Format &#1234;
		"&#%07d;",  // Format &#0001234;
		"&#x%x",    // Format &#x1abc
		"&#x%06x",  // Format &#x001abc
		"&#x%x;",   // Format &#x1abc;
		"&#x%06x;", // Format &#x001abc;
		"&#x%X",    // Format &#x1ABC
		"&#x%06X",  // Format &#x001ABC
		"&#x%X;",   // Format &#x1ABC;
		"&#x%06X;", // Format &#x001ABC;
		"&#X%x;",   // Format &#X1abc;
		"&#X%06x;", // Format &#X001abc;
	}
	for _, f := range formats {
		for _, n := range nums {
			ref := fmt.Sprintf(f, n.num)
			ch := string(n.ch)
			check(ref, ch)
			check(ref+" ", ch+" ")
			check(ref+"X", ch+"X")
		}
	}

	// Check invalid code points
	for _, cp := range []uint64{0xD800, 0xDB00, 0xDC00, 0xDFFF, 0x110000} {
		checkNum(cp, "\ufffd")
	}

	// Check more invalid code points
	for _, cp := range []uint64{0x1, 0xb, 0xe, 0x7f, 0xfffe, 0xffff, 0x10fffe, 0x10ffff} {
		checkNum(cp, "")
	}

	// Check invalid numbers
	for _, tt := range []struct {
		num uint64
		ch  string
	}{{0x0d, "\r"}, {0x80, "\u20ac"}, {0x95, "\u2022"}} {
		checkNum(tt.num, tt.ch)
	}

	// Check small numbers
	checkNum(0, "\ufffd")
	checkNum(9, "\t")

	// Check a big number
	checkNum(1000000000000000000, "\ufffd")

	// Check that multiple trailing semicolons are handled correctly
	for _, e := range []string{"&quot;;", "&#34;;", "&#x22;;", "&#X22;;"} {
		check(e, "\";")
	}

	// Check that semicolons in the middle don't create problems
	for _, e := range []string{"&quot;quot;", "&#34;quot;", "&#x22;quot;", "&#X22;quot;"} {
		check(e, "\"quot;")
	}

	// Check triple adjacent charrefs
	for _, e := range []string{"&quot", "&#34", "&#x22", "&#X22"} {
		check(strings.Repeat(e+";", 3), `"""`)
	}

	// Check that the case is respected
	for _, e := range []string{"&amp", "&amp;", "&AMP", "&AMP;"} {
		check(e, "&")
	}
	for _, e := range []string{"&Amp", "&Amp;"} {
		check(e, e)
	}

	// Check that nonexistent named entities are returned unchanged
	check("&svadilfari;", "&svadilfari;")

	// The following examples are in the html5 specs
	check("&notit", "¬it")
	check("&notit;", "¬it;")
	check("&notin", "¬in")
	check("&notin;", "∉")

	// A similar example with a long name
	check("&notReallyAnExistingNamedCharacterReference;", "¬ReallyAnExistingNamedCharacterReference;")

	// Longest valid name
	check("&CounterClockwiseContourIntegral;", "∳")

	// Check a charref that maps to two unicode chars
	check("&acE;", "\u223e\u0333")
	check("&acE", "&acE")

	// See Python #12888
	check(strings.Repeat("&#123; ", 1050), strings.Repeat("{ ", 1050))
	// See Python #15156
	check("&Eacuteric&Eacute;ric&alphacentauri&alpha;centauri", "ÉricÉric&alphacentauriαcentauri")
	check("&co;", "&co;")
}

func TestUnescapeEdgeCases(t *testing.T) {
	tests := []struct{ input, want string }{
		// Rust's regex `\d` matches Unicode digits, which then fail to parse as u32.
		{"&#\u0661\u0662;", "\ufffd"},
		{"&#x\u0661;", "\ufffd"},
		// Rust panics here (str slice inside a multi-byte char).
		{"&amp\u00e9", "&\u00e9"},
	}
	for _, tt := range tests {
		if got := Unescape(tt.input); got != tt.want {
			t.Errorf("Unescape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
