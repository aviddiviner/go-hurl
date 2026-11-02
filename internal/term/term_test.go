package term

import "testing"

func TestBufferedStdout(t *testing.T) {
	stdout := NewStdout(WriteModeBuffered)
	for _, s := range []string{"Hello", " ", "World!"} {
		if err := stdout.WriteAll([]byte(s)); err != nil {
			t.Fatal(err)
		}
	}
	if got := string(stdout.Buffer()); got != "Hello World!" {
		t.Errorf("got %q", got)
	}
}

func TestBufferedStderr(t *testing.T) {
	stderr := NewStderr(WriteModeBuffered)
	stderr.Eprintln("toto")
	stderr.SetProgressBar("some progress...\r")
	stderr.Eprintln("tutu")
	if got := stderr.Buffer(); got != "toto\ntutu\n" {
		t.Errorf("got %q", got)
	}
}
