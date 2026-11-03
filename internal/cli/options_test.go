package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
)

func tmpHurlFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "foo.hurl")
	if err := os.WriteFile(path, []byte("GET http://localhost\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// tmpHurlConfig creates a home directory with a Hurl config file.
func tmpHurlConfig(t *testing.T, content string) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".config", "hurl")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return home
}

func parseWith(t *testing.T, env map[string]string, term bool, args ...string) *CliOptions {
	t.Helper()
	envVars := NewEnvVars(env)
	ctx := NewRunContext(envVars, true, term, term)
	opts, err := Parse(append([]string{"hurl"}, args...), ctx, envVars)
	if err != nil {
		t.Fatalf("Parse(%q): %v", args, err)
	}
	return opts
}

func TestLocationTrustedArgOverridesLocationEnvVar(t *testing.T) {
	opts := parseWith(t, map[string]string{"HURL_LOCATION": "false"}, true, "--location-trusted", tmpHurlFile(t))
	if !opts.FollowLocation || !opts.FollowLocationTrusted {
		t.Errorf("got %+v", opts)
	}
}

func TestInteractiveDefault(t *testing.T) {
	opts := parseWith(t, nil, true, tmpHurlFile(t))
	if opts.Test || opts.ProgressBar.Get() || opts.Parallel.Get() || opts.OutputType != OutputTypeResponseBody ||
		!opts.ColorStdout.Get() || !opts.ColorStderr.Get() || opts.Pretty != pretty.Automatic {
		t.Errorf("got %+v", opts)
	}
}

func TestNoInteractiveDefault(t *testing.T) {
	envVars := NewEnvVars(nil)
	ctx := NewRunContext(envVars, true, false, false)
	opts, err := Parse([]string{"hurl", tmpHurlFile(t)}, ctx, envVars)
	if err != nil {
		t.Fatal(err)
	}
	if opts.Test || opts.ProgressBar.Get() || opts.Parallel.Get() || opts.OutputType != OutputTypeResponseBody ||
		opts.ColorStdout.Get() || opts.ColorStderr.Get() || opts.Pretty != pretty.None {
		t.Errorf("got %+v", opts)
	}
}

func TestEnvVarImplyProgressParallelNoOutput(t *testing.T) {
	opts := parseWith(t, map[string]string{"HURL_TEST": "true"}, true, tmpHurlFile(t))
	if !opts.Test || !opts.ProgressBar.Get() || !opts.Parallel.Get() || opts.OutputType != OutputTypeNoOutput {
		t.Errorf("got %+v", opts)
	}
}

func TestCIContextImplyNoInteractive(t *testing.T) {
	opts := parseWith(t, map[string]string{"CI": "1"}, true, "--test", tmpHurlFile(t))
	if !opts.Test || opts.ProgressBar.Get() || !opts.Parallel.Get() || opts.OutputType != OutputTypeNoOutput {
		t.Errorf("got %+v", opts)
	}
}

func TestRespectNoColorEnvVar(t *testing.T) {
	opts := parseWith(t, map[string]string{"NO_COLOR": "1"}, true, tmpHurlFile(t))
	if opts.ColorStdout.Get() || opts.ColorStderr.Get() {
		t.Errorf("got %+v", opts)
	}
}

func TestCliArgsHigherPriorityOnHttpVersion(t *testing.T) {
	env := map[string]string{"HURL_HTTP3": "1"}
	file := tmpHurlFile(t)
	if opts := parseWith(t, env, true, file); opts.HttpVersion != HttpVersionV3 {
		t.Errorf("got %v", opts.HttpVersion)
	}
	if opts := parseWith(t, env, true, "--http2", file); opts.HttpVersion != HttpVersionV2 {
		t.Errorf("got %v", opts.HttpVersion)
	}
}

func TestConfigFileImplyProgressParallelNoOutput(t *testing.T) {
	home := tmpHurlConfig(t, "--test")
	opts := parseWith(t, map[string]string{"HOME": home}, true, tmpHurlFile(t))
	if !opts.Test || !opts.ProgressBar.Get() || !opts.Parallel.Get() || opts.OutputType != OutputTypeNoOutput {
		t.Errorf("got %+v", opts)
	}
}

func TestPriorityOrderIsFromConfigToEnvVarsToCliArgs(t *testing.T) {
	file := tmpHurlFile(t)
	opts := parseWith(t, nil, true, file)
	if !opts.ColorStdout.Get() || !opts.ColorStderr.Get() {
		t.Errorf("default: got %+v", opts)
	}
	home := tmpHurlConfig(t, "--no-color")
	opts = parseWith(t, map[string]string{"HOME": home}, true, file)
	if opts.ColorStdout.Get() || opts.ColorStderr.Get() {
		t.Errorf("config: got %+v", opts)
	}
	env := map[string]string{"HOME": home, "HURL_COLOR": "1"}
	opts = parseWith(t, env, true, file)
	if !opts.ColorStdout.Get() || !opts.ColorStderr.Get() {
		t.Errorf("env: got %+v", opts)
	}
	opts = parseWith(t, env, true, "--no-color", file)
	if opts.ColorStdout.Get() || opts.ColorStderr.Get() {
		t.Errorf("cli: got %+v", opts)
	}
}

func TestRespectTlsEnvVars(t *testing.T) {
	file := tmpHurlFile(t)
	if opts := parseWith(t, map[string]string{"HURL_TLS_MAX": "1.2"}, true, file); opts.TlsMax != TlsVersionTls12 {
		t.Errorf("got %v", opts.TlsMax)
	}
	if opts := parseWith(t, map[string]string{"HURL_TLS_MIN": "1.2"}, true, file); opts.TlsMin != TlsVersionTls12 {
		t.Errorf("got %v", opts.TlsMin)
	}
}

func TestParseArgs(t *testing.T) {
	file := tmpHurlFile(t)
	opts := parseWith(t, nil, true,
		"-kL", "-H", "a: b", "--header=c:d", "--retry", "-1", "--max-redirs=10", "--jobs", "3",
		"--variable", "n=1", "--secret", "s=x", "--very-verbose", "--connect-timeout", "5",
		"-o", "out.txt", "--json", "--noproxy", "h", "--header=-x:1", file)
	if !opts.Insecure || !opts.FollowLocation || opts.FollowLocationTrusted {
		t.Errorf("flags: %+v", opts)
	}
	if strings.Join(opts.Headers, "|") != "a: b|c:d|-x:1" {
		t.Errorf("headers: %q", opts.Headers)
	}
	if opts.Retry != (ast.CountInfinite{}) || opts.MaxRedirect != (ast.CountFinite{Value: 10}) || opts.Jobs != 3 {
		t.Errorf("numbers: %+v", opts)
	}
	if opts.Variables["n"] != runner.NewInteger(1) || opts.Secrets["s"] != "x" {
		t.Errorf("variables: %v %v", opts.Variables, opts.Secrets)
	}
	if opts.Verbosity != VerbosityDebug || opts.ConnectTimeout != 5*time.Second || opts.OutputType != OutputTypeJSON {
		t.Errorf("output: %+v", opts)
	}
	if opts.Output == nil || opts.Output.String() != "out.txt" || *opts.NoProxy != "h" {
		t.Errorf("output: %+v", opts)
	}
	if len(opts.InputFiles) != 1 || opts.InputFiles[0].String() != file {
		t.Errorf("input files: %v", opts.InputFiles)
	}

	ro, err := opts.ToRunnerOptions(opts.InputFiles[0], "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if ro.Headers.Len() != 3 || !ro.FollowLocation.Follow || ro.FollowLocation.Forwarding != httpclient.OnlyInitialHost {
		t.Errorf("runner options: %+v", ro)
	}
}

func TestToRunnerOptionsErrors(t *testing.T) {
	file := tmpHurlFile(t)
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"-H", "foo"}, "Invalid header <foo>, missing `:`"},
		{[]string{"--no-header", " "}, "Missing header name"},
		{[]string{"--proxy-header", "foo"}, "Invalid proxy header <foo>, missing `:`"},
	}
	for _, tt := range tests {
		opts := parseWith(t, nil, true, append(tt.args, file)...)
		_, err := opts.ToRunnerOptions(opts.InputFiles[0], "/tmp")
		if err != (CliErrorInvalidOption{Message: tt.want}) {
			t.Errorf("%q: got %v, want %q", tt.args, err, tt.want)
		}
	}
}

// Expected messages are those of Hurl (clap).
func TestParseArgsErrors(t *testing.T) {
	file := tmpHurlFile(t)
	const try = "For more information, try '--help'.\n"
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"--toto", file}, "error: unexpected argument '--toto' found\n\n  tip: a similar argument exists: '--to-entry'\n\nUsage: hurl --to-entry <ENTRY_NUMBER> [FILES]...\n\n" + try},
		{[]string{"--foo"}, "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\nUsage: hurl [OPTIONS] [FILES]...\n\n" + try},
		{[]string{"-z"}, "error: unexpected argument '-z' found\n\n  tip: to pass '-z' as a value, use '-- -z'\n\nUsage: hurl [OPTIONS] [FILES]...\n\n" + try},
		{[]string{"--location", "--max-redirs", "-5", file}, "error: invalid value '-5' for '--max-redirs <NUM>' (-5 is not in -1..=2147483647)"},
		{[]string{"--jobs", "0", file}, "error: invalid value '0' for '--jobs <NUM>': 0 is not in 1..=4294967295\n\n" + try},
		{[]string{"--jobs", "abc", file}, "error: invalid value 'abc' for '--jobs <NUM>': invalid digit found in string\n\n" + try},
		{[]string{"--jobs", "-1", file}, "error: unexpected argument '-1' found\n\n  tip: to pass '-1' as a value, use '-- -1'\n\nUsage: hurl [OPTIONS] [FILES]...\n\n" + try},
		{[]string{"--limit-rate=-1", file}, "error: invalid value '-1' for '--limit-rate <SPEED>': invalid digit found in string\n\n" + try},
		{[]string{"--error-format", "shor", file}, "error: invalid value 'shor' for '--error-format <FORMAT>'\n  [possible values: short, long]\n\n  tip: a similar value exists: 'short'\n\n" + try},
		{[]string{"--tls-max=", file}, "error: a value is required for '--tls-max <VERSION>' but none was supplied\n  [possible values: 1.0, 1.1, 1.2, 1.3]\n\n" + try},
		{[]string{"--color", "--no-color", file}, "error: the argument '--color' cannot be used with '--no-color'\n\nUsage: hurl --color <FILES>...\n\n" + try},
		{[]string{"--netrc", "--netrc-optional", "--netrc-file", file, file}, "error: the argument '--netrc' cannot be used with:\n  --netrc-optional\n  --netrc-file <FILE>\n\nUsage: hurl --netrc <FILES>...\n\n" + try},
		{[]string{"--test", "--test", file}, "error: the argument '--test' cannot be used multiple times\n\nUsage: hurl [OPTIONS] [FILES]...\n\n" + try},
		{[]string{"--output", "a", "--output", "b", file}, "error: the argument '--output <FILE>' cannot be used multiple times\n\nUsage: hurl [OPTIONS] [FILES]...\n\n" + try},
		{[]string{"--test=1", file}, "error: unexpected value '1' for '--test' found; no more were expected\n\nUsage: hurl --test [FILES]...\n\n" + try},
		{[]string{"--header"}, "error: a value is required for '--header <NAME:VALUE>' but none was supplied\n\n" + try},
		{[]string{"--header", "--test", file}, "error: a value is required for '--header <NAME:VALUE>' but none was supplied\n\n" + try},
		{[]string{"nofile.hurl"}, "error: Cannot access 'nofile.hurl': No such file or directory"},
		{[]string{"--glob", "does_not_exist/*.hurl"}, "error: Cannot access 'does_not_exist/*.hurl': No such file or directory"},
		{[]string{"--delay", "abc", file}, "error: Invalid duration"},
		{[]string{"--variable", "a", file}, "error: Missing value for variable a!"},
		{[]string{"--secret", "a=1", "--secret", "a=2", file}, "error: secret 'a' can't be reassigned"},
		{[]string{"--cacert", "nofile.pem", file}, "error: Input file nofile.pem does not exist"},
		{[]string{"--cert", "nofile.pem", file}, "error: File nofile.pem does not exist"},
		// go-hurl dropped features.
		{[]string{"--http3", file}, "error: --http3 is not supported by go-hurl"},
		{[]string{"-0", file}, "error: --http1.0 is not supported by go-hurl"},
		{[]string{"--ntlm", file}, "error: --ntlm is not supported by go-hurl"},
		{[]string{"--negotiate", file}, "error: --negotiate is not supported by go-hurl"},
		{[]string{"--digest", file}, "error: --digest is not supported by go-hurl"},
		{[]string{"--aws-sigv4", "aws:amz:eu-central-1:foo", file}, "error: --aws-sigv4 is not supported by go-hurl"},
		{[]string{"--cert", file + ":password", file}, "error: --cert with a password is not supported by go-hurl"},
	}
	for _, tt := range tests {
		envVars := NewEnvVars(nil)
		ctx := NewRunContext(envVars, true, false, false)
		_, err := Parse(append([]string{"/usr/bin/hurl"}, tt.args...), ctx, envVars)
		if err == nil || err.Error() != tt.want {
			t.Errorf("Parse(%q):\ngot:  %q\nwant: %q", tt.args, err, tt.want)
		}
	}
}

func TestParseArgsInfo(t *testing.T) {
	helpWidth = func() int { return 100 }
	envVars := NewEnvVars(nil)
	ctx := NewRunContext(envVars, true, false, false)
	_, err := Parse([]string{"hurl", "-h", "--foo"}, ctx, envVars)
	if e, ok := err.(OptionsErrorDisplayHelp); !ok || e.Message != renderHelp("hurl", 100, false) {
		t.Errorf("got %v", err)
	}
	_, err = Parse([]string{"hurl", "--version"}, ctx, envVars)
	if e, ok := err.(OptionsErrorDisplayVersion); !ok || !strings.HasPrefix(e.Message, "hurl 8.1.0 (go-hurl 0.1.0) go") ||
		!strings.Contains(e.Message, "\nFeatures (net/http): ") || !strings.HasSuffix(e.Message, "\n") {
		t.Errorf("got %v", err)
	}
	_, err = Parse([]string{"hurl"}, ctx, envVars)
	if e, ok := err.(OptionsErrorNoInput); !ok || e.Message != renderHelp("hurl", 100, false) {
		t.Errorf("got %v", err)
	}
}

func TestGlobAndDirectory(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"b.hurl", "a.hurl", "c.txt", "sub/d.hurl", "sub/e.hurl", ".hurl"} {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := glob(dir + "/*.hurl")
	if err != nil {
		t.Fatal(err)
	}
	if want := nativePaths(dir+"/.hurl", dir+"/a.hurl", dir+"/b.hurl"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("glob: got %q", got)
	}
	got, err = glob(dir + "/**/*.hurl")
	if err != nil {
		t.Fatal(err)
	}
	if want := nativePaths(dir+"/.hurl", dir+"/a.hurl", dir+"/b.hurl", dir+"/sub/d.hurl", dir+"/sub/e.hurl"); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("glob **: got %q", got)
	}
	if _, err := glob(dir + "/a**"); err == nil {
		t.Error("expected a pattern error")
	}

	opts := parseWith(t, nil, true, dir)
	var files []string
	for _, f := range opts.InputFiles {
		files = append(files, f.String())
	}
	if want := nativePaths(dir+"/a.hurl", dir+"/b.hurl", dir+"/sub/d.hurl", dir+"/sub/e.hurl"); strings.Join(files, ",") != strings.Join(want, ",") {
		t.Errorf("directory: got %q", files)
	}
}

func nativePaths(paths ...string) []string {
	for i, p := range paths {
		paths[i] = filepath.FromSlash(p)
	}
	return paths
}

func TestCreateRunSummary(t *testing.T) {
	newResults := func(specs ...bool) []*runner.HurlResult {
		var results []*runner.HurlResult
		for _, success := range specs {
			results = append(results, &runner.HurlResult{Entries: make([]runner.EntryResult, 10), Success: success})
		}
		return results
	}
	tests := []struct {
		results  []*runner.HurlResult
		duration time.Duration
		want     string
	}{
		{newResults(true, true, true), 128 * time.Millisecond, "--------------------------------------------------------------------------------\n" +
			"Executed files:    3\n" +
			"Executed requests: 0 (0.0/s)\n" +
			"Succeeded files:   3 (100.0%)\n" +
			"Failed files:      0 (0.0%)\n" +
			"Duration:          128 ms (0h:0m:0s:128ms)\n"},
		{newResults(true, false, true), 200 * time.Millisecond, "--------------------------------------------------------------------------------\n" +
			"Executed files:    3\n" +
			"Executed requests: 0 (0.0/s)\n" +
			"Succeeded files:   2 (66.7%)\n" +
			"Failed files:      1 (33.3%)\n" +
			"Duration:          200 ms (0h:0m:0s:200ms)\n"},
		{newResults(true, true), 3661111 * time.Millisecond, "--------------------------------------------------------------------------------\n" +
			"Executed files:    2\n" +
			"Executed requests: 0 (0.0/s)\n" +
			"Succeeded files:   2 (100.0%)\n" +
			"Failed files:      0 (0.0%)\n" +
			"Duration:          3661111 ms (1h:1m:1s:111ms)\n"},
	}
	for _, tt := range tests {
		if got := Summary(tt.results, tt.duration); got != tt.want {
			t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
		}
	}
}
