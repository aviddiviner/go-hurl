package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func query(r *reader.Reader) (ast.Query, *ParseError) {
	start := r.Cursor()
	value, err := queryValue(r)
	if err != nil {
		return ast.Query{}, err
	}
	return ast.Query{SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos), Value: value}, nil
}

type queryFunc = func(*reader.Reader) (ast.QueryValue, *ParseError)

var queryParsers []queryFunc

func init() {
	queryParsers = []queryFunc{
		keywordQuery("status", ast.QueryStatus{}),
		keywordQuery("version", ast.QueryVersion{}),
		keywordQuery("url", ast.QueryURL{}),
		headerQuery,
		cookieQuery,
		keywordQuery("body", ast.QueryBody{}),
		xpathQuery,
		jsonpathQuery,
		regexQuery,
		variableQuery,
		keywordQuery("duration", ast.QueryDuration{}),
		keywordQuery("bytes", ast.QueryBytes{}),
		keywordQuery("rawbytes", ast.QueryRawBytes{}),
		keywordQuery("sha256", ast.QuerySHA256{}),
		keywordQuery("md5", ast.QueryMD5{}),
		certificateQuery,
		keywordQuery("ip", ast.QueryIP{}),
		keywordQuery("redirects", ast.QueryRedirects{}),
	}
}

func queryValue(r *reader.Reader) (ast.QueryValue, *ParseError) {
	return choice(queryParsers, r)
}

func keywordQuery(name string, value ast.QueryValue) queryFunc {
	return func(r *reader.Reader) (ast.QueryValue, *ParseError) {
		if err := tryLiteral(name, r); err != nil {
			return nil, err
		}
		return value, nil
	}
}

// templateQuery parses a query with a single quoted template argument.
func templateQuery(name string, build func(ast.Whitespace, ast.Template) ast.QueryValue) queryFunc {
	return func(r *reader.Reader) (ast.QueryValue, *ParseError) {
		if err := tryLiteral(name, r); err != nil {
			return nil, err
		}
		space0, err := oneOrMoreSpaces(r)
		if err != nil {
			return nil, err
		}
		t, err := nonRecover(quotedTemplate, r)
		if err != nil {
			return nil, err
		}
		return build(space0, t), nil
	}
}

var (
	headerQuery = templateQuery("header", func(s ast.Whitespace, t ast.Template) ast.QueryValue {
		return ast.QueryHeader{Space0: s, Name: t}
	})
	xpathQuery = templateQuery("xpath", func(s ast.Whitespace, t ast.Template) ast.QueryValue {
		return ast.QueryXPath{Space0: s, Expr: t}
	})
	jsonpathQuery = templateQuery("jsonpath", func(s ast.Whitespace, t ast.Template) ast.QueryValue {
		return ast.QueryJSONPath{Space0: s, Expr: t}
	})
	variableQuery = templateQuery("variable", func(s ast.Whitespace, t ast.Template) ast.QueryValue {
		return ast.QueryVariable{Space0: s, Name: t}
	})
)

func cookieQuery(r *reader.Reader) (ast.QueryValue, *ParseError) {
	if err := tryLiteral("cookie", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}

	// The cookie path value (without quotes) is parsed with a dedicated
	// reader.
	start := r.Cursor()
	s, err := quotedOnelineString(r)
	if err != nil {
		return nil, err
	}
	p := reader.Pos{Line: start.Pos.Line, Column: start.Pos.Column + 1}
	e, err := cookiepath(reader.WithPos(s, p))
	if err != nil {
		return nil, err
	}
	return ast.QueryCookie{Space0: space0, Expr: e}, nil
}

func regexQuery(r *reader.Reader) (ast.QueryValue, *ParseError) {
	if err := tryLiteral("regex", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	value, err := regexValue(r)
	if err != nil {
		return nil, err
	}
	return ast.QueryRegex{Space0: space0, Value: value}, nil
}

func regexValue(r *reader.Reader) (ast.RegexValue, *ParseError) {
	v, err := choice([]func(*reader.Reader) (ast.RegexValue, *ParseError){
		func(r *reader.Reader) (ast.RegexValue, *ParseError) {
			t, err := quotedTemplate(r)
			if err != nil {
				return nil, err
			}
			return ast.RegexTemplate{Value: t}, nil
		},
		func(r *reader.Reader) (ast.RegexValue, *ParseError) {
			re, err := regex(r)
			if err != nil {
				return nil, err
			}
			return ast.RegexRegex{Value: re}, nil
		},
	}, r)
	if err != nil {
		return nil, newError(err.Pos, false, ParseErrorExpecting{Value: `" or /`})
	}
	return v, nil
}

func certificateQuery(r *reader.Reader) (ast.QueryValue, *ParseError) {
	if err := tryLiteral("certificate", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	field, err := certificateField(r)
	if err != nil {
		return nil, err
	}
	return ast.QueryCertificate{Space0: space0, AttributeName: field}, nil
}

func certificateField(r *reader.Reader) (ast.CertificateAttributeName, *ParseError) {
	if err := literal(`"`, r); err != nil {
		return 0, err
	}
	for _, f := range []struct {
		name string
		attr ast.CertificateAttributeName
	}{
		{`Subject"`, ast.CertificateAttributeSubject},
		{`Issuer"`, ast.CertificateAttributeIssuer},
		{`Start-Date"`, ast.CertificateAttributeStartDate},
		{`Expire-Date"`, ast.CertificateAttributeExpireDate},
		{`Serial-Number"`, ast.CertificateAttributeSerialNumber},
		{`Subject-Alt-Name"`, ast.CertificateAttributeSubjectAltName},
		{`Value"`, ast.CertificateAttributeValue},
	} {
		if tryLiteral(f.name, r) == nil {
			return f.attr, nil
		}
	}
	value := "Field <Subject>, <Issuer>, <Start-Date>, <Expire-Date>, <Serial-Number>, <Subject-Alt-Name> or <Value>"
	return 0, newError(r.Cursor().Pos, false, ParseErrorExpecting{Value: value})
}
