// Package term wraps standard output and standard error: bytes are either
// written immediately or kept in an internal buffer. Ported from
// hurl/src/util/term.rs.
package term

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/term"
)

// WriteMode is the way to write on standard output and error.
type WriteMode int

const (
	// WriteModeImmediate prints messages immediately.
	WriteModeImmediate WriteMode = iota
	// WriteModeBuffered saves messages to an internal buffer, retrieved with
	// Stdout.Buffer / Stderr.Buffer.
	WriteModeBuffered
)

// Stdout is an indirection for standard output.
type Stdout struct {
	mode   WriteMode
	buffer bytes.Buffer
}

func NewStdout(mode WriteMode) *Stdout { return &Stdout{mode: mode} }

// WriteAll writes the entire buf to standard output (or the buffer).
func (s *Stdout) WriteAll(buf []byte) error {
	if s.mode == WriteModeBuffered {
		s.buffer.Write(buf)
		return nil
	}
	return writeStdout(buf)
}

// Buffer returns the buffered standard output.
func (s *Stdout) Buffer() []byte { return s.buffer.Bytes() }

// IsTerminal reports whether standard output is a terminal.
func (s *Stdout) IsTerminal() bool { return isTerminal(os.Stdout) }

func writeStdout(buf []byte) error {
	// On a Windows console, non UTF-8 bytes can't be written: as Hurl does,
	// we print a lossy UTF-8 conversion.
	if runtime.GOOS == "windows" && isTerminal(os.Stdout) {
		_, err := fmt.Fprintln(os.Stdout, strings.ToValidUTF8(string(buf), "\uFFFD"))
		return err
	}
	_, err := os.Stdout.Write(buf)
	return err
}

func isTerminal(f *os.File) bool { return term.IsTerminal(int(f.Fd())) }

// Stderr is an indirection for standard error.
//
// An optional progress bar can be used to report temporary progress to the
// user. It's always printed as the last lines of standard error. In
// WriteModeBuffered, the progress bar is not saved in the buffer.
type Stderr struct {
	mode        WriteMode
	buffer      strings.Builder
	progressBar string
}

func NewStderr(mode WriteMode) *Stderr { return &Stderr{mode: mode} }

// Mode returns the write mode of this standard error.
func (s *Stderr) Mode() WriteMode { return s.mode }

// Eprintln prints message to standard error, with a newline.
func (s *Stderr) Eprintln(message string) {
	s.write(message + "\n")
}

// Eprint prints message to standard error.
func (s *Stderr) Eprint(message string) {
	s.write(message)
}

func (s *Stderr) write(message string) {
	if s.mode == WriteModeBuffered {
		s.buffer.WriteString(message)
		return
	}
	hasProgress := s.progressBar != ""
	if hasProgress {
		s.rewindCursor()
	}
	os.Stderr.WriteString(message)
	if hasProgress {
		os.Stderr.WriteString(s.progressBar)
	}
}

// SetProgressBar sets the progress bar (only in WriteModeImmediate).
func (s *Stderr) SetProgressBar(progress string) {
	if s.mode != WriteModeImmediate {
		return
	}
	s.progressBar = progress
	os.Stderr.WriteString(s.progressBar)
}

// ClearProgressBar clears the progress bar (only in WriteModeImmediate).
func (s *Stderr) ClearProgressBar() {
	s.rewindCursor()
	s.progressBar = ""
}

// Buffer returns the buffered standard error.
func (s *Stderr) Buffer() string { return s.buffer.String() }

// SetBuffer replaces the buffered standard error.
func (s *Stderr) SetBuffer(buffer string) {
	s.buffer.Reset()
	s.buffer.WriteString(buffer)
}

// rewindCursor clears any progress and moves the terminal cursor back to the
// position of the last "real" message (only in WriteModeImmediate).
func (s *Stderr) rewindCursor() {
	if s.progressBar == "" || s.mode != WriteModeImmediate {
		return
	}
	// We count "\n" rather than lines: we don't want to go up for a single
	// char. ESC[1A is "cursor up", ESC[K is "erase in line".
	lines := strings.Count(s.progressBar, "\n")
	if lines > 0 {
		os.Stderr.WriteString(strings.Repeat("\x1b[1A\x1b[K", lines))
	} else {
		os.Stderr.WriteString("\x1b[K")
	}
}
