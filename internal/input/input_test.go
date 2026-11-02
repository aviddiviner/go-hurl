package input

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestStripBOM(t *testing.T) {
	tests := []struct{ in, want []byte }{
		{[]byte{}, []byte{}},
		{[]byte{0xef, 0xbb, 0xbf, 0x68, 0x65, 0x6c, 0x6c, 0x6f}, []byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}},
		{[]byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}, []byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}},
	}
	for _, tt := range tests {
		if got := stripBOM(tt.in); !bytes.Equal(got, tt.want) {
			t.Errorf("stripBOM(%x) = %x, want %x", tt.in, got, tt.want)
		}
	}
}

func TestStringFromUTF8BOM(t *testing.T) {
	for _, tt := range []struct {
		in   []byte
		want string
	}{
		{nil, ""},
		{[]byte{0xef, 0xbb, 0xbf, 0x68, 0x65, 0x6c, 0x6c, 0x6f}, "hello"},
		{[]byte{0x68, 0x65, 0x6c, 0x6c, 0x6f}, "hello"},
	} {
		got, err := stringFromUTF8(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("stringFromUTF8(%x) = %q, %v", tt.in, got, err)
		}
	}
	_, err := stringFromUTF8([]byte{0xef})
	if err == nil || err.Error() != "incomplete utf-8 byte sequence from index 0" {
		t.Errorf("got %v", err)
	}
}

// Expected messages are from Rust's String::from_utf8 (rustc 1.94).
func TestUTF8Errors(t *testing.T) {
	tests := []struct {
		in   []byte
		want string
	}{
		{[]byte{0xef}, "incomplete utf-8 byte sequence from index 0"},
		{[]byte{0x68, 0xff}, "invalid utf-8 sequence of 1 bytes from index 1"},
		{[]byte{0xe2, 0x28, 0xa1}, "invalid utf-8 sequence of 1 bytes from index 0"},
		{[]byte{0xf0, 0x90, 0x28}, "invalid utf-8 sequence of 2 bytes from index 0"},
		{[]byte{0xf0, 0x90, 0x80, 0x28}, "invalid utf-8 sequence of 3 bytes from index 0"},
		{[]byte{0xf0, 0x90, 0x80}, "incomplete utf-8 byte sequence from index 0"},
		{[]byte{0x61, 0xe9, 0x20}, "invalid utf-8 sequence of 1 bytes from index 1"},
		{[]byte{0xed, 0xa0, 0x80}, "invalid utf-8 sequence of 1 bytes from index 0"},
		{[]byte{0xc0, 0x80}, "invalid utf-8 sequence of 1 bytes from index 0"},
		{[]byte{0xf5, 0x80}, "invalid utf-8 sequence of 1 bytes from index 0"},
		{[]byte{0xe0, 0x80}, "invalid utf-8 sequence of 1 bytes from index 0"},
		{[]byte{0x61, 0x62, 0xe2, 0x82}, "incomplete utf-8 byte sequence from index 2"},
		{[]byte{0x80}, "invalid utf-8 sequence of 1 bytes from index 0"},
	}
	for _, tt := range tests {
		err := validateUTF8(tt.in)
		if err == nil || err.Error() != tt.want {
			t.Errorf("validateUTF8(%x) = %v, want %q", tt.in, err, tt.want)
		}
	}
	if err := validateUTF8([]byte("héllo ✈ 𝄞")); err != nil {
		t.Errorf("valid input: %v", err)
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.hurl")
	if err := os.WriteFile(path, []byte("\xef\xbb\xbfGET http://foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	in := New(path)
	if in.String() != path {
		t.Errorf("String() = %q", in.String())
	}
	if got, err := in.ReadToString(); err != nil || got != "GET http://foo\n" {
		t.Errorf("ReadToString() = %q, %v", got, err)
	}

	// Messages from Rust's io::Error Display (Unix errno values).
	if runtime.GOOS == "windows" {
		return
	}
	_, err := New(filepath.Join(dir, "missing")).ReadToString()
	if err == nil || err.Error() != "No such file or directory (os error 2)" || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: %v", err)
	}
	_, err = New(dir).ReadToString()
	if err == nil || err.Error() != "Is a directory (os error 21)" {
		t.Errorf("directory: %v", err)
	}
}

func TestStdin(t *testing.T) {
	in, err := fromReader(strings.NewReader("\xef\xbb\xbfGET http://foo"))
	if err != nil {
		t.Fatal(err)
	}
	if in.String() != "-" {
		t.Errorf("String() = %q", in.String())
	}
	// Stdin is cached as is: Hurl doesn't strip the BOM there.
	for range 2 {
		if got, err := in.ReadToString(); err != nil || got != "\xef\xbb\xbfGET http://foo" {
			t.Errorf("ReadToString() = %q, %v", got, err)
		}
	}
	if _, err := fromReader(strings.NewReader("\xff")); err == nil || err.Error() != "stream did not contain valid UTF-8" {
		t.Errorf("got %v", err)
	}
}
