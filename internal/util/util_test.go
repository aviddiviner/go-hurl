package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilenameAllowedAccessWithoutUserFileRoot(t *testing.T) {
	// $ cd /dir
	// $ hurl test.hurl
	ctx := NewContextDir("/dir", "")
	checkAccess(t, ctx,
		[]string{"foo.bin", "/dir/foo.bin", "a/foo.bin", "a/b/foo.bin", "../dir/a/b/foo.bin", "../../../dir/a/b/foo.bin"},
		[]string{"/file/foo.bin", "../foo.bin", "../../foo.bin", "../../file/foo.bin"},
	)
}

func TestFilenameAllowedAccessWithExplicitAbsoluteUserFileRoot(t *testing.T) {
	// $ cd /dir
	// $ hurl --file-root /file test.hurl
	allowed := []string{"foo.bin", "/file/foo.bin", "a/foo.bin", "a/b/foo.bin", "../../file/foo.bin"}
	denied := []string{"/dir/foo.bin", "../dir/a/b/foo.bin", "../foo.bin", "../../foo.bin", "../../../dir/a/b/foo.bin"}
	checkAccess(t, NewContextDir("/dir", "/file"), allowed, denied)
	checkAccess(t, NewContextDir("/dir", "../file"), allowed, denied)
}

func TestFilenameAllowedAccessWithImplicitRelativeUserFileRoot(t *testing.T) {
	// $ cd /dir
	// $ hurl a/b/test.hurl
	checkAccess(t, NewContextDir("/dir", "a/b"),
		[]string{"foo.bin", "c/foo.bin", "/dir/a/b/foo.bin", "/dir/a/b/c/d/foo.bin", "../../../dir/a/b/foo.bin"},
		[]string{"/dir/foo.bin"},
	)
}

func TestFilenameAllowedAccessWithExplicitRelativeUserFileRoot(t *testing.T) {
	// $ cd /dir
	// $ hurl --file-root ../dir test.hurl
	checkAccess(t, NewContextDir("/dir", "../dir"),
		[]string{"foo.bin", "/dir/foo.bin", "a/foo.bin", "a/b/foo.bin", "../dir/a/b/foo.bin", "../../../dir/a/b/foo.bin"},
		[]string{"/file/foo.bin", "../foo.bin", "../../foo.bin", "../../file/foo.bin"},
	)
}

func checkAccess(t *testing.T, ctx ContextDir, allowed, denied []string) {
	t.Helper()
	for _, f := range allowed {
		if !ctx.IsAccessAllowed(f) {
			t.Errorf("%+v: %s should be allowed", ctx, f)
		}
	}
	for _, f := range denied {
		if ctx.IsAccessAllowed(f) {
			t.Errorf("%+v: %s should be denied", ctx, f)
		}
	}
}

func TestIsDescendant(t *testing.T) {
	tests := []struct {
		child, parent string
		want          bool
	}{
		{"/dir/foo/bar.txt", "/dir", true},
		{"/dir/foo/../bar.txt", "/dir", true},
		{"bar.txt", "", true},
		{"/dir/foo/../../bar.txt", "/dir", false},
		{"/a/bar.txt", "/b", false},
		{"/bar.txt", "", false},
	}
	for _, tt := range tests {
		if got := isDescendant(tt.child, tt.parent); got != tt.want {
			t.Errorf("isDescendant(%q, %q) = %v, want %v", tt.child, tt.parent, got, tt.want)
		}
	}
}

func TestIsDescendantSymlink(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root")
	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skip(err)
	}
	if isDescendant(filepath.Join(root, "link", "foo.bin"), root) {
		t.Error("symlink escaping root should not be a descendant")
	}
	if !isDescendant(filepath.Join(root, "new", "foo.bin"), root) {
		t.Error("non existing file should be a descendant")
	}
}

func TestResolvedPath(t *testing.T) {
	sep := string(filepath.Separator)
	tests := []struct{ root, file, want string }{
		{"", "foo.out", "foo.out"},
		{"/tmp", "filename", "/tmp" + sep + "filename"},
		{"/tmp/", "filename", "/tmp/filename"},
		{"a/b", "/abs/file", "/abs/file"},
		{"a", "../b", "a" + sep + "../b"},
	}
	for _, tt := range tests {
		if got := NewContextDir("", tt.root).ResolvedPath(tt.file); got != tt.want {
			t.Errorf("ResolvedPath(%q, %q) = %q, want %q", tt.root, tt.file, got, tt.want)
		}
	}
}

func TestRedactedStringHidesSecret(t *testing.T) {
	secrets := []string{"foo", "bar", "baz"}
	tests := []struct{ in, want string }{
		{"Hello, here are secrets values: foo", "Hello, here are secrets values: ***"},
		{"bar", "***"},
		{"Baz is not secret", "Baz is not secret"},
	}
	for _, tt := range tests {
		if got := Redact(tt.in, secrets); got != tt.want {
			t.Errorf("Redact(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestOutput(t *testing.T) {
	if NewOutput("-") != (OutputStdout{}) {
		t.Error("- should be stdout")
	}
	if got := NewOutput("foo.bin").String(); got != "foo.bin" {
		t.Errorf("got %q", got)
	}

	dir := t.TempDir()
	ctx := NewContextDir(dir, dir)
	o, err := NewOutput("out.bin").TryWith(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := o.Write([]byte("Hello"), nil, false); err != nil {
		t.Fatal(err)
	}
	if err := o.Write([]byte(" World"), nil, true); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "out.bin"))
	if string(b) != "Hello World" {
		t.Errorf("got %q", b)
	}

	if _, err := NewOutput("../out.bin").TryWith(ctx); err == nil {
		t.Error("expected unauthorized access")
	}
}
