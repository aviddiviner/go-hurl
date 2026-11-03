package runner

import (
	"bytes"

	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/sourceerr"
	"github.com/aviddiviner/go-hurl/internal/timefmt"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// JSON values built and read here are serde_json::Value trees: nil, bool,
// json.Number (the number literal), string, []any and map[string]any.
// Objects are always written in sorted key order, like serde_json without
// preserve_order: Hurl converts its result structs with serde_json::to_value,
// so their field order is lost.

// ToJSON serializes r to a JSON value, written with EncodeJSON.
//
// content is the Hurl file content, used to render assert error messages. If
// responseDir is not empty, each HTTP response body is saved in this
// directory and referenced in the "body" field (the JSON report). secrets are
// redacted from the JSON fields.
func (r *HurlResult) ToJSON(content string, filename input.Input, responseDir string, secrets []string) (any, error) {
	entries := make([]any, 0, len(r.Entries))
	for i := range r.Entries {
		e, err := entryResultJSON(&r.Entries[i], content, filename, responseDir, secrets)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	cookies := []any{}
	if r.CookieStore != nil {
		for _, c := range r.CookieStore.Cookies() {
			cookies = append(cookies, cookieJSON(c, secrets))
		}
	}
	return map[string]any{
		"filename": filename.String(),
		"entries":  entries,
		"success":  r.Success,
		"time":     jsonInt(r.Duration.Milliseconds()),
		"cookies":  cookies,
	}, nil
}

func entryResultJSON(e *EntryResult, content string, filename input.Input, responseDir string, secrets []string) (any, error) {
	calls := make([]any, 0, len(e.Calls))
	for i := range e.Calls {
		c, err := callJSON(&e.Calls[i], responseDir, secrets)
		if err != nil {
			return nil, err
		}
		calls = append(calls, c)
	}
	captures := make([]any, 0, len(e.Captures))
	for _, c := range e.Captures {
		captures = append(captures, map[string]any{"name": c.Name, "value": valueToJSON(c.Value, secrets)})
	}
	asserts := make([]any, 0, len(e.Asserts))
	for _, a := range e.Asserts {
		asserts = append(asserts, assertJSON(a, content, filename, e, secrets))
	}
	return map[string]any{
		"index":    jsonInt(int64(e.EntryIndex.Get())),
		"line":     jsonInt(int64(e.SourceInfo.Start.Line)),
		"calls":    calls,
		"captures": captures,
		"asserts":  asserts,
		"time":     jsonInt(e.TransferDuration.Milliseconds()),
		"curl_cmd": e.CurlCmd.ToRedactedString(secrets),
	}, nil
}

func cookieJSON(c httpclient.Cookie, secrets []string) any {
	return map[string]any{
		"domain":            c.Domain(),
		"include_subdomain": c.IncludeSubdomain(),
		"path":              c.Path(),
		"https":             c.Https(),
		"expires":           json.Number(strconv.FormatUint(c.Expires(), 10)),
		"name":              c.Name(),
		"value":             util.Redact(c.Value(), secrets),
	}
}

func callJSON(c *httpclient.Call, responseDir string, secrets []string) (any, error) {
	response, err := responseJSON(&c.Response, responseDir, secrets)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"request":  requestJSON(&c.Request, secrets),
		"response": response,
		"timings":  timingsJSON(c.Timings),
	}, nil
}

func nameValueJSON(name, value string, secrets []string) any {
	return map[string]any{"name": name, "value": util.Redact(value, secrets)}
}

func requestJSON(r *httpclient.Request, secrets []string) any {
	headers := make([]any, 0, r.Headers.Len())
	for _, h := range r.Headers.All() {
		headers = append(headers, nameValueJSON(h.Name, h.Value, secrets))
	}
	cookies := []any{}
	for _, c := range r.Cookies() {
		cookies = append(cookies, nameValueJSON(c.Name, c.Value, secrets))
	}
	queryString := []any{}
	for _, p := range r.Url.QueryParams() {
		queryString = append(queryString, nameValueJSON(p.Name, p.Value, secrets))
	}
	return map[string]any{
		"method":       r.Method,
		"url":          util.Redact(r.Url.String(), secrets),
		"headers":      headers,
		"cookies":      cookies,
		"query_string": queryString,
	}
}

func responseJSON(r *httpclient.Response, responseDir string, secrets []string) (any, error) {
	headers := make([]any, 0, r.Headers.Len())
	for _, h := range r.Headers.All() {
		headers = append(headers, nameValueJSON(h.Name, h.Value, secrets))
	}
	cookies := []any{}
	for _, c := range r.Cookies() {
		cookies = append(cookies, responseCookieJSON(c, secrets))
	}
	m := map[string]any{
		"http_version": r.Version.String(),
		"status":       json.Number(strconv.FormatUint(uint64(r.Status), 10)),
		"headers":      headers,
		"cookies":      cookies,
	}
	if r.Certificate != nil {
		m["certificate"] = certificateJSON(r.Certificate)
	}
	if responseDir != "" {
		file, err := writeResponse(r, responseDir)
		if err != nil {
			return nil, err
		}
		m["body"] = filepath.Base(filepath.Clean(responseDir)) + "/" + file
	}
	return m, nil
}

func responseCookieJSON(c httpclient.ResponseCookie, secrets []string) any {
	m := map[string]any{"name": c.Name, "value": util.Redact(c.Value, secrets)}
	if v, ok := c.Expires(); ok {
		m["expires"] = v
	}
	if v, ok := c.MaxAge(); ok {
		m["max_age"] = strconv.FormatInt(v, 10)
	}
	if v, ok := c.Domain(); ok {
		m["domain"] = v
	}
	if v, ok := c.Path(); ok {
		m["path"] = v
	}
	if c.HasSecure() {
		m["secure"] = true
	}
	if c.HasHttpOnly() {
		m["httponly"] = true
	}
	if v, ok := c.SameSite(); ok {
		m["same_site"] = v
	}
	return m
}

func certificateJSON(c *httpclient.Certificate) any {
	m := map[string]any{}
	setString := func(key string, v *string) {
		if v != nil {
			m[key] = *v
		}
	}
	setDate := func(key string, v *time.Time) {
		if v != nil {
			m[key] = timefmt.Display(v.UTC())
		}
	}
	setString("subject", c.Subject())
	setString("issuer", c.Issuer())
	setDate("start_date", c.StartDate())
	setDate("expire_date", c.ExpireDate())
	setString("serial_number", c.SerialNumber())
	setString("subject_alt_name", c.SubjectAltName())
	setString("value", c.Value())
	return m
}

func timingsJSON(t httpclient.Timings) any {
	const rfc3339Micros = "2006-01-02T15:04:05.000000Z"
	return map[string]any{
		"begin_call":     t.BeginCall.UTC().Format(rfc3339Micros),
		"end_call":       t.EndCall.UTC().Format(rfc3339Micros),
		"name_lookup":    jsonInt(t.NameLookup.Microseconds()),
		"connect":        jsonInt(t.Connect.Microseconds()),
		"app_connect":    jsonInt(t.AppConnect.Microseconds()),
		"pre_transfer":   jsonInt(t.PreTransfer.Microseconds()),
		"start_transfer": jsonInt(t.StartTransfer.Microseconds()),
		"total":          jsonInt(t.Total.Microseconds()),
	}
}

func assertJSON(a AssertResult, content string, filename input.Input, e *EntryResult, secrets []string) any {
	err := a.ToRunnerError()
	m := map[string]any{"success": err == nil, "line": jsonInt(int64(a.Line()))}
	if err != nil {
		entrySrcInfo := e.SourceInfo
		message := sourceerr.Render(err, filename.String(), content, &entrySrcInfo, sourceerr.OutputFormatPlain{})
		m["message"] = util.Redact(message, secrets)
	}
	return m
}

// writeResponse writes the body of r to a new file in dir, and returns the
// file name.
func writeResponse(r *httpclient.Response, dir string) (string, error) {
	name := newUUIDv4() + "_response"
	switch {
	case r.IsJSON():
		name += ".json"
	case r.IsXML():
		name += ".xml"
	case r.IsHTML():
		name += ".html"
	}
	if err := os.WriteFile(filepath.Join(dir, name), r.Body, 0o666); err != nil {
		return "", err
	}
	return name, nil
}

// valueToJSON is Value::to_json, used for captures: natural JSON types are
// used where possible, and secrets are redacted from strings.
func valueToJSON(v Value, secrets []string) any {
	switch v := v.(type) {
	case ValueBool:
		return v.Value
	case ValueDate:
		return timefmt.Display(v.Value)
	case ValueNumber:
		return numberToJSON(v.Value)
	case ValueString:
		return util.Redact(v.Value, secrets)
	case ValueList:
		values := make([]any, 0, len(v.Value))
		for _, e := range v.Value {
			values = append(values, valueToJSON(e, secrets))
		}
		return values
	case ValueObject:
		m := make(map[string]any, len(v.Value))
		for _, kv := range v.Value {
			m[kv.Key] = valueToJSON(kv.Value, secrets)
		}
		return m
	case ValueNodeset:
		return map[string]any{"type": "nodeset", "size": jsonInt(int64(v.Value))}
	case ValueBytes:
		return base64.StdEncoding.EncodeToString(v.Value)
	case ValueNull:
		return nil
	case ValueRegex:
		return v.Value.String()
	case ValueUnit:
		return map[string]any{"type": "unit"}
	case ValueHTTPResponse:
		location := "None"
		if loc := v.Value.Location(); loc != nil {
			location = loc.Raw()
		}
		return map[string]any{
			"location": location,
			"status":   json.Number(strconv.FormatUint(uint64(v.Value.Status()), 10)),
		}
	}
	panic(fmt.Sprintf("unknown value %T", v))
}

// numberToJSON is Number::to_json. Non-finite floats, which make Hurl panic,
// are serialized as null.
func numberToJSON(n Number) any {
	switch n := n.(type) {
	case NumberInteger:
		return jsonInt(n.Value)
	case NumberFloat:
		if math.IsInf(n.Value, 0) || math.IsNaN(n.Value) {
			return nil
		}
		return json.Number(formatJSONFloat(n.Value))
	case NumberBigInteger:
		return json.Number(n.Value)
	}
	panic(fmt.Sprintf("unknown number %T", n))
}

func jsonInt(v int64) json.Number { return json.Number(strconv.FormatInt(v, 10)) }

// formatJSONFloat formats a finite float like serde_json (zmij): the
// shortest representation, in fixed notation when the decimal exponent is in
// -5..=15 (always with a fraction), in scientific notation otherwise
// ("1e+16", "1.5e-7").
func formatJSONFloat(f float64) string {
	if f == 0 {
		if math.Signbit(f) {
			return "-0.0"
		}
		return "0.0"
	}
	mantissa, exponent, _ := strings.Cut(strconv.FormatFloat(math.Abs(f), 'e', -1, 64), "e")
	digits := strings.Replace(mantissa, ".", "", 1)
	exp, _ := strconv.Atoi(exponent)
	n := len(digits)

	var b strings.Builder
	if f < 0 {
		b.WriteByte('-')
	}
	switch {
	case exp < -5 || exp > 15:
		b.WriteByte(digits[0])
		if n > 1 {
			b.WriteByte('.')
			b.WriteString(digits[1:])
		}
		b.WriteByte('e')
		if exp >= 0 {
			b.WriteByte('+')
		}
		b.WriteString(strconv.Itoa(exp))
	case n-1 <= exp:
		b.WriteString(digits)
		b.WriteString(strings.Repeat("0", exp-(n-1)))
		b.WriteString(".0")
	case exp >= 0:
		b.WriteString(digits[:exp+1])
		b.WriteByte('.')
		b.WriteString(digits[exp+1:])
	default:
		b.WriteString("0.")
		b.WriteString(strings.Repeat("0", -exp-1))
		b.WriteString(digits)
	}
	return b.String()
}

// EncodeJSON serializes a JSON value like serde_json::to_string: compact,
// object keys sorted, numbers written as their literal.
func EncodeJSON(v any) []byte {
	var b bytes.Buffer
	encodeJSON(&b, v)
	return b.Bytes()
}

func encodeJSON(b *bytes.Buffer, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case json.Number:
		b.WriteString(string(v))
	case string:
		b.WriteString(jsonQuote(v))
	case []any:
		b.WriteByte('[')
		for i, e := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			encodeJSON(b, e)
		}
		b.WriteByte(']')
	case map[string]any:
		b.WriteByte('{')
		for i, k := range slices.Sorted(maps.Keys(v)) {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(jsonQuote(k))
			b.WriteByte(':')
			encodeJSON(b, v[k])
		}
		b.WriteByte('}')
	default:
		panic(fmt.Sprintf("unexpected JSON value %T", v))
	}
}

// DecodeJSON parses a JSON document like serde_json::from_str::<Value> with
// arbitrary_precision: number literals are kept, with the exponent written
// as e+N or e-N.
func DecodeJSON(data []byte) (any, error) {
	v, err := parseJSON(data)
	if err != nil {
		return nil, err
	}
	return normalizeJSONNumbers(v), nil
}

func normalizeJSONNumbers(v any) any {
	switch v := v.(type) {
	case json.Number:
		mantissa, exp, ok := strings.Cut(strings.ToLower(string(v)), "e")
		if !ok {
			return v
		}
		if exp[0] != '+' && exp[0] != '-' {
			exp = "+" + exp
		}
		return json.Number(mantissa + "e" + exp)
	case []any:
		for i, e := range v {
			v[i] = normalizeJSONNumbers(e)
		}
	case map[string]any:
		for k, e := range v {
			v[k] = normalizeJSONNumbers(e)
		}
	}
	return v
}

// IsDeserializable reports whether the JSON value v has the schema of a
// serialized HurlResult (HurlResult::is_deserializable). As with serde,
// unknown fields are ignored, optional fields can be missing or null, and a
// struct can also be given as an array of its fields in declaration order.
func IsDeserializable(v any) bool { return isStructDeserializable(v, hurlResultSchema) }

type jsonType int

const (
	jsonString jsonType = iota
	jsonBool
	jsonU64
	jsonU32
	jsonAny
	jsonList
	jsonStruct
)

type jsonField struct {
	name     string
	typ      jsonType
	optional bool
	// fields are the fields of a struct, or of the elements of a list.
	fields []jsonField
}

func jsonRequired(name string, typ jsonType) jsonField { return jsonField{name: name, typ: typ} }

func jsonOptional(name string, typ jsonType) jsonField {
	return jsonField{name: name, typ: typ, optional: true}
}

func jsonListOf(name string, fields []jsonField) jsonField {
	return jsonField{name: name, typ: jsonList, fields: fields}
}

func jsonStructOf(name string, fields []jsonField) jsonField {
	return jsonField{name: name, typ: jsonStruct, fields: fields}
}

var (
	hurlResultSchema = []jsonField{
		jsonRequired("filename", jsonString),
		jsonListOf("entries", entryResultSchema),
		jsonRequired("success", jsonBool),
		jsonRequired("time", jsonU64),
		jsonListOf("cookies", cookieSchema),
	}
	entryResultSchema = []jsonField{
		jsonRequired("index", jsonU64),
		jsonRequired("line", jsonU64),
		jsonListOf("calls", callSchema),
		jsonListOf("captures", []jsonField{jsonRequired("name", jsonString), jsonRequired("value", jsonAny)}),
		jsonListOf("asserts", []jsonField{jsonRequired("success", jsonBool), jsonOptional("message", jsonString), jsonRequired("line", jsonU64)}),
		jsonRequired("time", jsonU64),
		jsonRequired("curl_cmd", jsonString),
	}
	cookieSchema = []jsonField{
		jsonRequired("domain", jsonString),
		jsonRequired("include_subdomain", jsonBool),
		jsonRequired("path", jsonString),
		jsonRequired("https", jsonBool),
		jsonRequired("expires", jsonU64),
		jsonRequired("name", jsonString),
		jsonRequired("value", jsonString),
	}
	callSchema = []jsonField{
		jsonStructOf("request", requestSchema),
		jsonStructOf("response", responseSchema),
		jsonStructOf("timings", timingsSchema),
	}
	nameValueSchema = []jsonField{jsonRequired("name", jsonString), jsonRequired("value", jsonString)}
	requestSchema   = []jsonField{
		jsonRequired("method", jsonString),
		jsonRequired("url", jsonString),
		jsonListOf("headers", nameValueSchema),
		jsonListOf("cookies", nameValueSchema),
		jsonListOf("query_string", nameValueSchema),
	}
	responseSchema = []jsonField{
		jsonRequired("http_version", jsonString),
		jsonRequired("status", jsonU32),
		jsonListOf("headers", nameValueSchema),
		jsonListOf("cookies", responseCookieSchema),
		{name: "certificate", typ: jsonStruct, optional: true, fields: certificateSchema},
		jsonOptional("body", jsonString),
	}
	timingsSchema = []jsonField{
		jsonRequired("begin_call", jsonString),
		jsonRequired("end_call", jsonString),
		jsonRequired("name_lookup", jsonU64),
		jsonRequired("connect", jsonU64),
		jsonRequired("app_connect", jsonU64),
		jsonRequired("pre_transfer", jsonU64),
		jsonRequired("start_transfer", jsonU64),
		jsonRequired("total", jsonU64),
	}
	responseCookieSchema = []jsonField{
		jsonRequired("name", jsonString),
		jsonRequired("value", jsonString),
		jsonOptional("expires", jsonString),
		jsonOptional("max_age", jsonString),
		jsonOptional("domain", jsonString),
		jsonOptional("path", jsonString),
		jsonOptional("secure", jsonBool),
		jsonOptional("httponly", jsonBool),
		jsonOptional("same_site", jsonString),
	}
	certificateSchema = []jsonField{
		jsonOptional("subject", jsonString),
		jsonOptional("issuer", jsonString),
		jsonOptional("start_date", jsonString),
		jsonOptional("expire_date", jsonString),
		jsonOptional("serial_number", jsonString),
		jsonOptional("subject_alt_name", jsonString),
		jsonOptional("value", jsonString),
	}
)

func isStructDeserializable(v any, fields []jsonField) bool {
	switch v := v.(type) {
	case map[string]any:
		for _, f := range fields {
			e, ok := v[f.name]
			if !ok {
				if f.optional {
					continue
				}
				return false
			}
			if !isFieldDeserializable(e, f) {
				return false
			}
		}
		return true
	case []any:
		if len(v) != len(fields) {
			return false
		}
		for i, f := range fields {
			if !isFieldDeserializable(v[i], f) {
				return false
			}
		}
		return true
	}
	return false
}

func isFieldDeserializable(v any, f jsonField) bool {
	if f.optional && v == nil {
		return true
	}
	switch f.typ {
	case jsonString:
		_, ok := v.(string)
		return ok
	case jsonBool:
		_, ok := v.(bool)
		return ok
	case jsonU64:
		return isUnsigned(v, math.MaxUint64)
	case jsonU32:
		return isUnsigned(v, math.MaxUint32)
	case jsonAny:
		return true
	case jsonList:
		values, ok := v.([]any)
		if !ok {
			return false
		}
		for _, e := range values {
			if !isStructDeserializable(e, f.fields) {
				return false
			}
		}
		return true
	case jsonStruct:
		return isStructDeserializable(v, f.fields)
	}
	return false
}

// isUnsigned reports whether v is an integer literal in 0..=max, the
// numbers accepted by serde for unsigned integers.
func isUnsigned(v any, limit uint64) bool {
	n, ok := v.(json.Number)
	if !ok {
		return false
	}
	if u, err := strconv.ParseUint(string(n), 10, 64); err == nil {
		return u <= limit
	}
	i, err := strconv.ParseInt(string(n), 10, 64)
	return err == nil && i >= 0 && uint64(i) <= limit
}
