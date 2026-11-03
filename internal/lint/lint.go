// Package lint rewrites a Hurl file in its canonical format, ported from
// hurlfmt's linter (rewrite.rs).
package lint

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
)

// HurlFile returns the linted text of file.
func HurlFile(file *ast.HurlFile) string {
	var b strings.Builder
	for i := range file.Entries {
		entry(&b, &file.Entries[i])
	}
	lineTerminators(&b, file.LineTerminators)
	return b.String()
}

func entry(b *strings.Builder, e *ast.Entry) {
	request(b, &e.Request)
	if e.Response != nil {
		response(b, e.Response)
	}
}

func lineTerminators(b *strings.Builder, lts []ast.LineTerminator) {
	for i := range lts {
		lineTerminator(b, &lts[i], false)
	}
}

func lineTerminator(b *strings.Builder, lt *ast.LineTerminator, trailing bool) {
	if lt.Comment != nil {
		if trailing {
			// Keep the leading whitespace of a trailing comment, to keep the
			// user's alignment.
			b.WriteString(lt.Space0.Value)
		}
		b.WriteString(comment(lt.Comment))
	}
	// We always terminate a file by a newline.
	if lt.Newline.Value == "" {
		b.WriteByte('\n')
	} else {
		b.WriteString(lt.Newline.Value)
	}
}

func comment(c *ast.Comment) string {
	return "#" + strings.TrimRightFunc(c.Value, unicode.IsSpace)
}

func request(b *strings.Builder, r *ast.Request) {
	lineTerminators(b, r.LineTerminators)
	b.WriteString(r.Method.ToSource())
	b.WriteByte(' ')
	b.WriteString(r.URL.ToSource())
	lineTerminator(b, &r.LineTerminator0, true)
	for i := range r.Headers {
		keyValue(b, &r.Headers[i])
	}

	// Sections are reordered.
	for _, match := range []func(ast.SectionValue) bool{
		is[ast.SectionOptions],
		is[ast.SectionQueryParams],
		func(v ast.SectionValue) bool {
			auth, ok := v.(ast.SectionBasicAuth)
			return ok && auth.Value != nil
		},
		is[ast.SectionFormParams],
		is[ast.SectionMultipartFormData],
		is[ast.SectionCookies],
	} {
		if s := findSection(r.Sections, match); s != nil {
			section(b, s)
		}
	}
	if r.Body != nil {
		body(b, r.Body)
	}
}

func response(b *strings.Builder, r *ast.Response) {
	lineTerminators(b, r.LineTerminators)
	b.WriteString(r.Version.Value.ToSource())
	b.WriteByte(' ')
	b.WriteString(r.Status.Value.ToSource())
	lineTerminator(b, &r.LineTerminator0, true)
	for i := range r.Headers {
		keyValue(b, &r.Headers[i])
	}
	for _, match := range []func(ast.SectionValue) bool{
		is[ast.SectionCaptures],
		is[ast.SectionAsserts],
	} {
		if s := findSection(r.Sections, match); s != nil {
			section(b, s)
		}
	}
	if r.Body != nil {
		body(b, r.Body)
	}
}

func is[T ast.SectionValue](v ast.SectionValue) bool {
	_, ok := v.(T)
	return ok
}

func findSection(sections []ast.Section, match func(ast.SectionValue) bool) *ast.Section {
	for i := range sections {
		if match(sections[i].Value) {
			return &sections[i]
		}
	}
	return nil
}

func section(b *strings.Builder, s *ast.Section) {
	lineTerminators(b, s.LineTerminators)
	b.WriteString("[" + s.Identifier() + "]")
	lineTerminator(b, &s.LineTerminator0, true)
	switch v := s.Value.(type) {
	case ast.SectionQueryParams:
		for i := range v.Value {
			keyValue(b, &v.Value[i])
		}
	case ast.SectionBasicAuth:
		if v.Value != nil {
			keyValue(b, v.Value)
		}
	case ast.SectionFormParams:
		for i := range v.Value {
			keyValue(b, &v.Value[i])
		}
	case ast.SectionMultipartFormData:
		for _, p := range v.Value {
			switch p := p.(type) {
			case ast.MultipartParamParam:
				keyValue(b, &p.Value)
			case ast.MultipartParamFilenameParam:
				filenameParam(b, &p.Value)
			}
		}
	case ast.SectionCookies:
		for i := range v.Value {
			cookie(b, &v.Value[i])
		}
	case ast.SectionCaptures:
		for i := range v.Value {
			capture(b, &v.Value[i])
		}
	case ast.SectionAsserts:
		for i := range v.Value {
			assert(b, &v.Value[i])
		}
	case ast.SectionOptions:
		for i := range v.Value {
			entryOption(b, &v.Value[i])
		}
	}
}

func keyValue(b *strings.Builder, kv *ast.KeyValue) {
	lineTerminators(b, kv.LineTerminators)
	b.WriteString(kv.Key.ToSource())
	b.WriteByte(':')
	if !kv.Value.IsEmpty() {
		b.WriteByte(' ')
		b.WriteString(kv.Value.ToSource())
	}
	lineTerminator(b, &kv.LineTerminator0, true)
}

func cookie(b *strings.Builder, c *ast.Cookie) {
	lineTerminators(b, c.LineTerminators)
	b.WriteString(c.Name.ToSource() + ": " + c.Value.ToSource())
	lineTerminator(b, &c.LineTerminator0, true)
}

func filenameParam(b *strings.Builder, p *ast.FilenameParam) {
	lineTerminators(b, p.LineTerminators)
	b.WriteString(p.Key.ToSource() + ": file," + p.Value.Filename.ToSource() + ";")
	if ct := p.Value.ContentType; ct != nil {
		b.WriteString(" " + ct.ToSource())
	}
	lineTerminator(b, &p.LineTerminator0, true)
}

func body(b *strings.Builder, body *ast.Body) {
	lineTerminators(b, body.LineTerminators)
	b.WriteString(bytes(body.Value))
	lineTerminator(b, &body.LineTerminator0, true)
}

func bytes(v ast.Bytes) string {
	switch v := v.(type) {
	case ast.BytesJSON:
		return v.Value.ToSource()
	case ast.BytesXML:
		return v.Value
	case ast.BytesMultilineString:
		return v.Value.ToSource()
	case ast.BytesOnelineString:
		return v.Value.ToSource()
	case ast.BytesBase64:
		return base64(&v.Value)
	case ast.BytesFile:
		return file(&v.Value)
	case ast.BytesHex:
		return hex(&v.Value)
	}
	return ""
}

func base64(v *ast.Base64) string { return "base64," + v.Source + ";" }
func hex(v *ast.Hex) string       { return "hex," + v.Source + ";" }
func file(f *ast.File) string     { return "file," + f.Filename.ToSource() + ";" }

func filters(b *strings.Builder, fs []ast.SpacedFilter) {
	if len(fs) == 0 {
		return
	}
	linted := make([]string, len(fs))
	for i, f := range fs {
		linted[i] = filter(f.Filter.Value)
	}
	b.WriteString(" " + strings.Join(linted, " "))
}

func capture(b *strings.Builder, c *ast.Capture) {
	lineTerminators(b, c.LineTerminators)
	b.WriteString(c.Name.ToSource() + ": " + query(&c.Query))
	filters(b, c.Filters)
	if c.Redacted {
		b.WriteString(" redact")
	}
	lineTerminator(b, &c.LineTerminator0, true)
}

func assert(b *strings.Builder, a *ast.Assert) {
	lineTerminators(b, a.LineTerminators)
	b.WriteString(query(&a.Query))
	filters(b, a.Filters)
	b.WriteString(" " + predicate(&a.Predicate))
	lineTerminator(b, &a.LineTerminator0, true)
}

func query(q *ast.Query) string {
	s := q.Value.Identifier()
	switch v := q.Value.(type) {
	case ast.QueryHeader:
		s += " " + v.Name.ToSource()
	case ast.QueryCookie:
		s += " " + v.Expr.ToSource()
	case ast.QueryXPath:
		s += " " + v.Expr.ToSource()
	case ast.QueryJSONPath:
		s += " " + v.Expr.ToSource()
	case ast.QueryRegex:
		s += " " + regexValue(v.Value)
	case ast.QueryVariable:
		s += " " + v.Name.ToSource()
	case ast.QueryCertificate:
		s += " " + v.AttributeName.ToSource()
	}
	return s
}

func regexValue(v ast.RegexValue) string {
	switch v := v.(type) {
	case ast.RegexTemplate:
		return v.Value.ToSource()
	case ast.RegexRegex:
		return v.Value.ToSource()
	}
	return ""
}

func filter(f ast.FilterValue) string {
	s := f.Identifier()
	switch f := f.(type) {
	case ast.FilterCharsetDecode:
		s += " " + f.Encoding.ToSource()
	case ast.FilterDecode:
		s += " " + f.Encoding.ToSource()
	case ast.FilterFormat:
		s += " " + f.Fmt.ToSource()
	case ast.FilterDateFormat:
		s += " " + f.Fmt.ToSource()
	case ast.FilterJSONPath:
		s += " " + f.Expr.ToSource()
	case ast.FilterNth:
		s += " " + integerValue(f.N)
	case ast.FilterRegex:
		s += " " + regexValue(f.Value)
	case ast.FilterReplace:
		s += " " + f.OldValue.ToSource() + " " + f.NewValue.ToSource()
	case ast.FilterSplit:
		s += " " + f.Sep.ToSource()
	case ast.FilterReplaceRegex:
		s += " " + regexValue(f.Pattern) + " " + f.NewValue.ToSource()
	case ast.FilterToDate:
		s += " " + f.Fmt.ToSource()
	case ast.FilterURLQueryParam:
		s += " " + f.Param.ToSource()
	case ast.FilterXPath:
		s += " " + f.Expr.ToSource()
	}
	return s
}

func integerValue(v ast.IntegerValue) string {
	switch v := v.(type) {
	case ast.IntegerLiteral:
		return v.Value.ToSource()
	case ast.IntegerPlaceholder:
		return v.Value.ToSource()
	}
	return ""
}

func predicate(p *ast.Predicate) string {
	s := ""
	if p.Not {
		s = "not "
	}
	f := p.PredicateFunc.Value
	s += f.Identifier()
	if v := predicateFuncOperand(f); v != nil {
		s += " " + predicateValue(v)
	}
	return s
}

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

func predicateValue(v ast.PredicateValue) string {
	switch v := v.(type) {
	case ast.PredicateBase64:
		return base64(&v.Value)
	case ast.PredicateBool:
		return strconv.FormatBool(v.Value)
	case ast.PredicateFile:
		return file(&v.Value)
	case ast.PredicateHex:
		return hex(&v.Value)
	case ast.PredicateMultilineString:
		return v.Value.ToSource()
	case ast.PredicateNull:
		return "null"
	case ast.PredicateNumber:
		return v.Value.ToSource()
	case ast.PredicatePlaceholder:
		return v.Value.ToSource()
	case ast.PredicateRegex:
		return v.Value.ToSource()
	case ast.PredicateString:
		return v.Value.ToSource()
	}
	return ""
}

func entryOption(b *strings.Builder, o *ast.EntryOption) {
	lineTerminators(b, o.LineTerminators)
	b.WriteString(o.Kind.Identifier() + ": " + optionValue(o.Kind))
	lineTerminator(b, &o.LineTerminator0, true)
}

func optionValue(kind ast.OptionKind) string {
	switch k := kind.(type) {
	case ast.OptionAWSSigV4:
		return k.Value.ToSource()
	case ast.OptionCACertificate:
		return k.Value.ToSource()
	case ast.OptionClientCert:
		return k.Value.ToSource()
	case ast.OptionClientKey:
		return k.Value.ToSource()
	case ast.OptionCompressed:
		return boolOption(k.Value)
	case ast.OptionConnectTo:
		return k.Value.ToSource()
	case ast.OptionConnectTimeout:
		return durationOption(k.Value, ast.DurationUnitMilliSecond)
	case ast.OptionDelay:
		return durationOption(k.Value, ast.DurationUnitMilliSecond)
	case ast.OptionDigest:
		return boolOption(k.Value)
	case ast.OptionFailWithBody:
		return boolOption(k.Value)
	case ast.OptionHeader:
		return k.Value.ToSource()
	case ast.OptionHTTP10:
		return boolOption(k.Value)
	case ast.OptionHTTP11:
		return boolOption(k.Value)
	case ast.OptionHTTP2:
		return boolOption(k.Value)
	case ast.OptionHTTP2PriorKnowledge:
		return boolOption(k.Value)
	case ast.OptionHTTP3:
		return boolOption(k.Value)
	case ast.OptionInsecure:
		return boolOption(k.Value)
	case ast.OptionIPv4:
		return boolOption(k.Value)
	case ast.OptionIPv6:
		return boolOption(k.Value)
	case ast.OptionFollowLocation:
		return boolOption(k.Value)
	case ast.OptionFollowLocationTrusted:
		return boolOption(k.Value)
	case ast.OptionLimitRate:
		return naturalOption(k.Value)
	case ast.OptionMaxRedirect:
		return countOption(k.Value)
	case ast.OptionMaxTime:
		return durationOption(k.Value, ast.DurationUnitMilliSecond)
	case ast.OptionNegotiate:
		return boolOption(k.Value)
	case ast.OptionNetRc:
		return boolOption(k.Value)
	case ast.OptionNetRcFile:
		return k.Value.ToSource()
	case ast.OptionNetRcOptional:
		return boolOption(k.Value)
	case ast.OptionNoHeader:
		return k.Value.ToSource()
	case ast.OptionNoJSONPathCoercion:
		return boolOption(k.Value)
	case ast.OptionNtlm:
		return boolOption(k.Value)
	case ast.OptionOutput:
		return k.Value.ToSource()
	case ast.OptionPathAsIs:
		return boolOption(k.Value)
	case ast.OptionPinnedPublicKey:
		return k.Value.ToSource()
	case ast.OptionProxy:
		return k.Value.ToSource()
	case ast.OptionRepeat:
		return countOption(k.Value)
	case ast.OptionResolve:
		return k.Value.ToSource()
	case ast.OptionRetry:
		return countOption(k.Value)
	case ast.OptionRetryInterval:
		return durationOption(k.Value, ast.DurationUnitMilliSecond)
	case ast.OptionSkip:
		return boolOption(k.Value)
	case ast.OptionTLSMax:
		return k.Value.ToSource()
	case ast.OptionTLSMin:
		return k.Value.ToSource()
	case ast.OptionUnixSocket:
		return k.Value.ToSource()
	case ast.OptionUser:
		return k.Value.ToSource()
	case ast.OptionVariable:
		return k.Value.Name + "=" + k.Value.Value.ToSource()
	case ast.OptionVerbose:
		return boolOption(k.Value)
	case ast.OptionVerbosity:
		return k.Value.String()
	case ast.OptionVeryVerbose:
		return boolOption(k.Value)
	}
	return ""
}

func boolOption(o ast.BooleanOption) string {
	switch o := o.(type) {
	case ast.BooleanLiteral:
		return strconv.FormatBool(o.Value)
	case ast.BooleanPlaceholder:
		return o.Value.ToSource()
	}
	return ""
}

func countOption(o ast.CountOption) string {
	switch o := o.(type) {
	case ast.CountLiteral:
		return o.Value.String()
	case ast.CountPlaceholder:
		return o.Value.ToSource()
	}
	return ""
}

func naturalOption(o ast.NaturalOption) string {
	switch o := o.(type) {
	case ast.NaturalLiteral:
		return o.Value.ToSource()
	case ast.NaturalPlaceholder:
		return o.Value.ToSource()
	}
	return ""
}

func durationOption(o ast.DurationOption, defaultUnit ast.DurationUnit) string {
	switch o := o.(type) {
	case ast.DurationLiteral:
		unit := defaultUnit
		if o.Value.Unit != nil {
			unit = *o.Value.Unit
		}
		return o.Value.Value.ToSource() + unit.String()
	case ast.DurationPlaceholder:
		return o.Value.ToSource()
	}
	return ""
}
