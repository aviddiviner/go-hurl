package main

import (
	"testing"
)

// Expected messages are those of Hurl's hurlfmt (clap).
func TestParseArgsErrors(t *testing.T) {
	const hello = "../../integration/hurlfmt/tests_ok/hello.hurl"
	const try = "For more information, try '--help'.\n"
	tests := []struct {
		args []string
		want string
		info bool
	}{
		{[]string{"--foo"}, "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\nUsage: hurlfmt [OPTIONS] [FILES]...\n\n" + try, false},
		{[]string{"-x"}, "error: unexpected argument '-x' found\n\n  tip: to pass '-x' as a value, use '-- -x'\n\nUsage: hurlfmt [OPTIONS] [FILES]...\n\n" + try, false},
		{[]string{"--colo", hello}, "error: unexpected argument '--colo' found\n\n  tip: a similar argument exists: '--color'\n\nUsage: hurlfmt --color [FILES]...\n\n" + try, false},
		{[]string{hello, "--colo"}, "error: unexpected argument '--colo' found\n\n  tip: a similar argument exists: '--color'\n\nUsage: hurlfmt --color <FILES>...\n\n" + try, false},
		{[]string{"--check", "--output", "x", hello}, "error: the argument '--check' cannot be used with '--output <FILE>'\n\nUsage: hurlfmt --check <FILES>...\n\n" + try, false},
		{[]string{"--standalone", "--no-color", "--out", "html", hello}, "error: the argument '--standalone' cannot be used with '--no-color'\n\nUsage: hurlfmt --standalone --out <FORMAT> <FILES>...\n\n" + try, false},
		{[]string{"--check", "--check", hello}, "error: the argument '--check' cannot be used multiple times\n\nUsage: hurlfmt [OPTIONS] [FILES]...\n\n" + try, false},
		{[]string{"--check=1", hello}, "error: unexpected value '1' for '--check' found; no more were expected\n\nUsage: hurlfmt --check [FILES]...\n\n" + try, false},
		{[]string{"--in"}, "error: a value is required for '--in <FORMAT>' but none was supplied\n\n" + try, false},
		{[]string{"--in", "--check", hello}, "error: a value is required for '--in <FORMAT>' but none was supplied\n\n" + try, false},
		{[]string{"-o"}, "error: a value is required for '--output <FILE>' but none was supplied\n\n" + try, false},
		{[]string{"--in", "xml", hello}, "Invalid input format xml", false},
		{[]string{"--out", "xml", hello}, "Invalid output format xml", false},
		{[]string{"--standalone", hello}, "use --standalone option only with html output", false},
		{[]string{"--in-place", "--in", "curl", hello}, "You can use --in-place only hurl format!", false},
		{[]string{"--in-place"}, "You can not use --in-place with standard input stream!", false},
		{[]string{"nofile.hurl"}, "error: Cannot access 'nofile.hurl': No such file or directory", false},
		{[]string{"-h", "--foo"}, helpText, true},
		{[]string{"--version"}, versionText, true},
	}
	for _, tt := range tests {
		_, err := parseArgs(append([]string{"/usr/bin/hurlfmt"}, tt.args...))
		e, ok := err.(*optionsError)
		if !ok || e.message != tt.want || e.info != tt.info {
			t.Errorf("parseArgs(%q):\ngot:  %#v\nwant: %q", tt.args, err, tt.want)
		}
	}
}

func TestParseArgs(t *testing.T) {
	const hello = "../../integration/hurlfmt/tests_ok/hello.hurl"
	opts, err := parseArgs([]string{"hurlfmt", "--out=html", "--standalone", "-o", "out.html", hello})
	if err != nil {
		t.Fatal(err)
	}
	if opts.outputFormat != outputHTML || !opts.standalone || opts.outputFile != "out.html" ||
		len(opts.inputFiles) != 1 || opts.inputFiles[0].String() != hello || opts.color != nil {
		t.Errorf("got %+v", opts)
	}
	opts, err = parseArgs([]string{"hurlfmt", "--in-place", hello})
	if err != nil {
		t.Fatal(err)
	}
	if !opts.inPlace || opts.color == nil || *opts.color {
		t.Errorf("got %+v", opts)
	}
}

func TestFileName(t *testing.T) {
	tests := map[string]string{
		"hurlfmt": "hurlfmt", "/usr/bin/hurlfmt": "hurlfmt", "./hurlfmt": "hurlfmt",
		"bin/hurlfmt/": "hurlfmt", "bin/.": "bin", ".": "", "..": "", "/": "",
	}
	for in, want := range tests {
		if got := fileName(in); got != want {
			t.Errorf("fileName(%q) = %q, want %q", in, got, want)
		}
	}
}
