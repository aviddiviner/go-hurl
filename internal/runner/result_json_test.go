package runner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/reader"
)

func srcInfo(line, col, endLine, endCol int) ast.SourceInfo {
	return ast.SourceInfo{Start: reader.Pos{Line: line, Column: col}, End: reader.Pos{Line: endLine, Column: endCol}}
}

func mustParseUrl(t *testing.T, s string) httpclient.Url {
	t.Helper()
	u, err := httpclient.ParseUrl(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func curlCmd(t *testing.T, url string) httpclient.CurlCmd {
	spec := httpclient.NewRequestSpec()
	spec.Method = "GET"
	spec.Url = mustParseUrl(t, url)
	return httpclient.NewCurlCmd(spec, httpclient.NewCookieStore(), util.ContextDir{}, nil, httpclient.DefaultClientOptions())
}

func testTimings() httpclient.Timings {
	begin := time.Date(2026, 11, 2, 10, 0, 0, 123456789, time.UTC)
	return httpclient.Timings{
		BeginCall:     begin,
		EndCall:       begin.Add(5 * time.Millisecond),
		NameLookup:    1001 * time.Microsecond,
		Connect:       2002 * time.Microsecond,
		AppConnect:    0,
		PreTransfer:   3003 * time.Microsecond,
		StartTransfer: 4004 * time.Microsecond,
		Total:         5005*time.Microsecond + 999*time.Nanosecond,
	}
}

const testTimingsJSON = `{"app_connect":0,"begin_call":"2026-11-02T10:00:00.123456Z","connect":2002,"end_call":"2026-11-02T10:00:00.128456Z","name_lookup":1001,"pre_transfer":3003,"start_transfer":4004,"total":5005}`

// Reproduces integration/hurl/tests_failed/assert_status/assert_status.out.pattern.
func TestHurlResultToJSONAssertStatus(t *testing.T) {
	const filename = "tests_failed/assert_status/assert_status.hurl"
	content, err := os.ReadFile(filepath.Join("../../integration/hurl", filename))
	if err != nil {
		t.Fatal(err)
	}
	url := "http://localhost:8000/not_found"
	entry := newEntryResult(ast.NewIndex(1), srcInfo(7, 1, 7, 36))
	entry.Calls = []httpclient.Call{{
		Request: httpclient.NewRequest("GET", mustParseUrl(t, url), httpclient.NewHeaderVec(
			httpclient.NewHeader("Host", "localhost:8000"),
			httpclient.NewHeader("Accept", "*/*"),
			httpclient.NewHeader("User-Agent", "hurl/8.1.0"),
		), nil),
		Response: httpclient.Response{
			Version: httpclient.HttpVersionHttp11,
			Status:  404,
			Headers: httpclient.NewHeaderVec(
				httpclient.NewHeader("Content-Length", "207"),
				httpclient.NewHeader("Content-Type", "text/html; charset=utf-8"),
				httpclient.NewHeader("Date", "Mon, 02 Nov 2026 10:00:00 GMT"),
				httpclient.NewHeader("Server", "Flask Server"),
				httpclient.NewHeader("Via", "waitress"),
			),
		},
		Timings: testTimings(),
	}}
	entry.Asserts = []AssertResult{
		AssertResultImplicitVersion{Actual: "HTTP/1.1", Expected: "HTTP", SourceInfo: srcInfo(9, 1, 9, 5)},
		AssertResultImplicitStatus{Actual: 404, Expected: 200, SourceInfo: srcInfo(9, 6, 9, 9)},
	}
	entry.TransferDuration = 7*time.Millisecond + 900*time.Microsecond
	entry.CurlCmd = curlCmd(t, url)
	result := HurlResult{
		Entries:     []EntryResult{entry},
		Duration:    12 * time.Millisecond,
		Success:     false,
		CookieStore: httpclient.NewCookieStore(),
	}

	v, err := result.ToJSON(string(content), input.New(filename), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"cookies":[],"entries":[{"asserts":[{"line":9,"success":true},{"line":9,"message":"Assert status code\n  --> tests_failed/assert_status/assert_status.hurl:9:6\n   |\n   | GET http://localhost:8000/not_found\n   | ...\n 9 | HTTP 200\n   |      ^^^ actual value is <404>\n   |","success":false}],"calls":[{"request":{"cookies":[],"headers":[{"name":"Host","value":"localhost:8000"},{"name":"Accept","value":"*/*"},{"name":"User-Agent","value":"hurl/8.1.0"}],"method":"GET","query_string":[],"url":"http://localhost:8000/not_found"},"response":{"cookies":[],"headers":[{"name":"Content-Length","value":"207"},{"name":"Content-Type","value":"text/html; charset=utf-8"},{"name":"Date","value":"Mon, 02 Nov 2026 10:00:00 GMT"},{"name":"Server","value":"Flask Server"},{"name":"Via","value":"waitress"}],"http_version":"HTTP/1.1","status":404},"timings":` + testTimingsJSON + `}],"captures":[],"curl_cmd":"curl 'http://localhost:8000/not_found'","index":1,"line":7,"time":7}],"filename":"tests_failed/assert_status/assert_status.hurl","success":false,"time":12}`
	if got := string(EncodeJSON(v)); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if !IsDeserializable(v) {
		t.Error("result is not deserializable")
	}
}

func TestHurlResultToJSONFull(t *testing.T) {
	secrets := []string{"s3cr3t"}
	url := "http://localhost:8000/login?user=bob&token=s3cr3t"
	next := mustParseUrl(t, "http://localhost:8000/next")
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	subject, serial := "CN=localhost", "1e:2f"

	entry := newEntryResult(ast.NewIndex(2), srcInfo(4, 1, 4, 10))
	entry.Calls = []httpclient.Call{{
		Request: httpclient.NewRequest("GET", mustParseUrl(t, url), httpclient.NewHeaderVec(
			httpclient.NewHeader("Host", "localhost:8000"),
			httpclient.NewHeader("Cookie", "session=s3cr3t; theme=dark"),
			httpclient.NewHeader("Authorization", "Bearer s3cr3t"),
		), nil),
		Response: httpclient.Response{
			Version: httpclient.HttpVersionHttp2,
			Status:  201,
			Headers: httpclient.NewHeaderVec(
				httpclient.NewHeader("Content-Type", "application/json"),
				httpclient.NewHeader("Set-Cookie", "id=s3cr3t; Max-Age=60; Path=/; Domain=localhost; Secure; HttpOnly; SameSite=Lax; Expires=Wed, 13 Jan 2078 22:23:01 GMT"),
				httpclient.NewHeader("Set-Cookie", "lang=en"),
			),
			Body:        []byte(`{"token":"s3cr3t"}`),
			Certificate: httpclient.NewCertificate(&subject, nil, &start, nil, &serial, nil, nil),
		},
		Timings: testTimings(),
	}}
	entry.Captures = []CaptureResult{
		{"token", NewString("abc s3cr3t")},
		{"int", NewInteger(-1)},
		{"float", NewFloat(1)},
		{"big", ValueNumber{NumberBigInteger{"123456789012345678901234567890"}}},
		{"list", ValueList{[]Value{NewBool(true), NewString("s3cr3t"), ValueNull{}}}},
		{"object", ValueObject{[]KeyValue{{"b", NewInteger(1)}, {"a", NewString("x")}, {"b", NewInteger(2)}}}},
		{"nodeset", ValueNodeset{3}},
		{"unit", ValueUnit{}},
		{"bytes", ValueBytes{[]byte("hello")}},
		{"date", ValueDate{time.Date(2078, 1, 13, 22, 23, 1, 0, time.UTC)}},
		{"regex", ValueRegex{regexp.MustCompile(`a/b\d+`)}},
		{"redirect", ValueHTTPResponse{NewHTTPResponse(&next, 302)}},
		{"response", ValueHTTPResponse{NewHTTPResponse(nil, 200)}},
		{"text", NewString("é\x01\"<>&\u2028")},
	}
	entry.Asserts = []AssertResult{
		AssertResultExplicit{Actual: NewString("s3cr3t"), SourceInfo: srcInfo(6, 1, 6, 10)},
	}
	entry.TransferDuration = 3 * time.Millisecond
	entry.CurlCmd = curlCmd(t, url)

	store := httpclient.NewCookieStore()
	if err := store.AddCookie("#HttpOnly_.localhost\tTRUE\t/\tTRUE\t1\tid\ts3cr3t"); err != nil {
		t.Fatal(err)
	}
	result := HurlResult{
		Entries:     []EntryResult{entry},
		Duration:    1500 * time.Microsecond,
		Success:     true,
		CookieStore: store,
	}

	responseDir := filepath.Join(t.TempDir(), "store")
	if err := os.Mkdir(responseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	v, err := result.ToJSON("", input.New("test.hurl"), responseDir+"/", secrets)
	if err != nil {
		t.Fatal(err)
	}
	got := string(EncodeJSON(v))

	bodyRe := regexp.MustCompile(`"body":"store/([0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}_response\.json)"`)
	m := bodyRe.FindStringSubmatch(got)
	if m == nil {
		t.Fatalf("no body file in %s", got)
	}
	// Response bodies are saved as is, without redaction.
	if body, err := os.ReadFile(filepath.Join(responseDir, m[1])); err != nil || string(body) != `{"token":"s3cr3t"}` {
		t.Errorf("body file: %q, %v", body, err)
	}
	got = bodyRe.ReplaceAllString(got, `"body":"store/BODY"`)

	want := `{"cookies":[{"domain":".localhost","expires":1,"https":true,"include_subdomain":true,"name":"id","path":"/","value":"***"}],` +
		`"entries":[{"asserts":[{"line":6,"success":true}],` +
		`"calls":[{"request":{"cookies":[{"name":"session","value":"***"},{"name":"theme","value":"dark"}],` +
		`"headers":[{"name":"Host","value":"localhost:8000"},{"name":"Cookie","value":"session=***; theme=dark"},{"name":"Authorization","value":"Bearer ***"}],` +
		`"method":"GET","query_string":[{"name":"user","value":"bob"},{"name":"token","value":"***"}],"url":"http://localhost:8000/login?user=bob&token=***"},` +
		`"response":{"body":"store/BODY","certificate":{"serial_number":"1e:2f","start_date":"2025-01-01 00:00:00 UTC","subject":"CN=localhost"},` +
		`"cookies":[{"domain":"localhost","expires":"Wed, 13 Jan 2078 22:23:01 GMT","httponly":true,"max_age":"60","name":"id","path":"/","same_site":"Lax","secure":true,"value":"***"},{"name":"lang","value":"en"}],` +
		`"headers":[{"name":"Content-Type","value":"application/json"},{"name":"Set-Cookie","value":"id=***; Max-Age=60; Path=/; Domain=localhost; Secure; HttpOnly; SameSite=Lax; Expires=Wed, 13 Jan 2078 22:23:01 GMT"},{"name":"Set-Cookie","value":"lang=en"}],` +
		`"http_version":"HTTP/2","status":201},"timings":` + testTimingsJSON + `}],` +
		`"captures":[{"name":"token","value":"abc ***"},{"name":"int","value":-1},{"name":"float","value":1.0},` +
		`{"name":"big","value":123456789012345678901234567890},{"name":"list","value":[true,"***",null]},` +
		`{"name":"object","value":{"a":"x","b":2}},{"name":"nodeset","value":{"size":3,"type":"nodeset"}},` +
		`{"name":"unit","value":{"type":"unit"}},{"name":"bytes","value":"aGVsbG8="},{"name":"date","value":"2078-01-13 22:23:01 UTC"},` +
		`{"name":"regex","value":"a/b\\d+"},{"name":"redirect","value":{"location":"http://localhost:8000/next","status":302}},` +
		`{"name":"response","value":{"location":"None","status":200}},{"name":"text","value":"é\u0001\"<>&` + "\u2028" + `"}],` +
		`"curl_cmd":"curl 'http://localhost:8000/login?user=bob&token=***'","index":2,"line":4,"time":3}],` +
		`"filename":"test.hurl","success":true,"time":1}`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if !IsDeserializable(v) {
		t.Error("result is not deserializable")
	}
}

func TestFormatJSONFloat(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{1, "1.0"},
		{-1, "-1.0"},
		{0, "0.0"},
		{0.1, "0.1"},
		{1.5, "1.5"},
		{123.456, "123.456"},
		{100, "100.0"},
		{1e15, "1000000000000000.0"},
		{1e16, "1e+16"},
		{1.5e16, "1.5e+16"},
		{0.00001, "0.00001"},
		{0.000001, "1e-6"},
		{1.5e-7, "1.5e-7"},
		{6.62607015e-34, "6.62607015e-34"},
		{5.444310685350916e+14, "544431068535091.6"},
		{2.9802322387695312e-08, "2.9802322387695312e-8"},
		{3.439070283483335e+35, "3.439070283483335e+35"},
		{5e-324, "5e-324"},
		{2.2250738585072004e-308, "2.2250738585072004e-308"},
		{1.7976931348623157e308, "1.7976931348623157e+308"},
	}
	for _, tt := range tests {
		if got := formatJSONFloat(tt.value); got != tt.want {
			t.Errorf("formatJSONFloat(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestNumberToJSON(t *testing.T) {
	negZero := 0.0
	negZero = -negZero
	tests := []struct {
		number Number
		want   string
	}{
		{NumberInteger{1}, "1"},
		{NumberInteger{-9223372036854775808}, "-9223372036854775808"},
		{NumberFloat{1}, "1.0"},
		{NumberFloat{negZero}, "-0.0"},
		{NumberFloat{-1.1}, "-1.1"},
		{NumberBigInteger{"-18446744073709551616"}, "-18446744073709551616"},
	}
	for _, tt := range tests {
		if got := string(EncodeJSON(numberToJSON(tt.number))); got != tt.want {
			t.Errorf("numberToJSON(%v) = %s, want %s", tt.number, got, tt.want)
		}
	}
}

func TestDecodeEncodeJSON(t *testing.T) {
	v, err := DecodeJSON([]byte(` {"b":1E5,"a":[1.0e3,2e-1,-0,123456789012345678901234567890],"c":{"z":null,"y":"\u00e9\n"},"b":true} `))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a":[1.0e+3,2e-1,-0,123456789012345678901234567890],"b":true,"c":{"y":"é\n","z":null}}`
	if got := string(EncodeJSON(v)); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, err := DecodeJSON([]byte(`[] x`)); err == nil {
		t.Error("expected an error")
	}
}

func TestIsDeserializable(t *testing.T) {
	entry := `{"index":1,"line":1,"calls":[{"request":{"method":"GET","url":"http://foo","headers":[],"cookies":[],"query_string":[]},` +
		`"response":{"http_version":"HTTP/1.1","status":STATUS,"headers":[{"name":"a","value":"b"}],"cookies":[{"name":"a","value":"b","secure":SECURE}]CERT},` +
		`"timings":{"begin_call":"","end_call":"","name_lookup":0,"connect":0,"app_connect":0,"pre_transfer":0,"start_transfer":0,"total":0}}],` +
		`"captures":[{"name":"a"CAPTURE}],"asserts":[{"success":true,"line":1MESSAGE}],"time":TIME,"curl_cmd":""}`
	result := `{"filename":"a.hurl","entries":[ENTRY],"success":true,"time":0,"cookies":[],"extra":1}`
	replacements := map[string]string{
		"STATUS": "200", "SECURE": "true", "CERT": "", "CAPTURE": `,"value":null`, "MESSAGE": "", "TIME": "0",
	}
	build := func(overrides map[string]string) string {
		s := entry
		for k, v := range replacements {
			if o, ok := overrides[k]; ok {
				v = o
			}
			s = strings.ReplaceAll(s, k, v)
		}
		return strings.ReplaceAll(result, "ENTRY", s)
	}

	tests := []struct {
		name      string
		overrides map[string]string
		want      bool
	}{
		{"valid", nil, true},
		{"status u32 max", map[string]string{"STATUS": "4294967295"}, true},
		{"status too big", map[string]string{"STATUS": "4294967296"}, false},
		{"status negative", map[string]string{"STATUS": "-1"}, false},
		{"status float", map[string]string{"STATUS": "200.0"}, false},
		{"status string", map[string]string{"STATUS": `"200"`}, false},
		{"time minus zero", map[string]string{"TIME": "-0"}, true},
		{"time u64 max", map[string]string{"TIME": "18446744073709551615"}, true},
		{"time too big", map[string]string{"TIME": "18446744073709551616"}, false},
		{"time exponent", map[string]string{"TIME": "1e2"}, false},
		{"optional null", map[string]string{"SECURE": "null"}, true},
		{"optional wrong type", map[string]string{"SECURE": `"yes"`}, false},
		{"certificate null", map[string]string{"CERT": `,"certificate":null`}, true},
		{"certificate", map[string]string{"CERT": `,"certificate":{"subject":"a","foo":1}`}, true},
		{"certificate as array", map[string]string{"CERT": `,"certificate":[null,null,null,null,null,null,"v"]`}, true},
		{"certificate short array", map[string]string{"CERT": `,"certificate":[null]`}, false},
		{"certificate wrong type", map[string]string{"CERT": `,"certificate":"a"`}, false},
		{"capture any value", map[string]string{"CAPTURE": `,"value":{"a":[1.5]}`}, true},
		{"capture missing value", map[string]string{"CAPTURE": ""}, false},
		{"message", map[string]string{"MESSAGE": `,"message":"m"`}, true},
		{"message wrong type", map[string]string{"MESSAGE": `,"message":1`}, false},
	}
	for _, tt := range tests {
		v, err := DecodeJSON([]byte(build(tt.overrides)))
		if err != nil {
			t.Fatalf("%s: %v", tt.name, err)
		}
		if got := IsDeserializable(v); got != tt.want {
			t.Errorf("%s: IsDeserializable = %v, want %v", tt.name, got, tt.want)
		}
	}

	for _, s := range []string{
		`null`,
		`[]`,
		`{"filename":"a.hurl","entries":[],"success":true,"time":0}`,
		`{"filename":"a.hurl","entries":{},"success":true,"time":0,"cookies":[]}`,
		`{"filename":"a.hurl","entries":[],"success":1,"time":0,"cookies":[]}`,
		`["a.hurl",[],true,0,[],1]`,
	} {
		v, err := DecodeJSON([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		if IsDeserializable(v) {
			t.Errorf("IsDeserializable(%s) = true", s)
		}
	}
	for _, s := range []string{
		`["a.hurl",[],true,0,[]]`,
		`{"filename":"a.hurl","entries":[],"success":true,"time":0,"cookies":[{"domain":"","include_subdomain":false,"path":"","https":false,"expires":0,"name":"","value":""}]}`,
	} {
		v, err := DecodeJSON([]byte(s))
		if err != nil {
			t.Fatal(err)
		}
		if !IsDeserializable(v) {
			t.Errorf("IsDeserializable(%s) = false", s)
		}
	}
}
