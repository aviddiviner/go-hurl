package output

import (
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/term"
	"github.com/aviddiviner/go-hurl/internal/util"
)

func mustUrl(t *testing.T, s string) httpclient.Url {
	t.Helper()
	u, err := httpclient.ParseUrl(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func hurlResultJSON(t *testing.T) *runner.HurlResult {
	entry := func(index int, url string, response httpclient.Response) runner.EntryResult {
		e := runner.DefaultEntryResult()
		e.EntryIndex = ast.NewIndex(index)
		e.Calls = []httpclient.Call{{
			Request:  httpclient.Request{Url: mustUrl(t, url), Method: "GET"},
			Response: response,
		}}
		return e
	}
	defaultResponse := httpclient.Response{Version: httpclient.HttpVersionHttp10, Status: 200, Url: mustUrl(t, "http://localhost")}
	headers := httpclient.NewHeaderVec(
		httpclient.NewHeader("x-foo", "xxx"),
		httpclient.NewHeader("x-bar", "yyy0"),
		httpclient.NewHeader("x-bar", "yyy1"),
		httpclient.NewHeader("x-bar", "yyy2"),
		httpclient.NewHeader("x-baz", "zzz"),
	)
	return &runner.HurlResult{
		Entries: []runner.EntryResult{
			entry(1, "https://foo.com", defaultResponse),
			entry(2, "https://bar.com", defaultResponse),
			entry(3, "https://baz.com", httpclient.Response{
				Version: httpclient.HttpVersionHttp3,
				Status:  204,
				Headers: headers,
				Body:    []byte(`{"say": "Hello World!"}`),
				Url:     mustUrl(t, "https://baz.com"),
			}),
		},
		Duration: 100 * time.Millisecond,
		Success:  true,
	}
}

func TestWriteLastBodyWithHeaders(t *testing.T) {
	stdout := term.NewStdout(term.WriteModeBuffered)
	err := WriteLastBody(hurlResultJSON(t), true, false, pretty.None, util.OutputStdout{}, stdout, true)
	if err != nil {
		t.Fatal(err)
	}
	want := "HTTP/3 204\n" +
		"x-foo: xxx\n" +
		"x-bar: yyy0\n" +
		"x-bar: yyy1\n" +
		"x-bar: yyy2\n" +
		"x-baz: zzz\n" +
		"\n" +
		`{"say": "Hello World!"}`
	if got := string(stdout.Buffer()); got != want {
		t.Errorf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestWriteLastBodyNoEntries(t *testing.T) {
	stdout := term.NewStdout(term.WriteModeBuffered)
	if err := WriteLastBody(&runner.HurlResult{Success: true}, false, false, pretty.None, nil, stdout, false); err != nil {
		t.Fatal(err)
	}
	if len(stdout.Buffer()) != 0 {
		t.Errorf("got %q", stdout.Buffer())
	}
}
