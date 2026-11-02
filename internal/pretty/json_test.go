package pretty

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"
)

func formatString(t *testing.T, input []byte, color Color) (string, error) {
	t.Helper()
	var out bytes.Buffer
	if err := Format(input, color, &out); err != nil {
		return "", err
	}
	if !utf8.Valid(out.Bytes()) {
		t.Fatalf("output of %q is not valid UTF-8", input)
	}
	return out.String(), nil
}

func TestParseNumberOK(t *testing.T) {
	tests := []struct{ input, want string }{
		// Parse some integers
		{"1234xxxx", "1234"},
		{"42", "42"},
		{"1233456787766677889778998789988", "1233456787766677889778998789988"},
		{"0000", "0"},
		{"0", "0"},
		{"-0", "-0"},
		{"012345", "0"},
		{"0abcdef", "0"},
		{"-10256", "-10256"},
		{"-012344", "-0"},
		// Parse real (with fraction)
		{"1.000", "1.000"},
		{"1.7b", "1.7"},
	}
	for _, tt := range tests {
		var out bytes.Buffer
		f := &formatter{input: []byte(tt.input), color: NoColor, out: &out}
		if err := f.parseNumber(); err != nil {
			t.Errorf("parseNumber(%q): %v", tt.input, err)
			continue
		}
		if got := out.String(); got != tt.want {
			t.Errorf("parseNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseNumberFailed(t *testing.T) {
	for _, input := range []string{"1.", "78980.a", "abc"} {
		var out bytes.Buffer
		f := &formatter{input: []byte(input), color: NoColor, out: &out}
		if err := f.parseNumber(); err == nil {
			t.Errorf("parseNumber(%q): expected error", input)
		}
	}
}

func assertAgainstStd(t *testing.T, input []byte, n int) {
	t.Helper()
	// We pass the full buffer to the parser, with some trailing bytes.
	f := &formatter{input: input, color: NoColor}
	err := f.nextUTF8Char()

	// We test against a buffer without trailing.
	if utf8.Valid(input[:n]) {
		if err != nil {
			t.Fatalf("nextUTF8Char(% x): %v", input, err)
		}
		if f.pos != n {
			t.Fatalf("nextUTF8Char(% x): pos = %d, want %d", input, f.pos, n)
		}
	} else if err == nil {
		t.Fatalf("nextUTF8Char(% x): expected error", input)
	}
}

func TestReadOneByteToUTF8(t *testing.T) {
	// Iterate through all 1-byte UTF-8 bytes, even invalid
	for b := range 0x100 {
		assertAgainstStd(t, []byte{byte(b), 'x', 'x', 'x'}, 1)
	}
}

func TestReadTwoBytesToUTF8(t *testing.T) {
	// Iterate through all UTF-8 2-bytes: C0..=DF 80..=BF
	// It may contains invalid ones (overlong for instance).
	for b1 := 0xC0; b1 <= 0xDF; b1++ {
		for b2 := 0x80; b2 <= 0xBF; b2++ {
			assertAgainstStd(t, []byte{byte(b1), byte(b2), 'x', 'x', 'x'}, 2)
		}
	}
}

func TestReadThreeBytesToUTF8(t *testing.T) {
	// Iterate through all UTF-8 3-bytes: E0..=EF 80..=BF 80..=BF
	// It may contains invalid ones (overlong for instance).
	// The Rust test iterates over F0..=F7 for the first byte; we test both ranges.
	for _, r := range [][2]int{{0xE0, 0xEF}, {0xF0, 0xF7}} {
		for b1 := r[0]; b1 <= r[1]; b1++ {
			for b2 := 0x80; b2 <= 0xBF; b2++ {
				for b3 := 0x80; b3 <= 0xBF; b3++ {
					assertAgainstStd(t, []byte{byte(b1), byte(b2), byte(b3), 'x', 'x', 'x'}, 3)
				}
			}
		}
	}
}

func TestReadFourBytesToUTF8(t *testing.T) {
	// Iterate through all UTF-8 4-bytes: F0..=F7 80..=BF 80..=BF 80..=BF
	// It may contains invalid ones (overlong for instance).
	for b1 := 0xF0; b1 <= 0xF7; b1++ {
		for b2 := 0x80; b2 <= 0xBF; b2++ {
			for b3 := 0x80; b3 <= 0xBF; b3++ {
				for b4 := 0x80; b4 <= 0xBF; b4++ {
					assertAgainstStd(t, []byte{byte(b1), byte(b2), byte(b3), byte(b4), 'x', 'x', 'x'}, 4)
				}
			}
		}
	}
}

func TestFormatValidJSON(t *testing.T) {
	tests := []struct{ input, want string }{
		{
			`{"strings":{"english":"Hello, world!","chinese":"你好，世界","japanese":"こんにちは世界","korean":"안녕하세요 세계","arabic":"مرحبا بالعالم","hindi":"नमस्ते दुनिया","russian":"Привет, мир","greek":"Γειά σου Κόσμε","hebrew":"שלום עולם","accented":"Curaçao, naïve, façade, jalapeño"},"numbers":{"zero":0,"positive_int":42,"negative_int":-42,"large_int":1234567890123456789,"small_float":0.000123,"negative_float":-3.14159,"large_float":1.7976931348623157e308,"smallest_float":5e-324,"sci_notation_positive":6.022e23,"sci_notation_negative":-2.99792458e8},"booleans":{"isActive":true,"isDeleted":false},"emojis":{"happy":"😀","sad":"😢","fire":"🔥","rocket":"🚀","earth":"🌍","heart":"❤️","multi":"👩‍💻🧑🏽‍🚀👨‍👩‍👧‍👦"},"nothing":null}`,
			`{
  "strings": {
    "english": "Hello, world!",
    "chinese": "你好，世界",
    "japanese": "こんにちは世界",
    "korean": "안녕하세요 세계",
    "arabic": "مرحبا بالعالم",
    "hindi": "नमस्ते दुनिया",
    "russian": "Привет, мир",
    "greek": "Γειά σου Κόσμε",
    "hebrew": "שלום עולם",
    "accented": "Curaçao, naïve, façade, jalapeño"
  },
  "numbers": {
    "zero": 0,
    "positive_int": 42,
    "negative_int": -42,
    "large_int": 1234567890123456789,
    "small_float": 0.000123,
    "negative_float": -3.14159,
    "large_float": 1.7976931348623157e308,
    "smallest_float": 5e-324,
    "sci_notation_positive": 6.022e23,
    "sci_notation_negative": -2.99792458e8
  },
  "booleans": {
    "isActive": true,
    "isDeleted": false
  },
  "emojis": {
    "happy": "😀",
    "sad": "😢",
    "fire": "🔥",
    "rocket": "🚀",
    "earth": "🌍",
    "heart": "❤️",
    "multi": "👩‍💻🧑🏽‍🚀👨‍👩‍👧‍👦"
  },
  "nothing": null
}
`,
		},
		{
			`{
		"numbers": [333333333.33333329, 1E30, 4.50,
					2e-3, 0.000000000000000000000000001, -0],
		"string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
		"literals": [null, true, false]
	}`,
			`{
  "numbers": [
    333333333.33333329,
    1E30,
    4.50,
    2e-3,
    0.000000000000000000000000001,
    -0
  ],
  "string": "\u20ac$\u000F\u000aA'\u0042\u0022\u005c\\\"\/",
  "literals": [
    null,
    true,
    false
  ]
}
`,
		},
		{
			`{
		"\u20ac": "Euro Sign",
		"\r": "Carriage Return",
		"\ufb33": "Hebrew Letter Dalet With Dagesh",
		"1": "One",
		"\ud83d\ude00": "Emoji: Grinning Face",
		"\u0080": "Control",
		"\u00f6": "Latin Small Letter O With Diaeresis"
	}`,
			`{
  "\u20ac": "Euro Sign",
  "\r": "Carriage Return",
  "\ufb33": "Hebrew Letter Dalet With Dagesh",
  "1": "One",
  "\ud83d\ude00": "Emoji: Grinning Face",
  "\u0080": "Control",
  "\u00f6": "Latin Small Letter O With Diaeresis"
}
`,
		},
		{
			" [ -9223372036854775808 , 9223372036854775807 ] ",
			`[
  -9223372036854775808,
  9223372036854775807
]
`,
		},
		{
			` { "0" : 0 , "1" : 1 , "0" : 0 }`,
			`{
  "0": 0,
  "1": 1,
  "0": 0
}
`,
		},
		{
			"[0,1,[12,22,[34,[45,56],7]],[]]",
			`[
  0,
  1,
  [
    12,
    22,
    [
      34,
      [
        45,
        56
      ],
      7
    ]
  ],
  []
]
`,
		},
		{
			`{"a":[{"b":[]},{},[2]]}`,
			`{
  "a": [
    {
      "b": []
    },
    {},
    [
      2
    ]
  ]
}
`,
		},
		{
			" { }",
			"{}\n",
		},
		{
			`{"X":{},"Y":{},"X":{}} `,
			`{
  "X": {},
  "Y": {},
  "X": {}
}
`,
		},
	}
	for _, tt := range tests {
		got, err := formatString(t, []byte(tt.input), NoColor)
		if err != nil {
			t.Errorf("Format(%q): %v", tt.input, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Format(%q) =\n%s\nwant\n%s", tt.input, got, tt.want)
		}
	}
}

func TestFormatColor(t *testing.T) {
	input := "\xef\xbb\xbf {\"a\":[1,true,false,null,\"x\",[]],\"b\":{}}"
	want := "\x1b[1;39m{\x1b[0m\n" +
		"  \x1b[1;34m\"a\"\x1b[0m\x1b[1;39m:\x1b[0m \x1b[1;39m[\x1b[0m\n" +
		"    \x1b[0;36m1\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"    \x1b[0;33mtrue\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"    \x1b[0;33mfalse\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"    \x1b[0;35mnull\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"    \x1b[0;32m\"x\"\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"    \x1b[1;39m[]\x1b[0m\n" +
		"  \x1b[1;39m]\x1b[0m\x1b[1;39m,\x1b[0m\n" +
		"  \x1b[1;34m\"b\"\x1b[0m\x1b[1;39m:\x1b[0m \x1b[1;39m{}\x1b[0m\n" +
		"\x1b[1;39m}\x1b[0m\n"
	got, err := formatString(t, []byte(input), Ansi)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaxIndentLevel(t *testing.T) {
	if _, err := formatString(t, []byte(strings.Repeat("[", 100)+strings.Repeat("]", 100)), NoColor); err != nil {
		t.Fatalf("100 levels: %v", err)
	}
	_, err := formatString(t, []byte(strings.Repeat("[", 101)), NoColor)
	if want := (MaxIndentLevelError{100, 101}); err != want {
		t.Fatalf("err = %#v, want %#v", err, want)
	}
	if got, want := err.Error(), "maximum indent level 100 at offset 101"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

// From Go Standard library <https://github.com/golang/go/blob/master/src/encoding/json/jsontext/decode_test.go>
func TestErrorOnInvalid(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr error
		wantMsg string
	}{
		{"Invalid start", " #", InvalidByteError{35, 1}, "invalid byte 0x23/'#' at offset 1"},
		{"Extra comma", " null , null ", InvalidByteError{44, 6}, "invalid byte 0x2c/',' at offset 6"},
		{"Truncated null", " nul", EOFError{}, "unexpected end of file"},
		{"Invalid null", "nulL", InvalidByteError{76, 3}, "invalid byte 0x4c/'L' at offset 3"},
		{"Truncated false", "fals", EOFError{}, "unexpected end of file"},
		{"Invalid false", "falsE", InvalidByteError{69, 4}, "invalid byte 0x45/'E' at offset 4"},
		{"Truncated true", "tru", EOFError{}, "unexpected end of file"},
		{"Invalid true", "truE", InvalidByteError{69, 3}, "invalid byte 0x45/'E' at offset 3"},
		{"Invalid string", `"start`, EOFError{}, "unexpected end of file"},
		{"Truncated string", `"start`, EOFError{}, "unexpected end of file"},
		{"Invalid string", "\"ok \x00", InvalidByteError{0, 4}, "invalid byte 0x00 at offset 4"},
		{"Invalid UTF-8", "\"living\xde\xad\xbe\xef\"", InvalidUTF8Error{[4]byte{0xbe, 0xef, 0, 0}, 2, 9}, "invalid UTF-8 2 bytes 0xbe 0xef at offset 9"},
		{"Truncated number", "0.", EOFError{}, "unexpected end of file"},
		{"Invalid number", "0.e", InvalidByteError{101, 2}, "invalid byte 0x65/'e' at offset 2"},
		{"Truncated number after start", "{", EOFError{}, "unexpected end of file"},
		{"Truncated number after start", "{", EOFError{}, "unexpected end of file"},
		{"Truncated number after name", `{"0"`, EOFError{}, "unexpected end of file"},
		{"Truncated number after colon", `{"0":`, EOFError{}, "unexpected end of file"},
		{"Truncated number after value", `{"0":0`, EOFError{}, "unexpected end of file"},
		{"Truncated number after comma", `{"0":0,`, EOFError{}, "unexpected end of file"},
		{"Invalid object missing colon", ` { "fizz" "buzz" } `, InvalidByteError{34, 10}, "invalid byte 0x22/'\"' at offset 10"},
		{"Invalid object missing colon got comma", ` { "fizz" , "buzz" } `, InvalidByteError{44, 10}, "invalid byte 0x2c/',' at offset 10"},
		{"Invalid object missing colon got hash", ` { "fizz" # "buzz" } `, InvalidByteError{35, 10}, "invalid byte 0x23/'#' at offset 10"},
		{"Invalid object missing comma", ` { "fizz" : "buzz" "gazz" } `, InvalidByteError{34, 19}, "invalid byte 0x22/'\"' at offset 19"},
		{"Invalid object missing comma got colon", ` { "fizz" : "buzz" : "gazz" } `, InvalidByteError{58, 19}, "invalid byte 0x3a/':' at offset 19"},
		{"Invalid object missing comma got hash", ` { "fizz" : "buzz" # "gazz" } `, InvalidByteError{35, 19}, "invalid byte 0x23/'#' at offset 19"},
		{"Invalid object extra comma after start", ` { , } `, InvalidByteError{44, 3}, "invalid byte 0x2c/',' at offset 3"},
		{"Invalid object extra comma after value", ` { "fizz" : "buzz" , } `, InvalidByteError{125, 21}, "invalid byte 0x7d/'}' at offset 21"},
		{"Invalid object invalid name got null", ` { null : null } `, InvalidByteError{110, 3}, "invalid byte 0x6e/'n' at offset 3"},
		{"Invalid object invalid name got false", ` { false : false } `, InvalidByteError{102, 3}, "invalid byte 0x66/'f' at offset 3"},
		{"Invalid object invalid name got true", ` { true : true } `, InvalidByteError{116, 3}, "invalid byte 0x74/'t' at offset 3"},
		{"Invalid object invalid name got number", ` { 0 : 0 } `, InvalidByteError{48, 3}, "invalid byte 0x30/'0' at offset 3"},
		{"Invalid object invalid name got object", ` { {} : {} } `, InvalidByteError{123, 3}, "invalid byte 0x7b/'{' at offset 3"},
		{"Invalid object invalid name got array", ` { [] : [] } `, InvalidByteError{91, 3}, "invalid byte 0x5b/'[' at offset 3"},
		{"Invalid object mismatching delim", ` { ] `, InvalidByteError{93, 3}, "invalid byte 0x5d/']' at offset 3"},
		{"Invalid object mismatching delim", ` { ] `, InvalidByteError{93, 3}, "invalid byte 0x5d/']' at offset 3"},
		{"Truncated array after start", "[", EOFError{}, "unexpected end of file"},
		{"Truncated array after value", "[0", EOFError{}, "unexpected end of file"},
		{"Truncated array after comma", "[0,", EOFError{}, "unexpected end of file"},
		{"Invalid array missing comma", `[ "fizz" "buzz" ] `, InvalidByteError{34, 9}, "invalid byte 0x22/'\"' at offset 9"},
		{"Invalid array mismatching delim", " [ } ", InvalidByteError{125, 3}, "invalid byte 0x7d/'}' at offset 3"},
		{"Invalid delim after top level", ` "", `, InvalidByteError{44, 3}, "invalid byte 0x2c/',' at offset 3"},
		{"Invalid delim after begin object", "{:", InvalidByteError{58, 1}, "invalid byte 0x3a/':' at offset 1"},
		{"Invalid delim after object name", `{"",`, InvalidByteError{44, 3}, "invalid byte 0x2c/',' at offset 3"},
		{"Valid delim after object name", `{"":`, EOFError{}, "unexpected end of file"},
		{"Invalid delim after object value", `{"":"":`, InvalidByteError{58, 6}, "invalid byte 0x3a/':' at offset 6"},
		{"Valid delim after object value", `{"":"",`, EOFError{}, "unexpected end of file"},
		{"Invalid delim after begin array", "[,", InvalidByteError{44, 1}, "invalid byte 0x2c/',' at offset 1"},
		{"Invalid delim after array value", `["":`, InvalidByteError{58, 3}, "invalid byte 0x3a/':' at offset 3"},
		{"Valid delim after array value", `["",`, EOFError{}, "unexpected end of file"},
		{"Error position", "\"a\xff000\"", InvalidUTF8Error{[4]byte{255, 48, 48, 48}, 4, 6}, "invalid UTF-8 4 bytes 0xff 0x30 0x30 0x30 at offset 6"},
		{"Error position /0", " [ \"a\xff111\" ] ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 9}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 9"},
		{"Error position /1", " [ \"a1\" , \"b\xff111\" ] ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 16}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 16"},
		{"Error position /0/0", " [ [ \"a\xff222\" ] ] ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 11}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 11"},
		{"Error position /1/0", " [ \"a1\" , [ \"a\xff222\" ] ] ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 18}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 18"},
		{"Error position /0/1", " [ [ \"a2\" , \"b\xff222\" ] ] ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 18}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 18"},
		{"Error position /1/1", " [ \"a1\" , [ \"a2\" , \"b\xff222\" ] ] ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 25}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 25"},
		{"Error position /a1-", " { \"a\xff111\" : \"b1\" } ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 9}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 9"},
		{"Error position /a1", " { \"a1\" : \"b\xff111\" } ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 16}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 16"},
		{"Error position /c1-", " { \"a1\" : \"b1\" , \"c\xff111\" : \"d1\" } ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 23}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 23"},
		{"Error position /c1", "{ \"a1\" : \"b1\" , \"c1\" : \"d\xff111\" } ", InvalidUTF8Error{[4]byte{255, 49, 49, 49}, 4, 29}, "invalid UTF-8 4 bytes 0xff 0x31 0x31 0x31 at offset 29"},
		{"Error position /a1/a2-", " { \"a1\" : { \"a\xff222\" : \"b2\" } } ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 18}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 18"},
		{"Error position /a1/a2", " { \"a1\" : { \"a2\" : \"b\xff222\" } } ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 25}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 25"},
		{"Error position /a1/c2-", " { \"a1\" : { \"a2\" : \"b2\" , \"c\xff222\" : \"d2\" } } ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 32}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 32"},
		{"Error position /a1/c2", " { \"a1\" : { \"a2\" : \"b2\" , \"c2\" : \"d\xff222\" } } ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 39}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 39"},
		{"Error position /1/a2", " [ \"a1\" , { \"a2\" : \"b\xff222\" } ] ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 25}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 25"},
		{"Error position /c1/1", " { \"a1\" : \"b1\" , \"c1\" : [ \"a2\" , \"b\xff222\" ] } ", InvalidUTF8Error{[4]byte{255, 50, 50, 50}, 4, 39}, "invalid UTF-8 4 bytes 0xff 0x32 0x32 0x32 at offset 39"},
		{"Error position /0/a1/1/c3/1", " [ { \"a1\" : [ \"a2\" , { \"a3\" : \"b3\" , \"c3\" : [ \"a4\" , \"b\xff444\" ] } ] } ] ", InvalidUTF8Error{[4]byte{255, 52, 52, 52}, 4, 59}, "invalid UTF-8 4 bytes 0xff 0x34 0x34 0x34 at offset 59"},
	}
	for _, tt := range tests {
		_, err := formatString(t, []byte(tt.input), NoColor)
		if err != tt.wantErr {
			t.Errorf("%s: err = %#v, want %#v", tt.name, err, tt.wantErr)
			continue
		}
		if got := err.Error(); got != tt.wantMsg {
			t.Errorf("%s: message = %q, want %q", tt.name, got, tt.wantMsg)
		}
	}
}
