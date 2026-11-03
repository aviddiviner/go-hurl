package report

import (
	"errors"
	"fmt"
	"io/fs"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// Error is a report error (Rust's ReportError): either an IO error on File,
// or a plain Message.
type Error struct {
	Message string
	// IO is set for an IO error on File, of kind Kind.
	IO   bool
	File string
	// Kind is the IO error kind, worded like Rust's io::ErrorKind
	// ("entity not found", "permission denied"...).
	Kind string
	Err  error
}

// NewError returns a report error with a plain message.
func NewError(message string) *Error { return &Error{Message: message} }

// NewIOError returns a report error for the IO error err on file.
func NewIOError(err error, file, message string) *Error {
	return &Error{Message: message, IO: true, File: file, Kind: ErrorKind(err), Err: err}
}

func (e *Error) Error() string {
	if e.IO {
		return fmt.Sprintf("%s %s (%s)", e.Message, e.File, e.Kind)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Err }

// ErrInvalidData is the error of reading a file that is not valid UTF-8 (an
// io::ErrorKind::InvalidData error in Rust).
var ErrInvalidData = errors.New("stream did not contain valid UTF-8")

// ErrorKind returns the Rust io::ErrorKind description of err.
func ErrorKind(err error) string {
	if errors.Is(err, ErrInvalidData) {
		return "invalid data"
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			return "entity not found"
		case errors.Is(err, fs.ErrPermission):
			return "permission denied"
		case errors.Is(err, fs.ErrExist):
			return "entity already exists"
		}
		return "other error"
	}
	// From std::sys::pal::unix::decode_error_kind.
	switch errno {
	case syscall.E2BIG:
		return "argument list too long"
	case syscall.EADDRINUSE:
		return "address in use"
	case syscall.EADDRNOTAVAIL:
		return "address not available"
	case syscall.EBUSY:
		return "resource busy"
	case syscall.ECONNABORTED:
		return "connection aborted"
	case syscall.ECONNREFUSED:
		return "connection refused"
	case syscall.ECONNRESET:
		return "connection reset"
	case syscall.EDEADLK:
		return "deadlock"
	case syscall.EDQUOT:
		return "quota exceeded"
	case syscall.EEXIST:
		return "entity already exists"
	case syscall.EFBIG:
		return "file too large"
	case syscall.EHOSTUNREACH:
		return "host unreachable"
	case syscall.EINTR:
		return "operation interrupted"
	case syscall.EINVAL:
		return "invalid input parameter"
	case syscall.EISDIR:
		return "is a directory"
	case syscall.ELOOP:
		return "filesystem loop or indirection limit (e.g. symlink loop)"
	case syscall.ENOENT:
		return "entity not found"
	case syscall.ENOMEM:
		return "out of memory"
	case syscall.ENOSPC:
		return "no storage space"
	case syscall.ENOSYS:
		return "unsupported"
	case syscall.EMLINK:
		return "too many links"
	case syscall.ENAMETOOLONG:
		return "invalid filename"
	case syscall.ENETDOWN:
		return "network down"
	case syscall.ENETUNREACH:
		return "network unreachable"
	case syscall.ENOTCONN:
		return "not connected"
	case syscall.ENOTDIR:
		return "not a directory"
	case syscall.ENOTEMPTY:
		return "directory not empty"
	case syscall.EPIPE:
		return "broken pipe"
	case syscall.EROFS:
		return "read-only filesystem or storage medium"
	case syscall.ESPIPE:
		return "seek on unseekable file"
	case syscall.ESTALE:
		return "stale network file handle"
	case syscall.ETIMEDOUT:
		return "timed out"
	case syscall.ETXTBSY:
		return "executable file busy"
	case syscall.EXDEV:
		return "cross-device link or rename"
	case syscall.EINPROGRESS:
		return "in progress"
	case syscall.EACCES, syscall.EPERM:
		return "permission denied"
	case syscall.EAGAIN:
		return "operation would block"
	}
	return "uncategorized error"
}

// OSErrorString formats err like Rust's io::Error Display for OS errors,
// e.g. "No such file or directory (os error 2)".
func OSErrorString(err error) string {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	msg := errno.Error()
	if r, size := utf8.DecodeRuneInString(msg); size > 0 {
		msg = string(unicode.ToUpper(r)) + msg[size:]
	}
	return fmt.Sprintf("%s (os error %d)", msg, int(errno))
}
