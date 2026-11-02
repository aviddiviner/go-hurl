package jsonpath

import "github.com/aviddiviner/go-hurl/reader"

func parseError(pos reader.Pos, kind ParseErrorKind) *ParseError {
	return &ParseError{Pos: pos, Kind: kind}
}

// expectStr consumes s, or fails at the current position.
// On failure, the reader is not reset.
func expectStr(s string, r *reader.Reader) *ParseError {
	start := r.Cursor()
	if r.IsEOF() {
		return parseError(start.Pos, Expecting(s))
	}
	for _, c := range s {
		x, ok := r.Read()
		if !ok || x != c {
			return parseError(start.Pos, Expecting(s))
		}
	}
	return nil
}

// matchStr consumes s if it is next, otherwise leaves the reader unchanged.
func matchStr(s string, r *reader.Reader) bool {
	save := r.Cursor()
	if expectStr(s, r) == nil {
		return true
	}
	r.Seek(save)
	return false
}

func skipWhitespace(r *reader.Reader) {
	for {
		c, ok := r.Peek()
		if !ok || (c != ' ' && c != '\t' && c != '\n' && c != '\r') {
			return
		}
		r.Read()
	}
}

func parseQuery(r *reader.Reader) (*Query, *ParseError) {
	if err := expectStr("$", r); err != nil {
		return nil, err
	}
	segments, err := parseSegments(r, 0)
	if err != nil {
		return nil, err
	}
	if !r.IsEOF() {
		return nil, parseError(r.Cursor().Pos, Expecting("end of query"))
	}
	return &Query{segments: segments}, nil
}

func tryFilterQuery(r *reader.Reader, depth int) (filterQuery, *ParseError) {
	if q, err := tryRelativeQuery(r, depth); err != nil {
		return nil, err
	} else if q != nil {
		return *q, nil
	}
	if q, err := tryAbsoluteQuery(r, depth); err != nil {
		return nil, err
	} else if q != nil {
		return q, nil
	}
	return nil, nil
}

func tryRelativeQuery(r *reader.Reader, depth int) (*relativeQuery, *ParseError) {
	if !matchStr("@", r) {
		return nil, nil
	}
	segments, err := parseSegments(r, depth)
	if err != nil {
		return nil, err
	}
	return &relativeQuery{segments: segments}, nil
}

func tryAbsoluteQuery(r *reader.Reader, depth int) (*Query, *ParseError) {
	if !matchStr("$", r) {
		return nil, nil
	}
	segments, err := parseSegments(r, depth)
	if err != nil {
		return nil, err
	}
	return &Query{segments: segments}, nil
}

func parseSegments(r *reader.Reader, depth int) ([]segment, *ParseError) {
	var segments []segment
	for {
		s, err := trySegment(r, depth)
		if err != nil {
			return nil, err
		}
		if s == nil {
			return segments, nil
		}
		segments = append(segments, s)
	}
}

func trySegment(r *reader.Reader, depth int) (segment, *ParseError) {
	save := r.Cursor()
	skipWhitespace(r)
	if s, err := tryDescendantSegment(r, depth); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	if s, err := tryChildSegment(r, depth); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	r.Seek(save)
	return nil, nil
}

func tryChildSegment(r *reader.Reader, depth int) (*childSegment, *ParseError) {
	selectors, err := tryBracketedSelection(r, depth)
	if err != nil {
		return nil, err
	}
	if selectors != nil {
		return &childSegment{selectors: selectors}, nil
	}
	if !matchStr(".", r) {
		return nil, nil
	}
	save := r.Cursor()
	if matchStr("*", r) {
		return &childSegment{selectors: []selector{wildcardSelector{}}}, nil
	}
	if name, err := memberNameShorthand(r); err == nil {
		return &childSegment{selectors: []selector{nameSelector{value: name}}}, nil
	}
	return nil, parseError(save.Pos, Expecting("a wildcard-selector or member-name shorthand"))
}

func tryDescendantSegment(r *reader.Reader, depth int) (*descendantSegment, *ParseError) {
	if !matchStr("..", r) {
		return nil, nil
	}
	save := r.Cursor()
	selectors, err := tryBracketedSelection(r, depth)
	if err != nil {
		return nil, err
	}
	if selectors != nil {
		return &descendantSegment{selectors: selectors}, nil
	}
	if matchStr("*", r) {
		return &descendantSegment{selectors: []selector{wildcardSelector{}}}, nil
	}
	if name, err := memberNameShorthand(r); err == nil {
		return &descendantSegment{selectors: []selector{nameSelector{value: name}}}, nil
	}
	return nil, parseError(save.Pos, Expecting("a bracketed-selection, wildcard-selector or member-name shorthand"))
}

// tryBracketedSelection returns nil if there is no bracketed selection.
func tryBracketedSelection(r *reader.Reader, depth int) ([]selector, *ParseError) {
	if !matchStr("[", r) {
		return nil, nil
	}
	skipWhitespace(r)
	selectors, err := parseSelectors(r, depth)
	if err != nil {
		return nil, err
	}
	skipWhitespace(r)
	if err := expectStr("]", r); err != nil {
		return nil, err
	}
	return selectors, nil
}

func memberNameShorthand(r *reader.Reader) (string, *ParseError) {
	c, ok := nameFirst(r)
	if !ok {
		return "", parseError(r.Cursor().Pos, Expecting("a member name"))
	}
	value := []rune{c}
	for {
		c, ok := nameChar(r)
		if !ok {
			return string(value), nil
		}
		value = append(value, c)
	}
}

func nameFirst(r *reader.Reader) (rune, bool) {
	save := r.Cursor()
	c, ok := r.Read()
	if !ok {
		return 0, false
	}
	if 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || c == '_' ||
		0x80 <= c && c <= 0xD7FF || 0xE000 <= c && c <= 0x10FFFF {
		return c, true
	}
	r.Seek(save)
	return 0, false
}

func nameChar(r *reader.Reader) (rune, bool) {
	if c, ok := nameFirst(r); ok {
		return c, true
	}
	return digit(r)
}

func digit(r *reader.Reader) (rune, bool) {
	save := r.Cursor()
	c, ok := r.Read()
	if !ok {
		return 0, false
	}
	if isDigit(c) {
		return c, true
	}
	r.Seek(save)
	return 0, false
}

func isDigit(c rune) bool { return '0' <= c && c <= '9' }

func parseSelectors(r *reader.Reader, depth int) ([]selector, *ParseError) {
	var selectors []selector
	for {
		s, err := parseSelector(r, depth)
		if err != nil {
			return nil, err
		}
		selectors = append(selectors, s)
		skipWhitespace(r)
		if !matchStr(",", r) {
			return selectors, nil
		}
		skipWhitespace(r)
	}
}

func parseSelector(r *reader.Reader, depth int) (selector, *ParseError) {
	initial := r.Cursor()
	if s, err := tryNameSelector(r); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	if matchStr("*", r) {
		return wildcardSelector{}, nil
	}
	if s, err := tryArraySliceSelector(r); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	if s, err := tryIndexSelector(r); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	if s, err := tryFilterSelector(r, depth); err != nil {
		return nil, err
	} else if s != nil {
		return *s, nil
	}
	return nil, parseError(initial.Pos, Expecting("a selector"))
}

func tryNameSelector(r *reader.Reader) (*nameSelector, *ParseError) {
	s, ok, err := tryStringLiteral(r)
	if err != nil || !ok {
		return nil, err
	}
	return &nameSelector{value: s}, nil
}

func tryIndexSelector(r *reader.Reader) (*indexSelector, *ParseError) {
	v, ok, err := tryInteger(r)
	if err != nil || !ok {
		return nil, err
	}
	return &indexSelector{value: v}, nil
}

func tryArraySliceSelector(r *reader.Reader) (*arraySliceSelector, *ParseError) {
	save := r.Cursor()
	optInteger := func() (*int64, *ParseError) {
		v, ok, err := tryInteger(r)
		if err != nil || !ok {
			return nil, err
		}
		skipWhitespace(r)
		return &v, nil
	}
	start, err := optInteger()
	if err != nil {
		return nil, err
	}
	if !matchStr(":", r) {
		// Not a slice, but may still be an index or a name selector.
		r.Seek(save)
		return nil, nil
	}
	skipWhitespace(r)
	end, err := optInteger()
	if err != nil {
		return nil, err
	}
	step := int64(1)
	if matchStr(":", r) {
		skipWhitespace(r)
		v, ok, err := tryInteger(r)
		if err != nil {
			return nil, err
		}
		if ok {
			step = v
		}
	}
	return &arraySliceSelector{start: start, end: end, step: step}, nil
}

func tryFilterSelector(r *reader.Reader, depth int) (*filterSelector, *ParseError) {
	if !matchStr("?", r) {
		return nil, nil
	}
	skipWhitespace(r)
	expr, err := logicalOrExpr(r, depth)
	if err != nil {
		return nil, err
	}
	return &filterSelector{expr: expr}, nil
}
