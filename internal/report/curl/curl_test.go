package curl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func curlCmd(t *testing.T, url string) httpclient.CurlCmd {
	t.Helper()
	u, err := httpclient.ParseUrl(url)
	if err != nil {
		t.Fatal(err)
	}
	spec := httpclient.NewRequestSpec()
	spec.Method = "GET"
	spec.Url = u
	return httpclient.NewCurlCmd(spec, httpclient.NewCookieStore(), util.ContextDir{}, nil, httpclient.DefaultClientOptions())
}

func TestWriteCurl(t *testing.T) {
	results := []*runner.HurlResult{
		{Entries: []runner.EntryResult{{CurlCmd: curlCmd(t, "http://localhost:8000/a")}, {CurlCmd: curlCmd(t, "http://localhost:8000/secret")}}},
		{Entries: []runner.EntryResult{{CurlCmd: curlCmd(t, "http://localhost:8000/b")}}},
	}
	filename := filepath.Join(t.TempDir(), "a", "b", "curl.txt")
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, []byte("previous content that is longer\n"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := WriteCurl(results, filename, []string{"secret"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	want := "curl 'http://localhost:8000/a'\ncurl 'http://localhost:8000/***'\ncurl 'http://localhost:8000/b'\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteCurlError(t *testing.T) {
	dir := t.TempDir()
	err := WriteCurl(nil, dir, nil)
	if err == nil || err.Error() != "Issue creating curl export "+dir+" (is a directory)" {
		t.Errorf("err = %v", err)
	}
}
