package runner

import (
	"crypto/md5"
	"crypto/sha256"
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/xpath"
)

// QueryOptions control query evaluation.
type QueryOptions struct {
	UseJSONPathCoercion bool
}

// DefaultQueryOptions returns the default query options (JSONPath coercion
// enabled).
func DefaultQueryOptions() QueryOptions { return QueryOptions{UseJSONPathCoercion: true} }

// evalQuery evaluates query against a list of HTTP responses (the last one
// being the final response of a redirection chain). A nil Value is no
// result.
func evalQuery(query ast.Query, variables *VariableSet, responses []*httpclient.Response, cache *bodyCache, options QueryOptions) (Value, *RunnerError) {
	last := responses[len(responses)-1]
	switch q := query.Value.(type) {
	case ast.QueryStatus:
		return NewInteger(int64(last.Status)), nil
	case ast.QueryVersion:
		return NewString(strings.TrimPrefix(last.Version.String(), "HTTP/")), nil
	case ast.QueryURL:
		return NewString(last.Url.String()), nil
	case ast.QueryHeader:
		return evalQueryHeader(last, q.Name, variables)
	case ast.QueryCookie:
		return evalQueryCookie(last, q.Expr.Name, q.Expr.Attribute, variables)
	case ast.QueryBody:
		s, err := last.Text()
		if err != nil {
			return nil, NewRunnerError(query.SourceInfo, RunnerErrorHTTP{Value: err}, false)
		}
		return NewString(s), nil
	case ast.QueryXPath:
		return evalQueryXPath(last, cache, q.Expr, variables, query.SourceInfo)
	case ast.QueryJSONPath:
		return evalQueryJSONPath(last, cache, q.Expr, variables, query.SourceInfo, options.UseJSONPathCoercion)
	case ast.QueryRegex:
		return evalQueryRegex(last, q.Value, variables, query.SourceInfo)
	case ast.QueryVariable:
		name, err := evalTemplate(q.Name, variables)
		if err != nil {
			return nil, err
		}
		if v, ok := variables.Get(name); ok {
			return v.Value, nil
		}
		return nil, nil
	case ast.QueryDuration:
		return NewInteger(last.Duration.Milliseconds()), nil
	case ast.QueryBytes:
		b, err := last.UncompressBody()
		if err != nil {
			return nil, NewRunnerError(query.SourceInfo, RunnerErrorHTTP{Value: err}, false)
		}
		return ValueBytes{b}, nil
	case ast.QueryRawBytes:
		return ValueBytes{last.Body}, nil
	case ast.QuerySHA256:
		b, err := last.UncompressBody()
		if err != nil {
			return nil, NewRunnerError(query.SourceInfo, RunnerErrorHTTP{Value: err}, false)
		}
		sum := sha256.Sum256(b)
		return ValueBytes{sum[:]}, nil
	case ast.QueryMD5:
		b, err := last.UncompressBody()
		if err != nil {
			return nil, NewRunnerError(query.SourceInfo, RunnerErrorHTTP{Value: err}, false)
		}
		sum := md5.Sum(b)
		return ValueBytes{sum[:]}, nil
	case ast.QueryCertificate:
		return evalQueryCertificate(last, q.AttributeName), nil
	case ast.QueryIP:
		return NewString(last.IpAddr.String()), nil
	case ast.QueryRedirects:
		return evalRedirects(responses), nil
	}
	panic("unknown query")
}

func evalQueryHeader(response *httpclient.Response, name ast.Template, variables *VariableSet) (Value, *RunnerError) {
	n, err := evalTemplate(name, variables)
	if err != nil {
		return nil, err
	}
	values := response.Headers.Values(n)
	switch len(values) {
	case 0:
		return nil, nil
	case 1:
		return NewString(values[0]), nil
	}
	list := make([]Value, len(values))
	for i, v := range values {
		list[i] = NewString(v)
	}
	return ValueList{list}, nil
}

func evalQueryCookie(response *httpclient.Response, name ast.Template, attribute *ast.CookieAttribute, variables *VariableSet) (Value, *RunnerError) {
	si := name.SourceInfo
	n, err := evalTemplate(name, variables)
	if err != nil {
		return nil, err
	}
	cookie, ok := response.GetCookie(n)
	if !ok {
		return nil, nil
	}
	attributeName := ast.CookieAttributeName{Kind: ast.CookieAttributeValue, Value: "Value"}
	if attribute != nil {
		attributeName = attribute.Name
	}
	return evalCookieAttributeName(attributeName, cookie, si)
}

func evalQueryXPath(response *httpclient.Response, cache *bodyCache, expr ast.Template, variables *VariableSet, si ast.SourceInfo) (Value, *RunnerError) {
	if cache.xml == nil {
		text, err := response.Text()
		if err != nil {
			return nil, NewRunnerError(si, RunnerErrorHTTP{Value: err}, false)
		}
		format := xpath.XML
		if response.IsHTML() {
			format = xpath.HTML
		}
		doc, perr := xpath.Parse(text, format)
		if perr != nil {
			return nil, NewRunnerError(si, RunnerErrorQueryInvalidXML{}, false)
		}
		cache.setXML(doc)
	}
	return evalXPathDoc(cache.xml, expr, variables)
}

func evalQueryJSONPath(response *httpclient.Response, cache *bodyCache, expr ast.Template, variables *VariableSet, si ast.SourceInfo, useCoercion bool) (Value, *RunnerError) {
	if !cache.hasJSON {
		text, err := response.Text()
		if err != nil {
			return nil, NewRunnerError(si, RunnerErrorHTTP{Value: err}, false)
		}
		json, perr := parseJSON([]byte(text))
		if perr != nil {
			return nil, NewRunnerError(si, RunnerErrorQueryInvalidJSON{}, false)
		}
		cache.setJSON(json)
	}
	return evalJSONPathJSON(cache.json, expr, variables, useCoercion)
}

func evalQueryRegex(response *httpclient.Response, regex ast.RegexValue, variables *VariableSet, si ast.SourceInfo) (Value, *RunnerError) {
	s, herr := response.Text()
	if herr != nil {
		return nil, NewRunnerError(si, RunnerErrorHTTP{Value: herr}, false)
	}
	re, err := evalRegexValue(regex, variables)
	if err != nil {
		return nil, err
	}
	m := re.FindStringSubmatchIndex(s)
	if len(m) < 4 || m[2] < 0 {
		return nil, nil
	}
	return NewString(s[m[2]:m[3]]), nil
}

func evalQueryCertificate(response *httpclient.Response, attribute ast.CertificateAttributeName) Value {
	c := response.Certificate
	if c == nil {
		return nil
	}
	str := func(s *string) Value {
		if s == nil {
			return nil
		}
		return NewString(*s)
	}
	date := func(t *time.Time) Value {
		if t == nil {
			return nil
		}
		return ValueDate{t.UTC()}
	}
	switch attribute {
	case ast.CertificateAttributeSubject:
		return str(c.Subject())
	case ast.CertificateAttributeIssuer:
		return str(c.Issuer())
	case ast.CertificateAttributeStartDate:
		return date(c.StartDate())
	case ast.CertificateAttributeExpireDate:
		return date(c.ExpireDate())
	case ast.CertificateAttributeSerialNumber:
		return str(c.SerialNumber())
	case ast.CertificateAttributeSubjectAltName:
		return str(c.SubjectAltName())
	case ast.CertificateAttributeValue:
		return str(c.Value())
	}
	return nil
}

// evalRedirects returns the redirections of a response chain: each response
// but the last one, with the URL of the next response as location.
func evalRedirects(responses []*httpclient.Response) Value {
	values := []Value{}
	for i := range len(responses) - 1 {
		location := responses[i+1].Url
		values = append(values, ValueHTTPResponse{NewHTTPResponse(&location, responses[i].Status)})
	}
	return ValueList{values}
}

func evalCookieAttributeName(name ast.CookieAttributeName, cookie httpclient.ResponseCookie, si ast.SourceInfo) (Value, *RunnerError) {
	optString := func(s string, ok bool) Value {
		if !ok {
			return nil
		}
		return NewString(s)
	}
	switch name.Kind {
	case ast.CookieAttributeValue:
		return NewString(cookie.Value), nil
	case ast.CookieAttributeExpires:
		s, ok := cookie.Expires()
		if !ok {
			return nil, nil
		}
		if t, ok := parseCookieExpires(s); ok {
			return ValueDate{t}, nil
		}
		kind := RunnerErrorHTTP{Value: httpclient.HttpErrorCouldNotParseCookieExpires{Value: s}}
		return nil, NewRunnerError(si, kind, true)
	case ast.CookieAttributeMaxAge:
		v, ok := cookie.MaxAge()
		if !ok {
			return nil, nil
		}
		return NewInteger(v), nil
	case ast.CookieAttributeDomain:
		return optString(cookie.Domain()), nil
	case ast.CookieAttributePath:
		return optString(cookie.Path()), nil
	case ast.CookieAttributeSecure:
		if cookie.HasSecure() {
			return ValueUnit{}, nil
		}
		return nil, nil
	case ast.CookieAttributeHTTPOnly:
		if cookie.HasHttpOnly() {
			return ValueUnit{}, nil
		}
		return nil, nil
	case ast.CookieAttributeSameSite:
		return optString(cookie.SameSite()), nil
	}
	return nil, nil
}

var rfc2822Layouts = []string{
	"Mon, _2 Jan 2006 15:04:05 -0700",
	"Mon, _2 Jan 2006 15:04 -0700",
	"_2 Jan 2006 15:04:05 -0700",
	"_2 Jan 2006 15:04 -0700",
}

// parseCookieExpires parses a cookie Expires attribute, either as RFC 2822
// (like chrono's parse_from_rfc2822) or with dashes (Wed, 13-Jan-2021
// 22:23:01 GMT).
func parseCookieExpires(s string) (time.Time, bool) {
	normalized := strings.TrimSpace(s)
	for _, zone := range []string{" GMT", " UTC", " UT", " Z"} {
		if rest, ok := strings.CutSuffix(normalized, zone); ok {
			normalized = rest + " +0000"
			break
		}
	}
	for _, layout := range rfc2822Layouts {
		if t, err := time.Parse(layout, normalized); err == nil {
			return t.UTC(), true
		}
	}
	if t, err := time.Parse("Mon, _2-Jan-2006 15:04:05 GMT", s); err == nil {
		return t.UTC(), true
	}
	return time.Time{}, false
}
