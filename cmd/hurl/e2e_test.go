package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// End-to-end tests: build the hurl binary and run it on Hurl files against
// a local test server.

var hurlBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-hurl-e2e")
	if err != nil {
		panic(err)
	}
	hurlBin = filepath.Join(dir, "hurl")
	if runtime.GOOS == "windows" {
		hurlBin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", hurlBin, ".").CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func newServer(t *testing.T) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/hello", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello World!")
	})
	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"name":"Bob","age":42}`)
	})
	mux.HandleFunc("/cookie", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "abc123", Path: "/"})
		fmt.Fprint(w, "cookie set")
	})
	mux.HandleFunc("/user/{name}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "user %s", r.PathValue("name"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

type result struct {
	stdout, stderr string
	code           int
}

// hurl runs the binary in dir with args.
func hurl(t *testing.T, dir string, args ...string) result {
	t.Helper()
	cmd := exec.Command(hurlBin, args...)
	cmd.Dir = dir
	// A clean environment: no config file, no colors, no CI detection.
	cmd.Env = []string{"HOME=" + dir, "PATH=" + os.Getenv("PATH"), "COLUMNS=100"}
	cmd.Stdin = strings.NewReader("")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return result{stdout.String(), stderr.String(), code}
}

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func check(t *testing.T, r result, code int, stdout string, stderrContains ...string) {
	t.Helper()
	if r.code != code {
		t.Errorf("exit code %d, want %d\nstdout: %s\nstderr: %s", r.code, code, r.stdout, r.stderr)
	}
	if stdout != "*" && r.stdout != stdout {
		t.Errorf("stdout:\n%q\nwant:\n%q", r.stdout, stdout)
	}
	for _, s := range stderrContains {
		if !strings.Contains(r.stderr, s) {
			t.Errorf("stderr doesn't contain %q:\n%s", s, r.stderr)
		}
	}
}

func TestE2E(t *testing.T) {
	srv := newServer(t)
	dir := writeFiles(t, map[string]string{
		"hello.hurl": "GET " + srv.URL + "/hello\nHTTP 200\n[Asserts]\nbody == \"Hello World!\"\n",
		"json.hurl": "GET " + srv.URL + "/json\nHTTP 200\n[Captures]\nname: jsonpath \"$.name\"\n[Asserts]\njsonpath \"$.age\" == 42\n\n" +
			"GET " + srv.URL + "/user/{{name}}\nHTTP 200\n[Asserts]\nbody == \"user Bob\"\n",
		"assert.hurl":   "GET " + srv.URL + "/hello\nHTTP 200\n[Asserts]\nbody == \"Goodbye\"\n",
		"status.hurl":   "GET " + srv.URL + "/missing\nHTTP 200\n",
		"parse.hurl":    "GET " + srv.URL + "/hello\nHTTP 200\n[Asserts]\nbody ==\n",
		"runtime.hurl":  "GET http://127.0.0.1:1/unreachable\nHTTP 200\n",
		"variable.hurl": "GET " + srv.URL + "/user/{{user}}\nHTTP 200\n[Asserts]\nbody == \"user {{user}}\"\n",
		"cookie.hurl":   "GET " + srv.URL + "/cookie\nHTTP 200\n",
		"empty.hurl":    "# no entries\n",
	})

	t.Run("success", func(t *testing.T) {
		check(t, hurl(t, dir, "hello.hurl"), exitOK, "Hello World!")
	})
	t.Run("captures and templates", func(t *testing.T) {
		check(t, hurl(t, dir, "json.hurl"), exitOK, "user Bob")
	})
	t.Run("include", func(t *testing.T) {
		r := hurl(t, dir, "-i", "hello.hurl")
		check(t, r, exitOK, "*")
		if !strings.HasPrefix(r.stdout, "HTTP/1.1 200\n") || !strings.HasSuffix(r.stdout, "\n\nHello World!") {
			t.Errorf("stdout: %q", r.stdout)
		}
	})
	t.Run("variable", func(t *testing.T) {
		check(t, hurl(t, dir, "--variable", "user=Alice", "variable.hurl"), exitOK, "user Alice")
	})
	t.Run("assert failure", func(t *testing.T) {
		check(t, hurl(t, dir, "assert.hurl"), exitErrorAssert, "", "error: Assert failure", "assert.hurl:4:0")
		check(t, hurl(t, dir, "status.hurl"), exitErrorAssert, "", "error: Assert status code")
	})
	t.Run("parse error", func(t *testing.T) {
		check(t, hurl(t, dir, "parse.hurl"), exitErrorParsing, "", "error: Parsing", "parse.hurl:4:")
		check(t, hurl(t, dir, "--test", "hello.hurl", "parse.hurl"), exitErrorParsing, "", "error: Parsing")
	})
	t.Run("runtime error", func(t *testing.T) {
		check(t, hurl(t, dir, "runtime.hurl"), exitErrorRuntime, "", "error: HTTP connection")
		check(t, hurl(t, dir, "assert.hurl", "runtime.hurl"), exitErrorRuntime, "*")
	})
	t.Run("no entries", func(t *testing.T) {
		check(t, hurl(t, dir, "empty.hurl"), exitOK, "", "warning: No entry have been executed for file empty.hurl")
	})
	t.Run("test mode", func(t *testing.T) {
		r := hurl(t, dir, "--test", "hello.hurl", "json.hurl", "assert.hurl")
		check(t, r, exitErrorAssert, "",
			"Success hello.hurl (1 request(s) in ",
			"Success json.hurl (2 request(s) in ",
			"Failure assert.hurl (1 request(s) in ",
			"Executed files:    3\n",
			"Executed requests: 4 (",
			"Succeeded files:   2 (66.7%)\n",
			"Failed files:      1 (33.3%)\n")
	})
	t.Run("test mode sequential", func(t *testing.T) {
		r := hurl(t, dir, "--test", "--jobs", "1", "--repeat", "2", "hello.hurl", "json.hurl")
		check(t, r, exitOK, "", "Executed files:    4\n", "Executed requests: 6 (")
		first := strings.Index(r.stderr, "Success hello.hurl")
		second := strings.Index(r.stderr, "Success json.hurl")
		if first < 0 || second < first {
			t.Errorf("unexpected order:\n%s", r.stderr)
		}
	})
	t.Run("json", func(t *testing.T) {
		r := hurl(t, dir, "--json", "hello.hurl", "json.hurl")
		check(t, r, exitOK, "*")
		lines := strings.Split(strings.TrimSuffix(r.stdout, "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("got %d lines: %q", len(lines), r.stdout)
		}
		var v struct {
			Filename string `json:"filename"`
			Success  bool   `json:"success"`
			Entries  []any  `json:"entries"`
		}
		if err := json.Unmarshal([]byte(lines[1]), &v); err != nil {
			t.Fatal(err)
		}
		if v.Filename != "json.hurl" || !v.Success || len(v.Entries) != 2 {
			t.Errorf("got %+v", v)
		}
	})
	t.Run("parallel json order", func(t *testing.T) {
		r := hurl(t, dir, "--parallel", "--json", "hello.hurl", "json.hurl")
		check(t, r, exitOK, "*")
		if strings.Count(r.stdout, "\n") != 2 {
			t.Errorf("stdout: %q", r.stdout)
		}
	})
	t.Run("very verbose", func(t *testing.T) {
		r := hurl(t, dir, "--very-verbose", "hello.hurl")
		check(t, r, exitOK, "Hello World!", "* Executing entry 1", "> GET /hello", "< HTTP/1.1 200")
	})
	t.Run("verbose parallel", func(t *testing.T) {
		r := hurl(t, dir, "--verbose", "--parallel", "--jobs", "2", "hello.hurl", "json.hurl")
		check(t, r, exitOK, "*", "* Parallel run using 2 workers")
	})
	t.Run("reports and cookies", func(t *testing.T) {
		r := hurl(t, dir, "--report-tap", "report.tap", "--report-junit", "report.xml", "--report-json", "json-report",
			"--report-html", "html-report", "--cookie-jar", "cookies.txt", "--curl", "curl.txt", "hello.hurl", "cookie.hurl")
		check(t, r, exitOK, "Hello World!cookie set")
		for _, f := range []string{"report.tap", "report.xml", "json-report/report.json", "html-report/index.html", "curl.txt"} {
			if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
				t.Errorf("missing report %s: %v", f, err)
			}
		}
		cookies, err := os.ReadFile(filepath.Join(dir, "cookies.txt"))
		if err != nil {
			t.Fatal(err)
		}
		want := "# Netscape HTTP Cookie File\n# This file was generated by Hurl\n\n# Cookies for file <hello.hurl>\n# Cookies for file <cookie.hurl>\n"
		if !strings.HasPrefix(string(cookies), want) || !strings.Contains(string(cookies), "\tsession\tabc123\n") {
			t.Errorf("cookies:\n%s", cookies)
		}
		tapReport, _ := os.ReadFile(filepath.Join(dir, "report.tap"))
		if string(tapReport) != "TAP version 13\n1..2\nok 1 - hello.hurl\nok 2 - cookie.hurl\n" {
			t.Errorf("tap:\n%s", tapReport)
		}
	})
	t.Run("output file", func(t *testing.T) {
		check(t, hurl(t, dir, "-o", "out.txt", "hello.hurl", "hello.hurl"), exitOK, "")
		out, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
		if string(out) != "Hello World!Hello World!" {
			t.Errorf("output: %q", out)
		}
	})
	t.Run("dropped flags", func(t *testing.T) {
		for _, flag := range []string{"--http3", "--http1.0", "--ntlm", "--negotiate", "--digest"} {
			check(t, hurl(t, dir, flag, "hello.hurl"), exitErrorCommandline, "", "error: "+flag+" is not supported by go-hurl\n")
		}
		check(t, hurl(t, dir, "--aws-sigv4", "aws:amz:eu-central-1:sts", "hello.hurl"), exitErrorCommandline, "",
			"error: --aws-sigv4 is not supported by go-hurl\n")
		check(t, hurl(t, dir, "--ssl-no-revoke", "hello.hurl"), exitOK, "Hello World!")
	})
	t.Run("command line errors", func(t *testing.T) {
		check(t, hurl(t, dir, "--toto", "hello.hurl"), exitErrorCommandline, "",
			"error: unexpected argument '--toto' found\n\n  tip: a similar argument exists: '--to-entry'\n\nUsage: hurl --to-entry <ENTRY_NUMBER> [FILES]...\n\nFor more information, try '--help'.\n")
		check(t, hurl(t, dir, "nofile.hurl"), exitErrorCommandline, "",
			"error: Cannot access 'nofile.hurl': No such file or directory\n")
		check(t, hurl(t, dir, "-H", "foo", "hello.hurl"), exitErrorCommandline, "", "error: Invalid header <foo>, missing `:`\n")
	})
	t.Run("help and version", func(t *testing.T) {
		want, err := os.ReadFile("../../integration/hurl/tests_pty/help/help.out")
		if err != nil {
			t.Fatal(err)
		}
		check(t, hurl(t, dir, "--help"), exitOK, string(want))
		r := hurl(t, dir, "--version")
		check(t, r, exitOK, "*")
		if !strings.HasPrefix(r.stdout, "hurl 8.1.0 (go-hurl 0.1.0) "+runtime.Version()+" "+runtime.GOOS+"/"+runtime.GOARCH+"\n") {
			t.Errorf("version: %q", r.stdout)
		}
	})
	t.Run("stdin", func(t *testing.T) {
		cmd := exec.Command(hurlBin)
		cmd.Env = []string{"HOME=" + dir}
		cmd.Stdin = strings.NewReader("GET " + srv.URL + "/hello\n")
		out, err := cmd.Output()
		if err != nil || string(out) != "Hello World!" {
			t.Errorf("got %q, %v", out, err)
		}
	})
}
