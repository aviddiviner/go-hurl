package parser

import (
	"slices"
	"strconv"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func hurlFile(r *reader.Reader) (ast.HurlFile, *ParseError) {
	entries, err := zeroOrMore(entry, r)
	if err != nil {
		return ast.HurlFile{}, err
	}
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.HurlFile{}, err
	}
	if err := eof(r); err != nil {
		return ast.HurlFile{}, err
	}
	return ast.HurlFile{Entries: entries, LineTerminators: lts}, nil
}

func entry(r *reader.Reader) (ast.Entry, *ParseError) {
	req, err := request(r)
	if err != nil {
		return ast.Entry{}, err
	}
	resp, err := optional(response, r)
	if err != nil {
		return ast.Entry{}, err
	}
	return ast.Entry{Request: req, Response: resp}, nil
}

func request(r *reader.Reader) (ast.Request, *ParseError) {
	start := r.Cursor()
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Request{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Request{}, err
	}
	m, err := method(r)
	if err != nil {
		return ast.Request{}, err
	}
	space1, err := oneOrMoreSpaces(r)
	if err != nil {
		return ast.Request{}, err
	}
	url, err := unquotedTemplate(r)
	if err != nil {
		return ast.Request{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Request{}, err
	}
	headers, err := zeroOrMore(keyValue, r)
	if err != nil {
		return ast.Request{}, err
	}
	sections, err := requestSections(r)
	if err != nil {
		return ast.Request{}, err
	}
	b, err := optional(body, r)
	if err != nil {
		return ast.Request{}, err
	}
	si := sourceInfo(start.Pos, r.Cursor().Pos)
	if err := checkDuplicatedSections(sections); err != nil {
		return ast.Request{}, err
	}
	return ast.Request{
		LineTerminators: lts,
		Space0:          space0,
		Method:          m,
		Space1:          space1,
		URL:             url,
		LineTerminator0: lt0,
		Headers:         headers,
		Sections:        sections,
		Body:            b,
		SourceInfo:      si,
	}, nil
}

func response(r *reader.Reader) (ast.Response, *ParseError) {
	start := r.Cursor()
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Response{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Response{}, err
	}
	v, err := version(r)
	if err != nil {
		return ast.Response{}, err
	}
	space1, err := oneOrMoreSpaces(r)
	if err != nil {
		return ast.Response{}, err
	}
	s, err := status(r)
	if err != nil {
		return ast.Response{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Response{}, err
	}
	headers, err := zeroOrMore(keyValue, r)
	if err != nil {
		return ast.Response{}, err
	}
	sections, err := responseSections(r)
	if err != nil {
		return ast.Response{}, err
	}
	b, err := optional(body, r)
	if err != nil {
		return ast.Response{}, err
	}
	si := sourceInfo(start.Pos, r.Cursor().Pos)
	if err := checkDuplicatedSections(sections); err != nil {
		return ast.Response{}, err
	}
	return ast.Response{
		LineTerminators: lts,
		Version:         v,
		Space0:          space0,
		Status:          s,
		Space1:          space1,
		LineTerminator0: lt0,
		Headers:         headers,
		Sections:        sections,
		Body:            b,
		SourceInfo:      si,
	}, nil
}

func checkDuplicatedSections(sections []ast.Section) *ParseError {
	var names []string
	for _, s := range sections {
		if slices.Contains(names, s.Identifier()) {
			return newError(s.SourceInfo.Start, false, ParseErrorDuplicateSection{})
		}
		names = append(names, s.Identifier())
	}
	return nil
}

func method(r *reader.Reader) (ast.Method, *ParseError) {
	if r.IsEOF() {
		return "", newError(r.Cursor().Pos, true, ParseErrorMethod{Name: "<EOF>"})
	}
	start := r.Cursor()
	name := r.ReadWhile(isASCIIAlphabetic)
	if name == "" || strings.ToUpper(name) != name {
		return "", newError(start.Pos, false, ParseErrorMethod{Name: name})
	}
	return ast.Method(name), nil
}

func version(r *reader.Reader) (ast.Version, *ParseError) {
	start := r.Cursor()
	if err := tryLiteral("HTTP", r); err != nil {
		return ast.Version{}, err
	}
	c, _ := r.Peek()
	switch c {
	case '/':
		for _, v := range []struct {
			s     string
			value ast.VersionValue
		}{
			{"/1.0", ast.Version1},
			{"/1.1", ast.Version11},
			{"/2", ast.Version2},
			{"/3", ast.Version3},
		} {
			if tryLiteral(v.s, r) == nil {
				return ast.Version{Value: v.value, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
			}
		}
	case ' ', '\t':
		return ast.Version{Value: ast.VersionAny, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
	}
	return ast.Version{}, newError(start.Pos, false, ParseErrorVersion{})
}

func status(r *reader.Reader) (ast.Status, *ParseError) {
	start := r.Cursor()
	var value ast.StatusValue
	if tryLiteral("*", r) == nil {
		value = ast.StatusAny{}
	} else {
		s := r.ReadWhile(isASCIIDigit)
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return ast.Status{}, newError(start.Pos, false, ParseErrorStatus{})
		}
		value = ast.StatusSpecific{Value: n}
	}
	return ast.Status{Value: value, SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func body(r *reader.Reader) (ast.Body, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Body{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Body{}, err
	}
	value, err := bytes(r)
	if err != nil {
		return ast.Body{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Body{}, err
	}
	return ast.Body{LineTerminators: lts, Space0: space0, Value: value, LineTerminator0: lt0}, nil
}
