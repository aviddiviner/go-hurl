package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func requestSections(r *reader.Reader) ([]ast.Section, *ParseError) {
	return zeroOrMore(requestSection, r)
}

func responseSections(r *reader.Reader) ([]ast.Section, *ParseError) {
	return zeroOrMore(responseSection, r)
}

func requestSection(r *reader.Reader) (ast.Section, *ParseError) {
	return section(r, func(name string, r *reader.Reader) (ast.SectionValue, *ParseError) {
		switch name {
		case "Query":
			return sectionValueQueryParams(r, true)
		case "QueryStringParams":
			return sectionValueQueryParams(r, false)
		case "BasicAuth":
			return sectionValueBasicAuth(r)
		case "Form":
			return sectionValueFormParams(r, true)
		case "FormParams":
			return sectionValueFormParams(r, false)
		case "Multipart":
			return sectionValueMultipartFormData(r, true)
		case "MultipartFormData":
			return sectionValueMultipartFormData(r, false)
		case "Cookies":
			return sectionValueCookies(r)
		case "Options":
			return sectionValueOptions(r)
		}
		return nil, nil
	}, func(name string) ParseErrorKind { return ParseErrorRequestSectionName{Name: name} })
}

func responseSection(r *reader.Reader) (ast.Section, *ParseError) {
	return section(r, func(name string, r *reader.Reader) (ast.SectionValue, *ParseError) {
		switch name {
		case "Captures":
			return sectionValueCaptures(r)
		case "Asserts":
			return sectionValueAsserts(r)
		}
		return nil, nil
	}, func(name string) ParseErrorKind { return ParseErrorResponseSectionName{Name: name} })
}

// section parses a section header and its value with parseValue, which
// returns a nil value and error for an unknown section name. Unknown names
// are reported with invalidName.
func section(
	r *reader.Reader,
	parseValue func(string, *reader.Reader) (ast.SectionValue, *ParseError),
	invalidName func(string) ParseErrorKind,
) (ast.Section, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Section{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Section{}, err
	}
	start := r.Cursor()
	name, err := sectionName(r)
	if err != nil {
		return ast.Section{}, err
	}
	si := sourceInfo(start.Pos, r.Cursor().Pos)

	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Section{}, err
	}
	value, err := parseValue(name, r)
	if err != nil {
		return ast.Section{}, err
	}
	if value == nil {
		p := reader.Pos{Line: start.Pos.Line, Column: start.Pos.Column + 1}
		return ast.Section{}, newError(p, false, invalidName(name))
	}
	return ast.Section{
		LineTerminators: lts,
		Space0:          space0,
		LineTerminator0: lt0,
		Value:           value,
		SourceInfo:      si,
	}, nil
}

func sectionName(r *reader.Reader) (string, *ParseError) {
	p := r.Cursor().Pos
	if err := tryLiteral("[", r); err != nil {
		return "", err
	}
	name := r.ReadWhile(isAlphanumeric)
	if name == "" {
		// Could be an empty JSON array for the body.
		return "", newError(p, true, ParseErrorExpecting{Value: "a valid section name"})
	}
	if err := tryLiteral("]", r); err != nil {
		return "", err
	}
	return name, nil
}

func sectionValueQueryParams(r *reader.Reader, short bool) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(keyValue, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionQueryParams{Value: items, Short: short}, nil
}

func sectionValueBasicAuth(r *reader.Reader) (ast.SectionValue, *ParseError) {
	v, err := optional(keyValue, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionBasicAuth{Value: v}, nil
}

func sectionValueFormParams(r *reader.Reader, short bool) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(keyValue, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionFormParams{Value: items, Short: short}, nil
}

func sectionValueMultipartFormData(r *reader.Reader, short bool) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(multipartParam, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionMultipartFormData{Value: items, Short: short}, nil
}

func sectionValueCookies(r *reader.Reader) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(cookie, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionCookies{Value: items}, nil
}

func sectionValueCaptures(r *reader.Reader) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(capture, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionCaptures{Value: items}, nil
}

func sectionValueAsserts(r *reader.Reader) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(assert, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionAsserts{Value: items}, nil
}

func sectionValueOptions(r *reader.Reader) (ast.SectionValue, *ParseError) {
	items, err := zeroOrMore(entryOption, r)
	if err != nil {
		return nil, err
	}
	return ast.SectionOptions{Value: items}, nil
}

func cookie(r *reader.Reader) (ast.Cookie, *ParseError) {
	kv, err := keyValue(r)
	if err != nil {
		return ast.Cookie{}, err
	}
	return ast.Cookie{
		LineTerminators: kv.LineTerminators,
		Space0:          kv.Space0,
		Name:            kv.Key,
		Space1:          kv.Space1,
		Space2:          kv.Space2,
		Value:           kv.Value,
		LineTerminator0: kv.LineTerminator0,
	}, nil
}

func multipartParam(r *reader.Reader) (ast.MultipartParam, *ParseError) {
	save := r.Cursor()
	f, err := fileParam(r)
	if err == nil {
		return ast.MultipartParamFilenameParam{Value: f}, nil
	}
	if !err.Recoverable {
		return nil, err
	}
	r.Seek(save)
	kv, err := keyValue(r)
	if err != nil {
		return nil, err
	}
	return ast.MultipartParamParam{Value: kv}, nil
}

func fileParam(r *reader.Reader) (ast.FilenameParam, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	key, err := recover(keyString, r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	if err := recoverLiteral(":", r); err != nil {
		return ast.FilenameParam{}, err
	}
	space2, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	value, err := fileValue(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.FilenameParam{}, err
	}
	return ast.FilenameParam{
		LineTerminators: lts,
		Space0:          space0,
		Key:             key,
		Space1:          space1,
		Space2:          space2,
		Value:           value,
		LineTerminator0: lt0,
	}, nil
}

func fileValue(r *reader.Reader) (ast.FilenameValue, *ParseError) {
	if err := tryLiteral("file,", r); err != nil {
		return ast.FilenameValue{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.FilenameValue{}, err
	}
	f, err := filename(r)
	if err != nil {
		return ast.FilenameValue{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.FilenameValue{}, err
	}
	if err := literal(";", r); err != nil {
		return ast.FilenameValue{}, err
	}
	save := r.Cursor()
	var space2 ast.Whitespace
	var contentType *ast.Template
	_, err = lineTerminator(r)
	r.Seek(save)
	if err == nil {
		space2 = ast.Whitespace{SourceInfo: sourceInfo(save.Pos, save.Pos)}
	} else {
		space2, err = zeroOrMoreSpaces(r)
		if err != nil {
			return ast.FilenameValue{}, err
		}
		start := r.Cursor()
		ct, err := unquotedTemplate(r)
		if err != nil {
			return ast.FilenameValue{}, newError(start.Pos, false, ParseErrorFileContentType{})
		}
		contentType = &ct
	}
	return ast.FilenameValue{Space0: space0, Filename: f, Space1: space1, Space2: space2, ContentType: contentType}, nil
}

func capture(r *reader.Reader) (ast.Capture, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Capture{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Capture{}, err
	}
	name, err := recover(keyString, r)
	if err != nil {
		return ast.Capture{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Capture{}, err
	}
	if err := recoverLiteral(":", r); err != nil {
		return ast.Capture{}, err
	}
	space2, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Capture{}, err
	}
	q, err := query(r)
	if err != nil {
		return ast.Capture{}, err
	}
	fs, err := filters(r)
	if err != nil {
		return ast.Capture{}, err
	}
	ws, err := optional(redacted, r)
	if err != nil {
		return ast.Capture{}, err
	}
	isRedacted := ws != nil
	var space3 ast.Whitespace
	if isRedacted {
		space3 = *ws
	} else {
		// Without the `redact` keyword, space3 is empty and any remaining
		// whitespace is consumed by the line terminator.
		p := r.Cursor().Pos
		space3 = ast.Whitespace{SourceInfo: sourceInfo(p, p)}
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Capture{}, err
	}
	return ast.Capture{
		LineTerminators: lts,
		Space0:          space0,
		Name:            name,
		Space1:          space1,
		Space2:          space2,
		Query:           q,
		Filters:         fs,
		Space3:          space3,
		Redacted:        isRedacted,
		LineTerminator0: lt0,
	}, nil
}

func redacted(r *reader.Reader) (ast.Whitespace, *ParseError) {
	space, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Whitespace{}, err
	}
	if err := tryLiteral("redact", r); err != nil {
		return ast.Whitespace{}, err
	}
	return space, nil
}

func assert(r *reader.Reader) (ast.Assert, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.Assert{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.Assert{}, err
	}
	q, err := query(r)
	if err != nil {
		return ast.Assert{}, err
	}
	fs, err := filters(r)
	if err != nil {
		return ast.Assert{}, err
	}
	space1, err := oneOrMoreSpaces(r)
	if err != nil {
		return ast.Assert{}, err
	}
	p, err := predicate(r)
	if err != nil {
		return ast.Assert{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.Assert{}, err
	}
	return ast.Assert{
		LineTerminators: lts,
		Space0:          space0,
		Query:           q,
		Filters:         fs,
		Space1:          space1,
		Predicate:       p,
		LineTerminator0: lt0,
	}, nil
}
