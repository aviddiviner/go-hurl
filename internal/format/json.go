package format

import (
	"encoding/base64"
	"strconv"

	"github.com/aviddiviner/go-hurl/ast"
)

// JSON returns the Hurl file exported as JSON.
func JSON(file *ast.HurlFile) string {
	return jsonFormat(hurlFileJSON(file))
}

func hurlFileJSON(f *ast.HurlFile) jvalue {
	entries := jlist{}
	for i := range f.Entries {
		entries = append(entries, entryJSON(&f.Entries[i]))
	}
	return jobject{{"entries", entries}}
}

func entryJSON(e *ast.Entry) jvalue {
	attrs := jobject{{"request", requestJSON(&e.Request)}}
	if e.Response != nil {
		attrs = append(attrs, jattr{"response", responseJSON(e.Response)})
	}
	return attrs
}

func mapList[T any](items []T, f func(*T) jvalue) jlist {
	l := make(jlist, len(items))
	for i := range items {
		l[i] = f(&items[i])
	}
	return l
}

func requestJSON(r *ast.Request) jvalue {
	attrs := jobject{
		{"method", jstring(r.Method.String())},
		{"url", jstring(r.URL.String())},
	}
	attrs = addHeaders(attrs, r.Headers)
	if params := r.QueryStringParams(); len(params) > 0 {
		attrs = append(attrs, jattr{"query_string_params", mapList(params, keyValueJSON)})
	}
	if params := r.FormParams(); len(params) > 0 {
		attrs = append(attrs, jattr{"form_params", mapList(params, keyValueJSON)})
	}
	if params := r.MultipartFormData(); len(params) > 0 {
		attrs = append(attrs, jattr{"multipart_form_data", mapList(params, multipartParamJSON)})
	}
	if cookies := r.Cookies(); len(cookies) > 0 {
		attrs = append(attrs, jattr{"cookies", mapList(cookies, cookieJSON)})
	}
	if options := r.Options(); len(options) > 0 {
		attrs = append(attrs, jattr{"options", mapList(options, entryOptionJSON)})
	}
	if r.Body != nil {
		attrs = append(attrs, jattr{"body", bytesJSON(r.Body.Value)})
	}

	// Request comments (can be used to check custom commands).
	var comments jlist
	for _, lt := range r.LineTerminators {
		if lt.Comment != nil {
			comments = append(comments, jstring(lt.Comment.Value))
		}
	}
	if len(comments) > 0 {
		attrs = append(attrs, jattr{"comments", comments})
	}
	return attrs
}

func responseJSON(r *ast.Response) jvalue {
	attrs := jobject{}
	if r.Version.Value != ast.VersionAny {
		attrs = append(attrs, jattr{"version", jstring(r.Version.Value.String())})
	}
	if s, ok := r.Status.Value.(ast.StatusSpecific); ok {
		attrs = append(attrs, jattr{"status", jnumber(strconv.FormatUint(s.Value, 10))})
	}
	attrs = addHeaders(attrs, r.Headers)
	if captures := r.Captures(); len(captures) > 0 {
		attrs = append(attrs, jattr{"captures", mapList(captures, captureJSON)})
	}
	if asserts := r.Asserts(); len(asserts) > 0 {
		attrs = append(attrs, jattr{"asserts", mapList(asserts, assertJSON)})
	}
	if r.Body != nil {
		attrs = append(attrs, jattr{"body", bytesJSON(r.Body.Value)})
	}
	return attrs
}

func addHeaders(attrs jobject, headers []ast.KeyValue) jobject {
	if len(headers) > 0 {
		attrs = append(attrs, jattr{"headers", mapList(headers, keyValueJSON)})
	}
	return attrs
}

func bytesJSON(b ast.Bytes) jvalue {
	switch v := b.(type) {
	case ast.BytesBase64:
		return encodedJSON(v.Value.Value)
	case ast.BytesHex:
		return encodedJSON(v.Value.Value)
	case ast.BytesFile:
		return fileJSON(&v.Value)
	case ast.BytesJSON:
		return jobject{{"type", jstring("json")}, {"value", jsonValueJSON(v.Value)}}
	case ast.BytesXML:
		return jobject{{"type", jstring("xml")}, {"value", jstring(v.Value)}}
	case ast.BytesOnelineString:
		return jobject{{"type", jstring("text")}, {"value", jstring(v.Value.String())}}
	case ast.BytesMultilineString:
		lang := v.Value.Lang()
		if lang == "" {
			lang = "text"
		}
		return jobject{{"type", jstring(lang)}, {"value", jstring(v.Value.String())}}
	}
	return jnull{}
}

func encodedJSON(value []byte) jvalue {
	return jobject{
		{"encoding", jstring("base64")},
		{"value", jstring(base64.StdEncoding.EncodeToString(value))},
	}
}

func fileJSON(f *ast.File) jvalue {
	return jobject{{"type", jstring("file")}, {"filename", jstring(f.Filename.String())}}
}

func keyValueJSON(kv *ast.KeyValue) jvalue {
	return jobject{{"name", jstring(kv.Key.String())}, {"value", jstring(kv.Value.String())}}
}

func multipartParamJSON(p *ast.MultipartParam) jvalue {
	switch p := (*p).(type) {
	case ast.MultipartParamParam:
		return keyValueJSON(&p.Value)
	case ast.MultipartParamFilenameParam:
		attrs := jobject{
			{"name", jstring(p.Value.Key.String())},
			{"filename", jstring(p.Value.Value.Filename.String())},
		}
		if ct := p.Value.Value.ContentType; ct != nil {
			attrs = append(attrs, jattr{"content_type", jstring(ct.String())})
		}
		return attrs
	}
	return jnull{}
}

func cookieJSON(c *ast.Cookie) jvalue {
	return jobject{{"name", jstring(c.Name.String())}, {"value", jstring(c.Value.String())}}
}

func entryOptionJSON(o *ast.EntryOption) jvalue {
	var value jvalue
	switch k := o.Kind.(type) {
	case ast.OptionAWSSigV4:
		value = jstring(k.Value.String())
	case ast.OptionCACertificate:
		value = jstring(k.Value.String())
	case ast.OptionClientCert:
		value = jstring(k.Value.String())
	case ast.OptionClientKey:
		value = jstring(k.Value.String())
	case ast.OptionCompressed:
		value = boolOptionJSON(k.Value)
	case ast.OptionConnectTo:
		value = jstring(k.Value.String())
	case ast.OptionConnectTimeout:
		value = durationOptionJSON(k.Value)
	case ast.OptionDelay:
		value = durationOptionJSON(k.Value)
	case ast.OptionDigest:
		value = boolOptionJSON(k.Value)
	case ast.OptionFailWithBody:
		value = boolOptionJSON(k.Value)
	case ast.OptionFollowLocation:
		value = boolOptionJSON(k.Value)
	case ast.OptionFollowLocationTrusted:
		value = boolOptionJSON(k.Value)
	case ast.OptionHeader:
		value = jstring(k.Value.String())
	case ast.OptionHTTP10:
		value = boolOptionJSON(k.Value)
	case ast.OptionHTTP11:
		value = boolOptionJSON(k.Value)
	case ast.OptionHTTP2:
		value = boolOptionJSON(k.Value)
	case ast.OptionHTTP2PriorKnowledge:
		value = boolOptionJSON(k.Value)
	case ast.OptionHTTP3:
		value = boolOptionJSON(k.Value)
	case ast.OptionInsecure:
		value = boolOptionJSON(k.Value)
	case ast.OptionIPv4:
		value = boolOptionJSON(k.Value)
	case ast.OptionIPv6:
		value = boolOptionJSON(k.Value)
	case ast.OptionLimitRate:
		value = naturalOptionJSON(k.Value)
	case ast.OptionMaxRedirect:
		value = countOptionJSON(k.Value)
	case ast.OptionMaxTime:
		value = durationOptionJSON(k.Value)
	case ast.OptionNegotiate:
		value = boolOptionJSON(k.Value)
	case ast.OptionNetRc:
		value = boolOptionJSON(k.Value)
	case ast.OptionNetRcFile:
		value = jstring(k.Value.String())
	case ast.OptionNetRcOptional:
		value = boolOptionJSON(k.Value)
	case ast.OptionNoHeader:
		value = jstring(k.Value.String())
	case ast.OptionNoJSONPathCoercion:
		value = boolOptionJSON(k.Value)
	case ast.OptionNtlm:
		value = boolOptionJSON(k.Value)
	case ast.OptionOutput:
		value = jstring(k.Value.String())
	case ast.OptionPathAsIs:
		value = boolOptionJSON(k.Value)
	case ast.OptionPinnedPublicKey:
		value = jstring(k.Value.String())
	case ast.OptionProxy:
		value = jstring(k.Value.String())
	case ast.OptionRepeat:
		value = countOptionJSON(k.Value)
	case ast.OptionResolve:
		value = jstring(k.Value.String())
	case ast.OptionRetry:
		value = countOptionJSON(k.Value)
	case ast.OptionRetryInterval:
		value = durationOptionJSON(k.Value)
	case ast.OptionSkip:
		value = boolOptionJSON(k.Value)
	case ast.OptionTLSMax:
		value = jstring(k.Value.String())
	case ast.OptionTLSMin:
		value = jstring(k.Value.String())
	case ast.OptionUnixSocket:
		value = jstring(k.Value.String())
	case ast.OptionUser:
		value = jstring(k.Value.String())
	case ast.OptionVariable:
		value = jstring(k.Value.Name + "=" + k.Value.Value.ToSource())
	case ast.OptionVerbose:
		value = boolOptionJSON(k.Value)
	case ast.OptionVerbosity:
		value = jstring(k.Value.String())
	case ast.OptionVeryVerbose:
		value = boolOptionJSON(k.Value)
	}

	// If the value is an object such as {"value": 10, "unit": "s"}, the
	// option is flattened to {"value": 10, "unit": "s", "name": "delay"}.
	name := jattr{"name", jstring(o.Kind.Identifier())}
	if obj, ok := value.(jobject); ok {
		return append(obj, name)
	}
	return jobject{name, {"value", value}}
}

func boolOptionJSON(o ast.BooleanOption) jvalue {
	switch o := o.(type) {
	case ast.BooleanLiteral:
		return jboolean(o.Value)
	case ast.BooleanPlaceholder:
		return placeholderJSON(&o.Value)
	}
	return jnull{}
}

func countOptionJSON(o ast.CountOption) jvalue {
	switch o := o.(type) {
	case ast.CountLiteral:
		return jnumber(o.Value.String())
	case ast.CountPlaceholder:
		return placeholderJSON(&o.Value)
	}
	return jnull{}
}

func durationOptionJSON(o ast.DurationOption) jvalue {
	switch o := o.(type) {
	case ast.DurationLiteral:
		if o.Value.Unit == nil {
			return jnumber(o.Value.Value.String())
		}
		return jobject{
			{"value", jnumber(o.Value.Value.String())},
			{"unit", jstring(o.Value.Unit.String())},
		}
	case ast.DurationPlaceholder:
		return placeholderJSON(&o.Value)
	}
	return jnull{}
}

func naturalOptionJSON(o ast.NaturalOption) jvalue {
	switch o := o.(type) {
	case ast.NaturalLiteral:
		return jnumber(o.Value.String())
	case ast.NaturalPlaceholder:
		return placeholderJSON(&o.Value)
	}
	return jnull{}
}

func placeholderJSON(p *ast.Placeholder) jvalue {
	return jstring("{{" + p.String() + "}}")
}

func filtersJSON(filters []ast.SpacedFilter) jlist {
	l := make(jlist, len(filters))
	for i, f := range filters {
		l[i] = filterJSON(f.Filter.Value)
	}
	return l
}

func captureJSON(c *ast.Capture) jvalue {
	attrs := jobject{
		{"name", jstring(c.Name.String())},
		{"query", queryJSON(&c.Query)},
	}
	if len(c.Filters) > 0 {
		attrs = append(attrs, jattr{"filters", filtersJSON(c.Filters)})
	}
	if c.Redacted {
		attrs = append(attrs, jattr{"redact", jboolean(true)})
	}
	return attrs
}

func assertJSON(a *ast.Assert) jvalue {
	attrs := jobject{{"query", queryJSON(&a.Query)}}
	if len(a.Filters) > 0 {
		attrs = append(attrs, jattr{"filters", filtersJSON(a.Filters)})
	}
	return append(attrs, jattr{"predicate", predicateJSON(&a.Predicate)})
}

func queryJSON(q *ast.Query) jvalue {
	attrs := jobject{{"type", jstring(q.Value.Identifier())}}
	switch v := q.Value.(type) {
	case ast.QueryJSONPath:
		attrs = append(attrs, jattr{"expr", jstring(v.Expr.String())})
	case ast.QueryHeader:
		attrs = append(attrs, jattr{"name", jstring(v.Name.String())})
	case ast.QueryCookie:
		attrs = append(attrs, jattr{"expr", jstring(v.Expr.String())})
	case ast.QueryXPath:
		attrs = append(attrs, jattr{"expr", jstring(v.Expr.String())})
	case ast.QueryRegex:
		attrs = append(attrs, jattr{"expr", regexValueJSON(v.Value)})
	case ast.QueryVariable:
		attrs = append(attrs, jattr{"name", jstring(v.Name.String())})
	case ast.QueryCertificate:
		attrs = append(attrs, jattr{"expr", jstring(v.AttributeName.Identifier())})
	}
	return attrs
}

func regexValueJSON(v ast.RegexValue) jvalue {
	switch v := v.(type) {
	case ast.RegexTemplate:
		return jstring(v.Value.String())
	case ast.RegexRegex:
		return regexJSON(&v.Value)
	}
	return jnull{}
}

func regexJSON(r *ast.Regex) jvalue {
	return jobject{{"type", jstring("regex")}, {"value", jstring(r.String())}}
}

func predicateJSON(p *ast.Predicate) jvalue {
	attrs := jobject{}
	if p.Not {
		attrs = append(attrs, jattr{"not", jboolean(true)})
	}
	attrs = append(attrs, jattr{"type", jstring(p.PredicateFunc.Value.Identifier())})
	if value := predicateFuncOperand(p.PredicateFunc.Value); value != nil {
		v, encoding := predicateValueJSON(value)
		attrs = append(attrs, jattr{"value", v})
		if encoding != "" {
			attrs = append(attrs, jattr{"encoding", jstring(encoding)})
		}
	}
	return attrs
}

// predicateFuncOperand returns the value of a predicate function, or nil for
// predicates without a value (exists, isString...).
func predicateFuncOperand(f ast.PredicateFuncValue) ast.PredicateValue {
	switch f := f.(type) {
	case ast.PredicateFuncEqual:
		return f.Value
	case ast.PredicateFuncNotEqual:
		return f.Value
	case ast.PredicateFuncGreaterThan:
		return f.Value
	case ast.PredicateFuncGreaterThanOrEqual:
		return f.Value
	case ast.PredicateFuncLessThan:
		return f.Value
	case ast.PredicateFuncLessThanOrEqual:
		return f.Value
	case ast.PredicateFuncStartWith:
		return f.Value
	case ast.PredicateFuncEndWith:
		return f.Value
	case ast.PredicateFuncContain:
		return f.Value
	case ast.PredicateFuncInclude:
		return f.Value
	case ast.PredicateFuncMatch:
		return f.Value
	}
	return nil
}

func predicateValueJSON(v ast.PredicateValue) (jvalue, string) {
	switch v := v.(type) {
	case ast.PredicateString:
		return jstring(v.Value.String()), ""
	case ast.PredicateMultilineString:
		return jstring(v.Value.Value().String()), ""
	case ast.PredicateBool:
		return jboolean(v.Value), ""
	case ast.PredicateNull:
		return jnull{}, ""
	case ast.PredicateNumber:
		return jnumber(v.Value.String()), ""
	case ast.PredicateFile:
		return fileJSON(&v.Value), ""
	case ast.PredicateHex:
		return jstring(base64.StdEncoding.EncodeToString(v.Value.Value)), "base64"
	case ast.PredicateBase64:
		return jstring(base64.StdEncoding.EncodeToString(v.Value.Value)), "base64"
	case ast.PredicatePlaceholder:
		return jstring(v.Value.String()), ""
	case ast.PredicateRegex:
		return jstring(v.Value.String()), "regex"
	}
	return jnull{}, ""
}

func jsonValueJSON(v ast.JSONValue) jvalue {
	switch v := v.(type) {
	case ast.JSONNull:
		return jnull{}
	case ast.JSONNumber:
		return jnumber(v.Value)
	case ast.JSONString:
		return jstring(v.Value.String())
	case ast.JSONBoolean:
		return jboolean(v.Value)
	case ast.JSONList:
		l := make(jlist, len(v.Elements))
		for i, e := range v.Elements {
			l[i] = jsonValueJSON(e.Value)
		}
		return l
	case ast.JSONObject:
		o := make(jobject, len(v.Elements))
		for i, e := range v.Elements {
			o[i] = jattr{e.Name.String(), jsonValueJSON(e.Value)}
		}
		return o
	case ast.JSONPlaceholder:
		return jstring("{{" + v.Value.String() + "}}")
	}
	return jnull{}
}

func filterJSON(f ast.FilterValue) jvalue {
	attrs := jobject{{"type", jstring(f.Identifier())}}
	switch f := f.(type) {
	case ast.FilterDecode:
		attrs = append(attrs, jattr{"encoding", jstring(f.Encoding.String())})
	case ast.FilterFormat:
		attrs = append(attrs, jattr{"fmt", jstring(f.Fmt.String())})
	case ast.FilterDateFormat:
		attrs = append(attrs, jattr{"fmt", jstring(f.Fmt.String())})
	case ast.FilterJSONPath:
		attrs = append(attrs, jattr{"expr", jstring(f.Expr.String())})
	case ast.FilterNth:
		attrs = append(attrs, jattr{"n", jnumber(f.N.String())})
	case ast.FilterRegex:
		attrs = append(attrs, jattr{"expr", regexValueJSON(f.Value)})
	case ast.FilterReplace:
		attrs = append(attrs,
			jattr{"old_value", jstring(f.OldValue.String())},
			jattr{"new_value", jstring(f.NewValue.String())})
	case ast.FilterReplaceRegex:
		attrs = append(attrs,
			jattr{"pattern", regexValueJSON(f.Pattern)},
			jattr{"new_value", jstring(f.NewValue.String())})
	case ast.FilterSplit:
		attrs = append(attrs, jattr{"sep", jstring(f.Sep.String())})
	case ast.FilterToDate:
		attrs = append(attrs, jattr{"fmt", jstring(f.Fmt.String())})
	case ast.FilterURLQueryParam:
		attrs = append(attrs, jattr{"param", jstring(f.Param.String())})
	case ast.FilterXPath:
		attrs = append(attrs, jattr{"expr", jstring(f.Expr.String())})
	}
	return attrs
}
