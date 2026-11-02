package httpclient

import (
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
)

// Charset is a WHATWG character encoding, the equivalent of an encoding_rs
// Encoding.
type Charset struct {
	name string
	enc  encoding.Encoding
}

// CharsetUTF8 is the UTF-8 charset.
var CharsetUTF8 = &Charset{name: "UTF-8"}

// encodingRsNames maps the lower case WHATWG names returned by htmlindex to
// the canonical names used by encoding_rs (and in Hurl error messages).
var encodingRsNames = map[string]string{
	"utf-8":          "UTF-8",
	"ibm866":         "IBM866",
	"iso-8859-2":     "ISO-8859-2",
	"iso-8859-3":     "ISO-8859-3",
	"iso-8859-4":     "ISO-8859-4",
	"iso-8859-5":     "ISO-8859-5",
	"iso-8859-6":     "ISO-8859-6",
	"iso-8859-7":     "ISO-8859-7",
	"iso-8859-8":     "ISO-8859-8",
	"iso-8859-8-i":   "ISO-8859-8-I",
	"iso-8859-10":    "ISO-8859-10",
	"iso-8859-13":    "ISO-8859-13",
	"iso-8859-14":    "ISO-8859-14",
	"iso-8859-15":    "ISO-8859-15",
	"iso-8859-16":    "ISO-8859-16",
	"koi8-r":         "KOI8-R",
	"koi8-u":         "KOI8-U",
	"gbk":            "GBK",
	"big5":           "Big5",
	"euc-jp":         "EUC-JP",
	"iso-2022-jp":    "ISO-2022-JP",
	"shift_jis":      "Shift_JIS",
	"euc-kr":         "EUC-KR",
	"utf-16be":       "UTF-16BE",
	"utf-16le":       "UTF-16LE",
	"x-mac-cyrillic": "x-mac-cyrillic",
}

// CharsetForLabel returns the charset for a WHATWG label (like
// encoding_rs::Encoding::for_label), or nil if the label is unknown.
func CharsetForLabel(label string) *Charset {
	enc, err := htmlindex.Get(strings.TrimSpace(label))
	if err != nil {
		return nil
	}
	name, err := htmlindex.Name(enc)
	if err != nil {
		return nil
	}
	if name == "utf-8" {
		return CharsetUTF8
	}
	if n, ok := encodingRsNames[name]; ok {
		name = n
	}
	return &Charset{name: name, enc: enc}
}

// Name returns the canonical name of this charset (e.g. "UTF-8",
// "windows-1252").
func (c *Charset) Name() string { return c.name }

// Decode decodes data without BOM handling and without replacement: it
// returns false if data contains malformed sequences.
func (c *Charset) Decode(data []byte) (string, bool) {
	if c.enc == nil {
		if !utf8.Valid(data) {
			return "", false
		}
		return string(data), true
	}
	if c.enc == encoding.Replacement {
		return "", len(data) == 0
	}
	s, err := c.enc.NewDecoder().Bytes(data)
	if err != nil {
		return "", false
	}
	// x/text decoders replace malformed sequences with U+FFFD.
	if strings.ContainsRune(string(s), utf8.RuneError) {
		return "", false
	}
	return string(s), true
}
