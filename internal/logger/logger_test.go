package logger

import (
	"testing"

	"github.com/aviddiviner/go-hurl/internal/term"
)

type fakeCmd string

func (c fakeCmd) ToRedactedString(secrets []string) string {
	s := string(c)
	for _, secret := range secrets {
		if s == secret {
			return "***"
		}
	}
	return s
}

func newLogger(verbosity Verbosity, secrets []string) *Logger {
	return New(LoggerOptions{Verbosity: verbosity}, term.NewStderr(term.WriteModeBuffered), secrets)
}

func TestVerboseOutput(t *testing.T) {
	l := newLogger(VerbosityVeryVerbose, nil)
	l.Debug("")
	l.Debug("Hello")
	l.DebugImportant("Response:")
	l.DebugCurl("Connected to localhost (127.0.0.1) port 8000")
	l.DebugMethodVersionOut("GET /hello HTTP/1.1")
	l.DebugHeaderOut("Host", "localhost:8000")
	l.DebugHeaderOutEnd()
	l.DebugStatusVersionIn("HTTP/1.1 200 OK")
	l.DebugHeaderIn("Content-Length", "12")
	l.DebugHeaderInEnd()
	l.Capture("token", "abc")
	l.Warning("careful")
	l.ErrorRich("boom")
	l.Info("raw")
	l.DebugCurlCmd(fakeCmd("curl 'http://localhost'"))
	want := "*\n" +
		"* Hello\n" +
		"* Response:\n" +
		"** Connected to localhost (127.0.0.1) port 8000\n" +
		"> GET /hello HTTP/1.1\n" +
		"> Host: localhost:8000\n" +
		">\n" +
		"< HTTP/1.1 200 OK\n" +
		"< Content-Length: 12\n" +
		"<\n" +
		"* token: abc\n" +
		"warning: careful\n" +
		"error: boom\n\n" +
		"raw\n" +
		"*\n" +
		"* Request can be run with the following curl command:\n" +
		"* curl 'http://localhost'\n" +
		"*\n"
	if got := l.Stderr.Buffer(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestLowVerbosity(t *testing.T) {
	l := newLogger(VerbosityLowVerbose, nil)
	l.Debug("hidden")
	l.Capture("hidden", "x")
	l.DebugHeaderIn("Shown", "yes")
	l.InfoCurlCmd(fakeCmd("curl x"))
	want := "< Shown: yes\n* Request can be run with the following curl command:\n* curl x\n"
	if got := l.Stderr.Buffer(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	l = newLogger(VerbosityNone, nil)
	l.DebugHeaderIn("Hidden", "yes")
	l.DebugMethodVersionOut("GET / HTTP/1.1")
	if got := l.Stderr.Buffer(); got != "" {
		t.Errorf("got %q", got)
	}
}

func TestSecrets(t *testing.T) {
	l := newLogger(VerbosityVerbose, []string{"s3cret"})
	l.Info("token=s3cret")
	l.Info("other=t0ken")
	l.InfoCurlCmd(fakeCmd("s3cret"))
	l.SetSecrets([]string{"s3cret", "t0ken"})
	want := "token=***\nother=***\n* Request can be run with the following curl command:\n* ***\n"
	if got := l.Stderr.Buffer(); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
