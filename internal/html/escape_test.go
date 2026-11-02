package html

import "testing"

func TestEscape(t *testing.T) {
	tests := []struct{ input, want string }{
		{"foo", "foo"},
		{"<tag>", "&lt;tag&gt;"},
		{"foo & bar", "foo &amp; bar"},
		{`string with double quote: "baz"`, "string with double quote: &quot;baz&quot;"},
		{"string with single quote: 'baz'", "string with single quote: &#x27;baz&#x27;"},
		{"", ""},
		{"&&", "&amp;&amp;"},
		{"caf\u00e9 <\U0001f600>", "caf\u00e9 &lt;\U0001f600&gt;"},
	}
	for _, tt := range tests {
		if got := Escape(tt.input); got != tt.want {
			t.Errorf("Escape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
