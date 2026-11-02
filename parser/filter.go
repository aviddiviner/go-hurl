package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func filters(r *reader.Reader) ([]ast.SpacedFilter, *ParseError) {
	var fs []ast.SpacedFilter
	for {
		save := r.Cursor()
		space, err := zeroOrMoreSpaces(r)
		if err != nil {
			return nil, err
		}
		if space.Value == "" {
			break
		}
		f, err := filter(r)
		if err != nil {
			if err.Recoverable {
				r.Seek(save)
				break
			}
			return nil, err
		}
		fs = append(fs, ast.SpacedFilter{Space0: space, Filter: f})
	}
	return fs, nil
}

type filterFunc = func(*reader.Reader) (ast.FilterValue, *ParseError)

var filterParsers []filterFunc

func init() {
	filterParsers = []filterFunc{
		keywordFilter("base64Decode", ast.FilterBase64Decode{}),
		keywordFilter("base64Encode", ast.FilterBase64Encode{}),
		keywordFilter("base64UrlSafeDecode", ast.FilterBase64URLSafeDecode{}),
		keywordFilter("base64UrlSafeEncode", ast.FilterBase64URLSafeEncode{}),
		charsetDecodeFilter,
		keywordFilter("count", ast.FilterCount{}),
		keywordFilter("daysAfterNow", ast.FilterDaysAfterNow{}),
		keywordFilter("daysBeforeNow", ast.FilterDaysBeforeNow{}),
		decodeFilter,
		keywordFilter("first", ast.FilterFirst{}),
		formatFilter,
		dateFormatFilter,
		keywordFilter("htmlUnescape", ast.FilterHTMLUnescape{}),
		keywordFilter("htmlEscape", ast.FilterHTMLEscape{}),
		jsonpathFilter,
		keywordFilter("last", ast.FilterLast{}),
		keywordFilter("location", ast.FilterLocation{}),
		nthFilter,
		regexFilter,
		replaceRegexFilter,
		replaceFilter,
		splitFilter,
		toDateFilter,
		keywordFilter("toFloat", ast.FilterToFloat{}),
		keywordFilter("toHex", ast.FilterToHex{}),
		keywordFilter("toInt", ast.FilterToInt{}),
		keywordFilter("toString", ast.FilterToString{}),
		keywordFilter("urlDecode", ast.FilterURLDecode{}),
		keywordFilter("urlEncode", ast.FilterURLEncode{}),
		urlQueryParamFilter,
		keywordFilter("utf8Decode", ast.FilterUTF8Decode{}),
		keywordFilter("utf8Encode", ast.FilterUTF8Encode{}),
		xpathFilter,
	}
}

func filter(r *reader.Reader) (ast.Filter, *ParseError) {
	start := r.Cursor()
	value, err := choice(filterParsers, r)
	if err != nil {
		if err.Recoverable {
			return ast.Filter{}, newError(err.Pos, true, ParseErrorExpecting{Value: "filter"})
		}
		return ast.Filter{}, err
	}
	return ast.Filter{SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos), Value: value}, nil
}

// keywordFilter parses a filter without argument.
func keywordFilter(name string, value ast.FilterValue) filterFunc {
	return func(r *reader.Reader) (ast.FilterValue, *ParseError) {
		if err := tryLiteral(name, r); err != nil {
			return nil, err
		}
		return value, nil
	}
}

// templateFilter parses a filter with a single quoted template argument.
// The template error is made non-recoverable if nonRecoverable is set.
func templateFilter(name string, nonRecoverable bool, build func(ast.Whitespace, ast.Template) ast.FilterValue) filterFunc {
	return func(r *reader.Reader) (ast.FilterValue, *ParseError) {
		if err := tryLiteral(name, r); err != nil {
			return nil, err
		}
		space0, err := oneOrMoreSpaces(r)
		if err != nil {
			return nil, err
		}
		t, err := quotedTemplate(r)
		if err != nil {
			if nonRecoverable {
				err = err.toNonRecoverable()
			}
			return nil, err
		}
		return build(space0, t), nil
	}
}

var (
	charsetDecodeFilter = templateFilter("charsetDecode", false, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterCharsetDecode{Space0: s, Encoding: t}
	})
	decodeFilter = templateFilter("decode", false, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterDecode{Space0: s, Encoding: t}
	})
	formatFilter = templateFilter("format", false, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterFormat{Space0: s, Fmt: t}
	})
	dateFormatFilter = templateFilter("dateFormat", false, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterDateFormat{Space0: s, Fmt: t}
	})
	jsonpathFilter = templateFilter("jsonpath", true, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterJSONPath{Space0: s, Expr: t}
	})
	splitFilter = templateFilter("split", true, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterSplit{Space0: s, Sep: t}
	})
	toDateFilter = templateFilter("toDate", false, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterToDate{Space0: s, Fmt: t}
	})
	urlQueryParamFilter = templateFilter("urlQueryParam", true, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterURLQueryParam{Space0: s, Param: t}
	})
	xpathFilter = templateFilter("xpath", true, func(s ast.Whitespace, t ast.Template) ast.FilterValue {
		return ast.FilterXPath{Space0: s, Expr: t}
	})
)

func nthFilter(r *reader.Reader) (ast.FilterValue, *ParseError) {
	if err := tryLiteral("nth", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	n, err := integerValue(r)
	if err != nil {
		return nil, err
	}
	return ast.FilterNth{Space0: space0, N: n}, nil
}

func integerValue(r *reader.Reader) (ast.IntegerValue, *ParseError) {
	start := r.Cursor()
	if v, err := integer(r); err == nil {
		return ast.IntegerLiteral{Value: v}, nil
	}
	r.Seek(start)
	p, err := placeholder(r)
	if err != nil {
		return nil, newError(err.Pos, false, ParseErrorExpecting{Value: "integer"})
	}
	return ast.IntegerPlaceholder{Value: p}, nil
}

func regexFilter(r *reader.Reader) (ast.FilterValue, *ParseError) {
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
	return ast.FilterRegex{Space0: space0, Value: value}, nil
}

func replaceFilter(r *reader.Reader) (ast.FilterValue, *ParseError) {
	if err := tryLiteral("replace", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	oldValue, err := nonRecover(quotedTemplate, r)
	if err != nil {
		return nil, err
	}
	space1, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	newValue, err := nonRecover(quotedTemplate, r)
	if err != nil {
		return nil, err
	}
	return ast.FilterReplace{Space0: space0, OldValue: oldValue, Space1: space1, NewValue: newValue}, nil
}

func replaceRegexFilter(r *reader.Reader) (ast.FilterValue, *ParseError) {
	if err := tryLiteral("replaceRegex", r); err != nil {
		return nil, err
	}
	space0, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	pattern, err := regexValue(r)
	if err != nil {
		return nil, err
	}
	space1, err := oneOrMoreSpaces(r)
	if err != nil {
		return nil, err
	}
	newValue, err := nonRecover(quotedTemplate, r)
	if err != nil {
		return nil, err
	}
	return ast.FilterReplaceRegex{Space0: space0, Pattern: pattern, Space1: space1, NewValue: newValue}, nil
}
