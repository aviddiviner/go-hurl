package curlimport

import (
	"reflect"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{`AAA BBB`, []string{"AAA", "BBB"}},
		{`AAA  BBB`, []string{"AAA", "BBB"}},
		{` AAA BBB `, []string{"AAA", "BBB"}},
		{`AAA 'BBB'`, []string{"AAA", "BBB"}},
		{`AAA $'BBB'`, []string{"AAA", "BBB"}},
		{`$'\''`, []string{"'"}},
	}
	for _, tt := range tests {
		got, err := split(tt.in)
		if err != nil || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("split(%q) = %q, %v, want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestSplitError(t *testing.T) {
	_, err := split(`AAA 'BBB`)
	if err == nil || err.Error() != "Missing delimiter ' at column 9" {
		t.Errorf("got %v", err)
	}
}

func TestParam(t *testing.T) {
	tests := []struct {
		in   string
		want string
		col  int
	}{
		{"value", "value", 6},
		{" value  ", "value", 8},
		{"'value'", "value", 8},
		{" 'value'  ", "value", 9},
		{`'\n'`, `\n`, 5},
		{`$'Test: \''`, "Test: '", 12},
		{`$'\n'`, "\n", 6},
	}
	for _, tt := range tests {
		p := newArgsParser(tt.in)
		got, ok, err := p.param()
		if err != nil || !ok || got != tt.want {
			t.Errorf("param(%q) = %q, %v, %v, want %q", tt.in, got, ok, err, tt.want)
		}
		if p.col() != tt.col {
			t.Errorf("param(%q): col = %d, want %d", tt.in, p.col(), tt.col)
		}
	}
}

func TestParamMissingClosingQuote(t *testing.T) {
	p := newArgsParser("'value")
	_, _, err := p.param()
	if err == nil || err.Error() != "Missing delimiter ' at column 7" {
		t.Errorf("got %v", err)
	}
	if p.col() != 7 {
		t.Errorf("col = %d", p.col())
	}
}

func TestNoMoreParam(t *testing.T) {
	for _, in := range []string{"", " "} {
		if _, ok, err := newArgsParser(in).param(); ok || err != nil {
			t.Errorf("param(%q) = %v, %v", in, ok, err)
		}
	}
}

func TestDelimiter(t *testing.T) {
	tests := []struct {
		in       string
		ok       bool
		escaping bool
		col      int
	}{
		{"value", false, false, 1},
		{"'value'", true, false, 2},
		{"$'value'", true, true, 3},
		{"$value", false, false, 1},
	}
	for _, tt := range tests {
		p := newArgsParser(tt.in)
		d, escaping, ok := p.delimiter()
		if ok != tt.ok || escaping != tt.escaping || (ok && d != '\'') || p.col() != tt.col {
			t.Errorf("delimiter(%q) = %q, %v, %v, col %d", tt.in, d, escaping, ok, p.col())
		}
	}
}

func TestMissingURL(t *testing.T) {
	for _, in := range []string{"curl", "curl -X POST", "curl -H 'Accept: application/json'"} {
		_, err := Parse(in)
		if err == nil || err.Error() != "Can not parse curl command at line 1: URL is missing" {
			t.Errorf("Parse(%q): %v", in, err)
		}
	}
}

func TestParse(t *testing.T) {
	tests := []struct{ in, want string }{
		{
			"curl http://localhost:8000/hello\ncurl http://localhost:8000/custom-headers -H 'Fruit:Raspberry'\n",
			"GET http://localhost:8000/hello\n\nGET http://localhost:8000/custom-headers\nFruit:Raspberry\n\n",
		},
		{
			"curl http://localhost:8000/custom_headers \\\n                -H 'Fruit:Raspberry' \\\n                -H 'Fruit:Banana'\n",
			"GET http://localhost:8000/custom_headers\nFruit:Raspberry\nFruit:Banana\n\n",
		},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Parse(%q) = %q, %v, want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestParseLine(t *testing.T) {
	tests := []struct{ in, want string }{
		{"curl https://example.com", "GET https://example.com\n"},
		{"curl --url https://example.com", "GET https://example.com\n"},
		{"curl example.com", "GET https://example.com\n"},
		{"curl http://localhost:8000/hello", "GET http://localhost:8000/hello\n"},
		{
			`curl http://localhost:8000/custom-headers -H 'Fruit:Raspberry' -H 'Fruit: Banana' -H $'Test: \''`,
			"GET http://localhost:8000/custom-headers\nFruit:Raspberry\nFruit: Banana\nTest: '\n",
		},
		{
			`curl http://localhost:8000/custom-headers   --header Fruit:Raspberry -H 'Fruit: Banana' -H $'Test: \''  `,
			"GET http://localhost:8000/custom-headers\nFruit:Raspberry\nFruit: Banana\nTest: '\n",
		},
		{
			"curl http://localhost:8000/empty-headers -H 'Empty-Header;'",
			"GET http://localhost:8000/empty-headers\nEmpty-Header:\n",
		},
		{
			"curl http://localhost:8000/custom-cookies -b 'name1=value1' -b 'name2=value2;name3=value3;;'",
			"GET http://localhost:8000/custom-cookies\ncookie: name1=value1; name2=value2; name3=value3\n",
		},
		{
			"curl http://localhost:8000/custom-cookies --cookie 'name1=value1' --cookie 'name2=value2;name3=value3;;'",
			"GET http://localhost:8000/custom-cookies\ncookie: name1=value1; name2=value2; name3=value3\n",
		},
		{
			"curl --digest http://localhost:8000/hello",
			"GET http://localhost:8000/hello\n[Options]\ndigest: true\n",
		},
		{
			`curl -d $'hello'  -H 'Content-Type: text/plain' -X POST http://localhost:8000/hello`,
			"POST http://localhost:8000/hello\nContent-Type: text/plain\n```\nhello\n```\n",
		},
		{
			"curl http://localhost:3000/data -d 'param1=value1&param2=value2'",
			"POST http://localhost:3000/data\nContent-Type: application/x-www-form-urlencoded\n```\nparam1=value1&param2=value2\n```\n",
		},
		{
			"curl -X POST http://localhost:3000/data -H 'Content-Type: application/x-www-form-urlencoded' --data 'param1=value1&param2=value2'",
			"POST http://localhost:3000/data\nContent-Type: application/x-www-form-urlencoded\n```\nparam1=value1&param2=value2\n```\n",
		},
		{
			`curl -d '{"key1":"value1", "key2":"value2"}' -H 'Content-Type: application/json' -X POST http://localhost:3000/data`,
			"POST http://localhost:3000/data\nContent-Type: application/json\n```\n{\"key1\":\"value1\", \"key2\":\"value2\"}\n```\n",
		},
		{
			`curl -d $'{\n  "key1": "value1",\n  "key2": "value2"\n}' -H 'Content-Type: application/json' -X POST http://localhost:3000/data`,
			"POST http://localhost:3000/data\nContent-Type: application/json\n```\n{\n  \"key1\": \"value1\",\n  \"key2\": \"value2\"\n}\n```\n",
		},
		{`curl --data @filename http://example.com/`, "POST http://example.com/\nfile, filename;\n"},
		{
			`curl -L http://localhost:8000/redirect-absolute`,
			"GET http://localhost:8000/redirect-absolute\n[Options]\nlocation: true\n",
		},
		{`curl -k https://localhost:8001/hello`, "GET https://localhost:8001/hello\n[Options]\ninsecure: true\n"},
		{
			`curl https://localhost:8001/hello --max-redirs 10`,
			"GET https://localhost:8001/hello\n[Options]\nmax-redirs: 10\n",
		},
		{"curl -v http://localhost:8000/hello", "GET http://localhost:8000/hello\n[Options]\nverbose: true\n"},
		{"curl --verbose http://localhost:8000/hello", "GET http://localhost:8000/hello\n[Options]\nverbose: true\n"},
		{
			"curl -u 'test_user:test_pass' http://localhost:8000/hello",
			"GET http://localhost:8000/hello\n[Options]\nuser: test_user:test_pass\n",
		},
		{
			"curl --user 'test_user:test_pass' http://localhost:8000/hello",
			"GET http://localhost:8000/hello\n[Options]\nuser: test_user:test_pass\n",
		},
		{"curl --ntlm http://localhost:8000/hello", "GET http://localhost:8000/hello\n[Options]\nntlm: true\n"},
		{"curl --negotiate http://localhost:8000/hello", "GET http://localhost:8000/hello\n[Options]\nnegotiate: true\n"},
		{
			`curl --data-raw @filename http://example.com/`,
			"POST http://example.com/\nContent-Type: application/x-www-form-urlencoded\n```\n@filename\n```\n",
		},
		{
			`curl --data-raw 'hello' -H 'Content-Type: text/plain' -X POST http://localhost:8000/hello`,
			"POST http://localhost:8000/hello\nContent-Type: text/plain\n```\nhello\n```\n",
		},
		{
			`curl --data-raw '{"key1":"value1", "key2":"value2"}' -H 'Content-Type: application/json' -X POST http://localhost:3000/data`,
			"POST http://localhost:3000/data\nContent-Type: application/json\n```\n{\"key1\":\"value1\", \"key2\":\"value2\"}\n```\n",
		},
		// Checked against hurlfmt 8.0.1.
		{"curl -Lk http://a", "GET http://a\n[Options]\nlocation: true\ninsecure: true\n"},
		{"curl -XPOST http://a", "POST http://a\n"},
		{"curl --request=PUT http://a", "PUT http://a\n"},
		{"curl -H=a:b http://a", "GET http://a\na:b\n"},
		{"curl -- -x", "GET https://-x\n"},
		{"curl --max-redirs -1 http://a", "GET http://a\n[Options]\nmax-redirs: -1\n"},
		{"curl --retry 4 http://a", "GET http://a\n[Options]\nretry: 4\nHTTP *\n[Asserts]\nstatus < 500\n"},
		{"curl -d a --data-raw b http://a", "POST http://a\nContent-Type: application/x-www-form-urlencoded\n```\nb\n```\n"},
		{"curl -d a --data-raw b -d c http://a", "POST http://a\nContent-Type: application/x-www-form-urlencoded\n```\nc\n```\n"},
		{"curl --url a b", "GET https://a\n"},
		{"wget http://a", "GET http://a\n"},
		{"curl -b ' ; ' http://a", "GET http://a\ncookie: \n"},
	}
	for _, tt := range tests {
		got, err := parseLine(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseLine(%q) = %q, %v, want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestParseLineErrorContains(t *testing.T) {
	tests := []struct{ in, want string }{
		{"curl http://localhost:8000/illegal-header -H 'Illegal-Header'", "headers must be formatted as '<NAME:VALUE>' or '<NAME>;'"},
		{"curl http://localhost:8000/empty-cookie -b 'valid=pair' -b ''", "empty value provided"},
		{"curl http://localhost:8000/empty-cookie -b 'valid=pair' -b 'invalid'", "invalid cookie pair provided"},
		{"curl http://localhost:8000/empty-cookie -b 'name=value' -b 'valid=pair; invalid-1; invalid-2'", "invalid cookie pairs provided: [invalid-1, invalid-2]"},
	}
	for _, tt := range tests {
		_, err := parseLine(tt.in)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("parseLine(%q) = %v, want error containing %q", tt.in, err, tt.want)
		}
	}
}

// TestParseLineError checks clap's error messages, as given by hurlfmt 8.0.1.
func TestParseLineError(t *testing.T) {
	const usage = "Usage: curl [OPTIONS] [url_param]\n\nFor more information, try '--help'.\n"
	const try = "For more information, try '--help'.\n"
	tests := []struct{ in, want string }{
		{"curl -v -v http://a", "error: the argument '--verbose' cannot be used multiple times\n\n" + usage},
		{"curl -d a -d b http://a", "error: the argument '--data <data>' cannot be used multiple times\n\n" + usage},
		{"curl --url a --url b", "error: the argument '--url <url>' cannot be used multiple times\n\n" + usage},
		{"curl --foo http://a", "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\n" + usage},
		{"curl -vx http://a", "error: unexpected argument '-x' found\n\n  tip: to pass '-x' as a value, use '-- -x'\n\n" + usage},
		{"curl --retry -1 http://a", "error: unexpected argument '-1' found\n\n  tip: to pass '-1' as a value, use '-- -1'\n\n" + usage},
		{"curl http://a http://b", "error: unexpected argument 'http://b' found\n\n" + usage},
		{
			"curl --verbos http://a",
			"error: unexpected argument '--verbos' found\n\n  tip: a similar argument exists: '--verbose'\n\nUsage: curl --verbose [url_param]\n\n" + try,
		},
		{
			"curl -v --verbos http://a",
			"error: unexpected argument '--verbos' found\n\n  tip: a similar argument exists: '--verbose'\n\nUsage: curl --verbose [url_param]\n\n" + try,
		},
		{
			"curl --verbose=x http://a",
			"error: unexpected value 'x' for '--verbose' found; no more were expected\n\nUsage: curl --verbose [url_param]\n\n" + try,
		},
		{
			"curl --compressed=1 http://a",
			"error: unexpected value '1' for '--compressed' found; no more were expected\n\nUsage: curl --compressed [url_param]\n\n" + try,
		},
		{"curl -X", "error: a value is required for '--request <METHOD>' but none was supplied\n\n" + try},
		{"curl -v --foo http://a", "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\nUsage: curl --verbose [url_param]\n\n" + try},
		{"curl -X --foo", "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\n" + usage},
		{
			"curl http://a --verbos",
			"error: unexpected argument '--verbos' found\n\n  tip: a similar argument exists: '--verbose'\n\nUsage: curl --verbose <url_param>\n\n" + try,
		},
		{
			"curl --hel",
			"error: unexpected argument '--hel' found\n\n  tip: a similar argument exists: '--help'\n\nUsage: curl --help [url_param]\n\n" + try,
		},
		{"curl -u", "error: a value is required for '--user <user>' but none was supplied\n\n" + try},
		{"curl -X --verbose http://a", "error: a value is required for '--request <METHOD>' but none was supplied\n\n" + try},
		{"curl -b invalid http://a", "error: invalid value 'invalid' for '--cookie <NAME1=VALUE1; NAME2=VALUE2>': invalid cookie pair provided\n\n" + try},
		{"curl -H x http://a", "error: invalid value 'x' for '--header <NAME:VALUE>': headers must be formatted as '<NAME:VALUE>' or '<NAME>;'\n\n" + try},
		{"curl --header= http://a", "error: invalid value '' for '--header <NAME:VALUE>': headers must be formatted as '<NAME:VALUE>' or '<NAME>;'\n\n" + try},
		{"curl --retry abc http://a", "error: invalid value 'abc' for '--retry <seconds>': invalid digit found in string\n\n" + try},
		{"curl --retry 99999999999 http://a", "error: invalid value '99999999999' for '--retry <seconds>': 99999999999 is not in -2147483648..=2147483647\n\n" + try},
		{"curl --max-redirs -2 http://a", "error: invalid value '-2' for '--max-redirs <NUM>': -2 is not in -1..=2147483647\n\n" + try},
		{"curl --max-redirs x http://a", "error: invalid value 'x' for '--max-redirs <NUM>': invalid digit found in string\n\n" + try},
		{"curl -h -v -v", curlHelp},
		{"wget -h", strings.Replace(curlHelp, "Usage: curl ", "Usage: wget ", 1)},
		{"Content-Type: text/html; charset=utf-8", "error: unexpected argument 'charset=utf-8' found\n\nUsage: Content-Type: [OPTIONS] [url_param]\n\n" + try},
		{"./bin/curl --foo", "error: unexpected argument '--foo' found\n\n  tip: to pass '--foo' as a value, use '-- --foo'\n\n" + usage},
		{"curl --help", curlHelp},
		{"curl 'abc", "Missing delimiter ' at column 10"},
	}
	for _, tt := range tests {
		_, err := parseLine(tt.in)
		if err == nil || err.Error() != tt.want {
			t.Errorf("parseLine(%q):\ngot:  %q\nwant: %q", tt.in, err, tt.want)
		}
	}
}

func TestJaro(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"", "", 1},
		{"foo", "", 0},
		{"martha", "marhta", 0.9444444444444445},
		{"dwayne", "duane", 0.8222222222222223},
	}
	for _, tt := range tests {
		if got := jaro(tt.a, tt.b); got != tt.want {
			t.Errorf("jaro(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
