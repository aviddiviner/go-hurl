package text

import (
	"reflect"
	"testing"
)

func TestHello(t *testing.T) {
	var message StyledString
	message.Push("Hello ")
	message.PushWith("Bob", NewStyle().Red())
	message.Push("!")
	if got := message.Render(FormatPlain); got != "Hello Bob!" {
		t.Errorf("plain = %q", got)
	}
	if got := message.Render(FormatANSI); got != "Hello \x1b[31mBob\x1b[0m!" {
		t.Errorf("ansi = %q", got)
	}
}

func TestPush(t *testing.T) {
	var message StyledString
	message.Push("Hello")
	message.Push(" ")
	message.PushWith("Bob", NewStyle().Red())
	message.Push("!")
	want := StyledString{tokens: []token{
		{content: "Hello ", style: NewStyle()},
		{content: "Bob", style: NewStyle().Red()},
		{content: "!", style: NewStyle()},
	}}
	if !reflect.DeepEqual(message, want) {
		t.Errorf("got %#v", message)
	}
}

func TestAppend(t *testing.T) {
	var message1 StyledString
	message1.Push("Hello ")
	message1.PushWith("Bob", NewStyle().Red())
	message1.Push("!")
	var message2 StyledString
	message2.Push("Hi ")
	message2.PushWith("Bill", NewStyle().Red())
	message2.Push("!")

	var messages StyledString
	messages.Push("Hello ")
	messages.PushWith("Bob", NewStyle().Red())
	messages.Push("!")
	messages.Push("Hi ")
	messages.PushWith("Bill", NewStyle().Red())
	messages.Push("!")

	message1.Append(message2)
	if !reflect.DeepEqual(message1, messages) {
		t.Errorf("got %#v", message1)
	}
}

func TestSplit(t *testing.T) {
	var line StyledString
	line.Push("Hello,Hi,")
	line.PushWith("Hola", NewStyle().Red())
	line.Push(",Bye,")
	line.PushWith("Adios", NewStyle().Red())

	var item0, item1, item2, item3, item4 StyledString
	item0.Push("Hello")
	item1.Push("Hi")
	item2.PushWith("Hola", NewStyle().Red())
	item3.Push("Bye")
	item4.PushWith("Adios", NewStyle().Red())
	if got, want := line.Split(','), []StyledString{item0, item1, item2, item3, item4}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}

	// Empty items
	line = StyledString{}
	line.Push("0,,2,")
	var e0, e1, e2, e3 StyledString
	e0.Push("0")
	e2.Push("2")
	if got, want := line.Split(','), []StyledString{e0, e1, e2, e3}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v", got)
	}
}

func TestEndsWith(t *testing.T) {
	var line StyledString
	line.Push("Hello,Hi,")
	if !line.EndsWith(",") {
		t.Error("expected to end with ','")
	}
	if line.EndsWith("\n") {
		t.Error("expected not to end with '\\n'")
	}
}

// Expected values are the colored crate's output ("foo".red().bold(),
// "bar".bold(), ...).
func TestCompareWithCrateColored(t *testing.T) {
	tests := []struct {
		content string
		style   Style
		want    string
	}{
		{"foo", NewStyle().Red().Bold(), "\x1b[1;31mfoo\x1b[0m"},
		{"bar", NewStyle().Bold(), "\x1b[1mbar\x1b[0m"},
		{"x", NewStyle().Blue().Bold(), "\x1b[1;34mx\x1b[0m"},
		{"x", NewStyle().BrightBlack(), "\x1b[90mx\x1b[0m"},
		{"x", NewStyle().Cyan(), "\x1b[36mx\x1b[0m"},
		{"x", NewStyle().Green(), "\x1b[32mx\x1b[0m"},
		{"x", NewStyle().Magenta(), "\x1b[35mx\x1b[0m"},
		{"x", NewStyle().Purple(), "\x1b[35mx\x1b[0m"},
		{"x", NewStyle().Yellow().Bold(), "\x1b[1;33mx\x1b[0m"},
		{"", NewStyle().Red(), "\x1b[31m\x1b[0m"},
		{"plain", NewStyle(), "plain"},
		// Inner resets re-apply the style (colored's escape_inner_reset_sequences).
		{"a\x1b[0mb", NewStyle().Red(), "\x1b[31ma\x1b[0m\x1b[31mb\x1b[0m"},
	}
	for _, tt := range tests {
		var s StyledString
		s.PushWith(tt.content, tt.style)
		if got := s.Render(FormatANSI); got != tt.want {
			t.Errorf("Render(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

func TestWrapSinglePlainToken(t *testing.T) {
	var line StyledString
	line.Push("aaaabbbbcccc")
	var wrapped StyledString
	wrapped.Push("aaaa\nbbbb\ncccc")
	if got := line.Wrap(4); !reflect.DeepEqual(got, wrapped) {
		t.Errorf("got %#v", got)
	}
	if line.Len() != 12 {
		t.Errorf("Len = %d", line.Len())
	}
	if n := line.Wrap(4).Len(); n != 14 {
		t.Errorf("wrapped Len = %d", n)
	}
}

func TestWrapSingleStyledToken(t *testing.T) {
	var line StyledString
	line.PushWith("aaaabbbbcccc", NewStyle().Blue())
	var wrapped StyledString
	wrapped.PushWith("aaaa", NewStyle().Blue())
	wrapped.Push("\n")
	wrapped.PushWith("bbbb", NewStyle().Blue())
	wrapped.Push("\n")
	wrapped.PushWith("cccc", NewStyle().Blue())
	if got := line.Wrap(4); !reflect.DeepEqual(got, wrapped) {
		t.Errorf("got %#v", got)
	}
	if line.Len() != 12 {
		t.Errorf("Len = %d", line.Len())
	}
	if n := line.Wrap(4).Len(); n != 14 {
		t.Errorf("wrapped Len = %d", n)
	}
}

func TestWrapMultiStyledToken(t *testing.T) {
	var line StyledString
	line.PushWith("aaa", NewStyle().Blue())
	line.PushWith("ab", NewStyle().Green())
	line.PushWith("bbbccc", NewStyle().Yellow())
	line.PushWith("cee", NewStyle().Purple())

	var wrapped StyledString
	wrapped.PushWith("aaa", NewStyle().Blue())
	wrapped.PushWith("a", NewStyle().Green())
	wrapped.Push("\n")
	wrapped.PushWith("b", NewStyle().Green())
	wrapped.PushWith("bbb", NewStyle().Yellow())
	wrapped.Push("\n")
	wrapped.PushWith("ccc", NewStyle().Yellow())
	wrapped.PushWith("c", NewStyle().Purple())
	wrapped.Push("\n")
	wrapped.PushWith("ee", NewStyle().Purple())
	if got := line.Wrap(4); !reflect.DeepEqual(got, wrapped) {
		t.Errorf("got %#v", got)
	}
}

func TestClone(t *testing.T) {
	var a StyledString
	a.Push("a")
	b := a.Clone()
	b.Push("b")
	if a.Render(FormatPlain) != "a" || b.Render(FormatPlain) != "ab" {
		t.Errorf("a = %q, b = %q", a.Render(FormatPlain), b.Render(FormatPlain))
	}
}
