package parser

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func multilineString(r *reader.Reader) (ast.MultilineString, *ParseError) {
	if err := tryLiteral("```", r); err != nil {
		return ast.MultilineString{}, err
	}
	return choice([]func(*reader.Reader) (ast.MultilineString, *ParseError){
		jsonText, xmlText, graphql, rawText, plainText,
	}, r)
}

// textBlock parses a multiline text block, with an optional language hint lang
// (like ```json). Multiline strings don't escape chars, and evaluate
// placeholders if templatized (raw multilines are not).
func textBlock(lang string, templatized bool, r *reader.Reader) (space, nl ast.Whitespace, value ast.Template, err *ParseError) {
	if lang != "" {
		if err = tryLiteral(lang, r); err != nil {
			return
		}
	}
	if space, err = zeroOrMoreSpaces(r); err != nil {
		return
	}

	// Supported language hints have been processed by earlier parse
	// functions, so anything left before the newline is an invalid hint.
	start := r.Cursor()
	if hint := languageHint(r); hint != "" {
		err = newError(start.Pos, false, ParseErrorMultilineLanguageHint{Value: hint})
		return
	}
	if nl, err = newline(r); err != nil {
		return
	}
	value, err = multilineStringValue(templatized, r)
	return
}

func multilineText(lang string, templatized bool, kind func(ast.Template) ast.MultilineStringKind) func(*reader.Reader) (ast.MultilineString, *ParseError) {
	return func(r *reader.Reader) (ast.MultilineString, *ParseError) {
		space, nl, value, err := textBlock(lang, templatized, r)
		if err != nil {
			return ast.MultilineString{}, err
		}
		return ast.MultilineString{Space: space, Newline: nl, Kind: kind(value)}, nil
	}
}

var (
	plainText = multilineText("", true, func(t ast.Template) ast.MultilineStringKind { return ast.MultilineStringText{Value: t} })
	rawText   = multilineText("raw", false, func(t ast.Template) ast.MultilineStringKind { return ast.MultilineStringRaw{Value: t} })
	jsonText  = multilineText("json", true, func(t ast.Template) ast.MultilineStringKind { return ast.MultilineStringJSON{Value: t} })
	xmlText   = multilineText("xml", true, func(t ast.Template) ast.MultilineStringKind { return ast.MultilineStringXML{Value: t} })
)

func graphql(r *reader.Reader) (ast.MultilineString, *ParseError) {
	if err := tryLiteral("graphql", r); err != nil {
		return ast.MultilineString{}, err
	}
	_ = tryLiteral(",", r)
	space, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.MultilineString{}, err
	}
	nl, err := newline(r)
	if err != nil {
		return ast.MultilineString{}, err
	}

	var chars []EncodedChar
	start := r.Cursor()
	build := func(end reader.Cursor, variables *ast.GraphQLVariables) (ast.MultilineString, *ParseError) {
		elements, err := Templatize(EncodedString{SourceInfo: sourceInfo(start.Pos, end.Pos), Chars: chars})
		if err != nil {
			return ast.MultilineString{}, err
		}
		value := ast.Template{Elements: elements, SourceInfo: sourceInfo(start.Pos, end.Pos)}
		return ast.MultilineString{
			Space:   space,
			Newline: nl,
			Kind:    ast.MultilineStringGraphQL{Value: ast.GraphQL{Value: value, Variables: variables}},
		}, nil
	}

	for r.PeekN(3) != "```" && !r.IsEOF() {
		p := r.Cursor().Pos
		c, _ := r.Read()
		chars = append(chars, EncodedChar{c, string(c), p})
		if c != '\n' {
			continue
		}
		end := r.Cursor()
		variables, err := optional(graphqlVariables, r)
		if err != nil {
			return ast.MultilineString{}, err
		}
		if variables == nil {
			continue
		}
		if err := literal("```", r); err != nil {
			return ast.MultilineString{}, err
		}
		return build(end, variables)
	}
	end := r.Cursor()
	if err := literal("```", r); err != nil {
		return ast.MultilineString{}, err
	}
	return build(end, nil)
}

func whitespace(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	c, ok := r.Read()
	if !ok || !strings.ContainsRune(" \t\n\r", c) {
		return ast.Whitespace{}, newError(start.Pos, true, ParseErrorSpace{})
	}
	return ast.Whitespace{Value: string(c), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func zeroOrMoreWhitespaces(r *reader.Reader) (ast.Whitespace, *ParseError) {
	start := r.Cursor()
	v, err := zeroOrMore(whitespace, r)
	if err != nil {
		return ast.Whitespace{}, err
	}
	return ast.Whitespace{Value: joinWhitespace(v), SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos)}, nil
}

func languageHint(r *reader.Reader) string {
	return r.ReadWhile(func(c rune) bool { return c != '\n' && c != '\r' && c != '`' })
}

func graphqlVariables(r *reader.Reader) (ast.GraphQLVariables, *ParseError) {
	if err := tryLiteral("variables", r); err != nil {
		return ast.GraphQLVariables{}, err
	}
	space, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.GraphQLVariables{}, err
	}
	start := r.Cursor()
	value, err := objectValue(r, 0)
	if err != nil {
		return ast.GraphQLVariables{}, newError(start.Pos, false, ParseErrorGraphQLVariables{})
	}
	ws, err := zeroOrMoreWhitespaces(r)
	if err != nil {
		return ast.GraphQLVariables{}, err
	}
	return ast.GraphQLVariables{Space: space, Value: value, Whitespace: ws}, nil
}

func multilineStringValue(templatize bool, r *reader.Reader) (ast.Template, *ParseError) {
	var chars []EncodedChar
	start := r.Cursor()
	for r.PeekN(3) != "```" && !r.IsEOF() {
		p := r.Cursor().Pos
		c, _ := r.Read()
		chars = append(chars, EncodedChar{c, string(c), p})
	}
	end := r.Cursor()
	if err := literal("```", r); err != nil {
		return ast.Template{}, err
	}

	si := sourceInfo(start.Pos, end.Pos)
	var elements []ast.TemplateElement
	if templatize {
		var err *ParseError
		elements, err = Templatize(EncodedString{SourceInfo: si, Chars: chars})
		if err != nil {
			return ast.Template{}, err
		}
	} else {
		var b strings.Builder
		for _, c := range chars {
			b.WriteRune(c.Value)
		}
		elements = []ast.TemplateElement{str(b.String())}
	}
	return ast.Template{Elements: elements, SourceInfo: si}, nil
}

func str(s string) ast.TemplateString { return ast.TemplateString{Value: s, Source: s} }
