package cli

import (
	"fmt"
	"math"
	"strconv"
)

// argDef describes a command line argument, mirroring Hurl's clap
// definitions (cli/options/commands.rs).
type argDef struct {
	id        string
	long      string
	short     string
	valueName string // empty for flags
	help      string
	heading   string
	// multiple is clap's ArgAction::Append.
	multiple       bool
	conflicts      []string
	possibleValues []string
	// parser validates a value like clap's value_parser!, returning the
	// error source on failure.
	parser        func(string) error
	allowNegative bool
	alias         string
}

func (a *argDef) isFlag() bool { return a.valueName == "" }

// display renders the arg as clap does in error messages.
func (a *argDef) display() string {
	if a.valueName != "" {
		return "--" + a.long + " <" + a.valueName + ">"
	}
	return "--" + a.long
}

const filesID = "input_files"

// argDefs are in clap's definition order (cli/options/args.rs), which is the
// order of the help and of the suggestions. clap's generated help and
// version are last.
var argDefs = []*argDef{
	// HTTP options
	{id: "aws_sigv4", long: "aws-sigv4", valueName: "PROVIDER1[:PROVIDER2[:REGION[:SERVICE]]]", help: "Use AWS V4 signature authentication in the transfer", heading: "HTTP options"},
	{id: "cacert_file", long: "cacert", valueName: "FILE", help: "CA certificate to verify peer against (PEM format)", heading: "HTTP options"},
	{id: "client_cert_file", long: "cert", short: "E", valueName: "CERTIFICATE[:PASSWORD]", help: "Client certificate file and password", heading: "HTTP options"},
	{id: "compressed", long: "compressed", help: "Request compressed response (using deflate or gzip)", heading: "HTTP options"},
	{id: "connect_timeout", long: "connect-timeout", valueName: "SECONDS", help: "Maximum time allowed for connection [default: 300]", heading: "HTTP options"},
	{id: "connect_to", long: "connect-to", valueName: "HOST1:PORT1:HOST2:PORT2", help: "For a request to the given HOST1:PORT1 pair, connect to HOST2:PORT2 instead", heading: "HTTP options", multiple: true},
	{id: "digest", long: "digest", help: "Tell Hurl to use HTTP Digest authentication", heading: "HTTP options"},
	{id: "header", long: "header", short: "H", valueName: "NAME:VALUE", help: "Pass custom header(s) to server", heading: "HTTP options", multiple: true},
	{id: "http10", long: "http1.0", short: "0", help: "Tell Hurl to use HTTP version 1.0", heading: "HTTP options"},
	{id: "http11", long: "http1.1", help: "Tell Hurl to use HTTP version 1.1", heading: "HTTP options"},
	{id: "http2", long: "http2", help: "Tell Hurl to use HTTP version 2", heading: "HTTP options"},
	{id: "http2_prior_knowledge", long: "http2-prior-knowledge", help: "Tell Hurl to use HTTP version 2 without HTTP/1.1 Upgrade", heading: "HTTP options"},
	{id: "http3", long: "http3", help: "Tell Hurl to use HTTP version 3", heading: "HTTP options"},
	{id: "insecure", long: "insecure", short: "k", help: "Allow insecure SSL connections", heading: "HTTP options"},
	{id: "ipv4", long: "ipv4", short: "4", help: "Tell Hurl to use IPv4 addresses only when resolving host names, and not for example try IPv6", heading: "HTTP options"},
	{id: "ipv6", long: "ipv6", short: "6", help: "Tell Hurl to use IPv6 addresses only when resolving host names, and not for example try IPv4", heading: "HTTP options"},
	{id: "client_key_file", long: "key", valueName: "KEY", help: "Private key file name", heading: "HTTP options"},
	{id: "limit_rate", long: "limit-rate", valueName: "SPEED", help: "Specify the maximum transfer rate in bytes/second, for both downloads and uploads", heading: "HTTP options", parser: parseU64},
	{id: "follow_location", long: "location", short: "L", help: "Follow redirects", heading: "HTTP options"},
	{id: "follow_location_trusted", long: "location-trusted", help: "Follow redirects but allows sending the name + password to all hosts that the site may redirect to", heading: "HTTP options"},
	{id: "max_filesize", long: "max-filesize", valueName: "BYTES", help: "Specify the maximum size in bytes of a file to download", heading: "HTTP options", parser: parseU64},
	{id: "max_redirects", long: "max-redirs", valueName: "NUM", help: "Maximum number of redirects allowed, -1 for unlimited redirects [default: 50]", heading: "HTTP options", parser: parseI32, allowNegative: true},
	{id: "max_time", long: "max-time", short: "m", valueName: "SECONDS", help: "Maximum time allowed for the transfer [default: 300]", heading: "HTTP options"},
	{id: "negotiate", long: "negotiate", help: "Tell Hurl to use Negotiate (SPNEGO) authentication", heading: "HTTP options"},
	{id: "no_cookie_store", long: "no-cookie-store", help: "Do not use cookie store between requests", heading: "HTTP options", conflicts: []string{"cookies_input_file"}},
	{id: "no_header", long: "no-header", valueName: "NAME", help: "Remove header(s) sent to server", heading: "HTTP options", multiple: true},
	{id: "no_proxy", long: "no-proxy", valueName: "HOST(S)", help: "List of hosts which do not use proxy", heading: "HTTP options", alias: "noproxy"},
	{id: "ntlm", long: "ntlm", help: "Tell Hurl to use NTLM authentication", heading: "HTTP options"},
	{id: "path_as_is", long: "path-as-is", help: "Tell Hurl to not handle sequences of /../ or /./ in the given URL path", heading: "HTTP options"},
	{id: "pinned_pub_key", long: "pinnedpubkey", valueName: "HASHES", help: "Public key to verify peer against", heading: "HTTP options"},
	{id: "proxy", long: "proxy", short: "x", valueName: "[PROTOCOL://]HOST[:PORT]", help: "Use proxy on given PROTOCOL/HOST/PORT", heading: "HTTP options"},
	{id: "proxy_header", long: "proxy-header", valueName: "HEADER", help: "Extra header to include in the request when sending to a proxy", heading: "HTTP options", multiple: true},
	{id: "resolve", long: "resolve", valueName: "HOST:PORT:ADDR", help: "Provide a custom address for a specific HOST and PORT pair", heading: "HTTP options", multiple: true},
	{id: "ssl_no_revoke", long: "ssl-no-revoke", help: "(Windows) Tell Hurl to disable certificate revocation checks", heading: "HTTP options"},
	{id: "tls_max", long: "tls-max", valueName: "VERSION", help: "Set the maximum allowed TLS version", heading: "HTTP options", possibleValues: []string{"1.0", "1.1", "1.2", "1.3"}},
	{id: "tls_min", long: "tls-min", valueName: "VERSION", help: "Set the minimum allowed TLS version", heading: "HTTP options", possibleValues: []string{"1.0", "1.1", "1.2", "1.3"}},
	{id: "unix_socket", long: "unix-socket", valueName: "PATH", help: "(HTTP) Connect through this Unix domain socket, instead of using the network", heading: "HTTP options"},
	{id: "user", long: "user", short: "u", valueName: "USER:PASSWORD", help: "Add basic Authentication header to each request", heading: "HTTP options"},
	{id: "user_agent", long: "user-agent", short: "A", valueName: "NAME", help: "Specify the User-Agent string to send to the HTTP server", heading: "HTTP options"},
	// Output options
	{id: "color", long: "color", help: "Colorize output", heading: "Output options", conflicts: []string{"no_color"}},
	{id: "curl", long: "curl", valueName: "FILE", help: "Export each request to a list of curl commands", heading: "Output options"},
	{id: "error_format", long: "error-format", valueName: "FORMAT", help: "Control the format of error messages [default: short]", heading: "Output options", possibleValues: []string{"short", "long"}},
	{id: "fail_with_body", long: "fail-with-body", help: "Output body response if there are any errors", heading: "Output options"},
	{id: "include", long: "include", short: "i", help: "Include the HTTP headers in the output", heading: "Output options"},
	{id: "json", long: "json", help: "Output each Hurl file result to JSON", heading: "Output options", conflicts: []string{"no_output"}},
	{id: "no_color", long: "no-color", help: "Do not colorize output", heading: "Output options", conflicts: []string{"color"}},
	{id: "no_output", long: "no-output", help: "Suppress output. By default, Hurl outputs the body of the last response", heading: "Output options", conflicts: []string{"json"}},
	{id: "no_pretty", long: "no-pretty", help: "Do not prettify response output", heading: "Output options", conflicts: []string{"pretty"}},
	{id: "output", long: "output", short: "o", valueName: "FILE", help: "Write to FILE instead of stdout", heading: "Output options"},
	{id: "pretty", long: "pretty", help: "Prettify JSON response output", heading: "Output options"},
	{id: "progress_bar", long: "progress-bar", help: "Display a progress bar in test mode", heading: "Output options"},
	{id: "verbose", long: "verbose", short: "v", help: "Turn on verbose (alias to --verbosity verbose)", heading: "Output options"},
	{id: "very_verbose", long: "very-verbose", help: "Turn on very verbose output, including HTTP response and libcurl logs (alias to --verbosity debug)", heading: "Output options"},
	{id: "verbosity", long: "verbosity", valueName: "LEVEL", help: "Set verbosity level for debug log", heading: "Output options", possibleValues: []string{"brief", "verbose", "debug"}},
	// Run options
	{id: "continue_on_error", long: "continue-on-error", help: "Continue executing requests even if an error occurs", heading: "Run options"},
	{id: "delay", long: "delay", valueName: "MILLISECONDS", help: "Sets delay before each request (aka sleep) [default: 0]", heading: "Run options"},
	{id: "from_entry", long: "from-entry", valueName: "ENTRY_NUMBER", help: "Execute Hurl file from ENTRY_NUMBER (starting at 1)", heading: "Run options", parser: parseU32Min1},
	{id: "jobs", long: "jobs", valueName: "NUM", help: "Maximum number of parallel jobs, 1 to disable parallel execution", heading: "Run options", parser: parseU32Min1},
	{id: "no_assert", long: "no-assert", help: "Ignore asserts defined in the Hurl file", heading: "Run options"},
	{id: "no_jsonpath_coercion", long: "no-jsonpath-coercion", help: "Disable JSONPath result coercion", heading: "Run options"},
	{id: "parallel", long: "parallel", help: "Run files in parallel (default in test mode)", heading: "Run options"},
	{id: "repeat", long: "repeat", valueName: "NUM", help: "Repeat the input files sequence NUM times, -1 for infinite loop", heading: "Run options", parser: parseI32, allowNegative: true},
	{id: "retry", long: "retry", valueName: "NUM", help: "Maximum number of retries, 0 for no retries, -1 for unlimited retries", heading: "Run options", parser: parseI32, allowNegative: true},
	{id: "retry_interval", long: "retry-interval", valueName: "MILLISECONDS", help: "Interval in milliseconds before a retry [default: 1000]", heading: "Run options"},
	{id: "secret", long: "secret", valueName: "NAME=VALUE", help: "Define a variable which value is secret", heading: "Run options", multiple: true},
	{id: "secrets_file", long: "secrets-file", valueName: "FILE", help: "Define a secrets file in which you define your secrets", heading: "Run options", multiple: true},
	{id: "test", long: "test", help: "Activate test mode (use parallel execution)", heading: "Run options"},
	{id: "to_entry", long: "to-entry", valueName: "ENTRY_NUMBER", help: "Execute Hurl file to ENTRY_NUMBER (starting at 1)", heading: "Run options", parser: parseU32Min1},
	{id: "variable", long: "variable", valueName: "NAME=VALUE", help: "Define a variable", heading: "Run options", multiple: true},
	{id: "variables_file", long: "variables-file", valueName: "FILE", help: "Define a properties file in which you define your variables", heading: "Run options", multiple: true},
	// Report options
	{id: "report_html", long: "report-html", valueName: "DIR", help: "Generate HTML report to DIR", heading: "Report options"},
	{id: "report_json", long: "report-json", valueName: "DIR", help: "Generate JSON report to DIR", heading: "Report options"},
	{id: "report_junit", long: "report-junit", valueName: "FILE", help: "Write a JUnit XML report to FILE", heading: "Report options"},
	{id: "report_tap", long: "report-tap", valueName: "FILE", help: "Write a TAP report to FILE", heading: "Report options"},
	// Other options
	{id: "cookies_input_file", long: "cookie", short: "b", valueName: "FILE", help: "Read cookies from FILE", heading: "Other options"},
	{id: "cookies_output_file", long: "cookie-jar", short: "c", valueName: "FILE", help: "Write cookies to FILE after running the session", heading: "Other options"},
	{id: "file_root", long: "file-root", valueName: "DIR", help: "Set root directory to import files [default: input file directory]", heading: "Other options"},
	{id: "glob", long: "glob", valueName: "GLOB", help: "Specify input files that match the given GLOB. Multiple glob flags may be used", heading: "Other options", multiple: true},
	{id: "netrc", long: "netrc", short: "n", help: "Must read .netrc for username and password", heading: "Other options", conflicts: []string{"netrc_file", "netrc_optional"}},
	{id: "netrc_file", long: "netrc-file", valueName: "FILE", help: "Specify FILE for .netrc", heading: "Other options", conflicts: []string{"netrc"}},
	{id: "netrc_optional", long: "netrc-optional", help: "Use either .netrc or the URL", heading: "Other options", conflicts: []string{"netrc"}},
	// clap generated
	{id: "help", long: "help", short: "h", help: "Print help"},
	{id: "version", long: "version", short: "V", help: "Print version"},
}

// rustParseInt parses s like Rust's str::parse for an integer type in
// [lo, hi], returning Rust's ParseIntError message on failure.
func rustParseInt(s string, lo, hi int64) (int64, error) {
	digits := s
	neg := false
	if len(digits) > 0 && (digits[0] == '+' || digits[0] == '-') {
		neg = digits[0] == '-'
		digits = digits[1:]
		if lo >= 0 && neg {
			return 0, fmt.Errorf("invalid digit found in string")
		}
	}
	if s == "" {
		return 0, fmt.Errorf("cannot parse integer from empty string")
	}
	if digits == "" {
		return 0, fmt.Errorf("invalid digit found in string")
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit found in string")
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < lo || n > hi {
		if neg {
			return 0, fmt.Errorf("number too small to fit in target type")
		}
		return 0, fmt.Errorf("number too large to fit in target type")
	}
	return n, nil
}

// rustParseUint64 parses s like Rust's str::parse::<u64>.
func rustParseUint64(s string) (uint64, error) {
	digits := s
	if len(digits) > 0 && digits[0] == '+' {
		digits = digits[1:]
	}
	if s == "" {
		return 0, fmt.Errorf("cannot parse integer from empty string")
	}
	if digits == "" {
		return 0, fmt.Errorf("invalid digit found in string")
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid digit found in string")
		}
	}
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("number too large to fit in target type")
	}
	return n, nil
}

// clapRangedI64 is clap's RangedI64ValueParser: parse as an i64, then check
// the bounds.
func clapRangedI64(s string, lo, hi int64) (int64, error) {
	n, err := rustParseInt(s, math.MinInt64, math.MaxInt64)
	if err != nil {
		return 0, err
	}
	if n < lo || n > hi {
		return 0, fmt.Errorf("%d is not in %d..=%d", n, lo, hi)
	}
	return n, nil
}

func parseU32Min1(s string) error {
	_, err := clapRangedI64(s, 1, math.MaxUint32)
	return err
}

func parseI32(s string) error {
	_, err := clapRangedI64(s, math.MinInt32, math.MaxInt32)
	return err
}

func parseU64(s string) error {
	_, err := rustParseUint64(s)
	return err
}
