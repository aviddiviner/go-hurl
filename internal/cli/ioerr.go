package cli

import (
	"errors"
	"strconv"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// rustIOError formats an OS error like Rust's io::Error Display, e.g.
// "No such file or directory (os error 2)".
func rustIOError(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	return capitalize(errno.Error()) + " (os error " + strconv.Itoa(int(errno)) + ")"
}

func capitalize(s string) string {
	r, size := utf8.DecodeRuneInString(s)
	if size == 0 {
		return s
	}
	return string(unicode.ToUpper(r)) + s[size:]
}
