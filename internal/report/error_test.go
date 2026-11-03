package report

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestError(t *testing.T) {
	_, err := os.ReadFile(filepath.Join(t.TempDir(), "missing"))
	tests := []struct {
		err  *Error
		want string
	}{
		{NewError("Invalid TAP Header <foo>"), "Invalid TAP Header <foo>"},
		{NewIOError(err, "a/b.json", "Issue reading JSON report"), "Issue reading JSON report a/b.json (entity not found)"},
		{NewIOError(fmt.Errorf("wrap: %w", ErrInvalidData), "a.tap", "Issue reading TAP report"), "Issue reading TAP report a.tap (invalid data)"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
	want := "No such file or directory (os error 2)"
	if runtime.GOOS == "windows" {
		want = "The system cannot find the file specified. (os error 2)"
	}
	if got := OSErrorString(err); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
