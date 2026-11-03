package runner

import (
	"encoding/base64"
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// evalRequest transforms an AST request into a request spec, given a set of
// variables.
func evalRequest(request *ast.Request, variables *VariableSet, contextDir util.ContextDir) (httpclient.RequestSpec, *RunnerError) {
	var spec httpclient.RequestSpec
	spec.Method = httpclient.Method(request.Method.String())

	urlStr, err := evalTemplate(request.URL, variables)
	if err != nil {
		return spec, err
	}
	url, uerr := httpclient.ParseUrl(urlStr)
	if uerr != nil {
		kind := RunnerErrorInvalidURL{URL: uerr.Url, Message: uerr.Reason}
		return spec, NewRunnerError(request.URL.SourceInfo, kind, false)
	}
	spec.Url = url

	for _, h := range request.Headers {
		name, value, err := evalKeyValue(h, variables)
		if err != nil {
			return spec, err
		}
		spec.Headers.Push(httpclient.NewHeader(name, value))
	}

	if kv := request.BasicAuth(); kv != nil {
		name, value, err := evalKeyValue(*kv, variables)
		if err != nil {
			return spec, err
		}
		authorization := base64.StdEncoding.EncodeToString([]byte(name + ":" + value))
		spec.Headers.Push(httpclient.NewHeader(httpclient.HeaderAuthorization, "Basic "+authorization))
	}

	for _, p := range request.QueryStringParams() {
		name, value, err := evalKeyValue(p, variables)
		if err != nil {
			return spec, err
		}
		spec.Querystring = append(spec.Querystring, httpclient.NewParam(name, value))
	}

	for _, p := range request.FormParams() {
		name, value, err := evalKeyValue(p, variables)
		if err != nil {
			return spec, err
		}
		spec.Form = append(spec.Form, httpclient.NewParam(name, value))
	}

	for _, c := range request.Cookies() {
		name, err := evalTemplate(c.Name, variables)
		if err != nil {
			return spec, err
		}
		value, err := evalTemplate(c.Value, variables)
		if err != nil {
			return spec, err
		}
		spec.Cookies = append(spec.Cookies, httpclient.RequestCookie{Name: name, Value: value})
	}

	spec.Body = httpclient.BodyBinary{}
	if request.Body != nil {
		if spec.Body, err = evalBody(*request.Body, variables, contextDir); err != nil {
			return spec, err
		}
	}

	for _, p := range request.MultipartFormData() {
		param, err := evalMultipartParam(p, variables, contextDir)
		if err != nil {
			return spec, err
		}
		spec.Multipart = append(spec.Multipart, param)
	}

	switch {
	case len(spec.Form) > 0:
		spec.ImplicitContentType = "application/x-www-form-urlencoded"
	case len(spec.Multipart) > 0:
		spec.ImplicitContentType = "multipart/form-data"
	case request.Body != nil:
		spec.ImplicitContentType = implicitBodyContentType(request.Body.Value)
	}
	return spec, nil
}

func implicitBodyContentType(b ast.Bytes) string {
	switch b := b.(type) {
	case ast.BytesJSON:
		return "application/json"
	case ast.BytesXML:
		return "application/xml"
	case ast.BytesMultilineString:
		switch b.Value.Kind.(type) {
		case ast.MultilineStringGraphQL, ast.MultilineStringJSON:
			return "application/json"
		case ast.MultilineStringXML:
			return "application/xml"
		}
	}
	return ""
}

func evalKeyValue(kv ast.KeyValue, variables *VariableSet) (string, string, *RunnerError) {
	name, err := evalTemplate(kv.Key, variables)
	if err != nil {
		return "", "", err
	}
	value, err := evalTemplate(kv.Value, variables)
	if err != nil {
		return "", "", err
	}
	return name, value, nil
}

// cmdCookieStorageSet returns the cookie of the experimental
// "@cookie_storage_set:" comment, to add to the cookie store.
func cmdCookieStorageSet(request *ast.Request) (string, bool) {
	for _, lt := range request.LineTerminators {
		if lt.Comment == nil {
			continue
		}
		if s, ok := strings.CutPrefix(strings.TrimLeftFunc(lt.Comment.Value, unicode.IsSpace), "@cookie_storage_set:"); ok {
			return strings.TrimSpace(s), true
		}
	}
	return "", false
}

// cmdCookieStorageClear reports whether the experimental
// "@cookie_storage_clear" comment is set, to clear the cookie store.
func cmdCookieStorageClear(request *ast.Request) bool {
	for _, lt := range request.LineTerminators {
		if lt.Comment != nil && strings.HasPrefix(strings.TrimLeftFunc(lt.Comment.Value, unicode.IsSpace), "@cookie_storage_clear") {
			return true
		}
	}
	return false
}
