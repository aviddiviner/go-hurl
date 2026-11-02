package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func mlText(space, nl ast.Whitespace, value ast.Template) ast.MultilineString {
	return ast.MultilineString{Space: space, Newline: nl, Kind: ast.MultilineStringText{Value: value}}
}

func TestMultilineString(t *testing.T) {
	lines := strTmpl("line1\nline2\nline3\n", si(2, 1, 5, 1))
	graphQL := func(space ast.Whitespace, nl ast.Whitespace) ast.MultilineString {
		return ast.MultilineString{Space: space, Newline: nl, Kind: ast.MultilineStringGraphQL{Value: ast.GraphQL{Value: lines}}}
	}
	tests := []struct {
		input string
		want  ast.MultilineString
	}{
		{"```\nline1\nline2\nline3\n```", mlText(ews(1, 4), ws("\n", 1, 4, 2, 1), lines)},
		{"```         \nline1\nline2\nline3\n```", mlText(ws("         ", 1, 4, 1, 13), ws("\n", 1, 13, 2, 1), lines)},
		{"```\nfoo\n{{var_1}} bar {{var2}}\nline3\n```", mlText(ews(1, 4), ws("\n", 1, 4, 2, 1),
			tmpl(0, si(2, 1, 5, 1), str("foo\n"), phElem("var_1", 3, 3), str(" bar "), phElem("var2", 3, 17), str("\nline3\n")))},
		{"```json\nline1\nline2\nline3\n```", ast.MultilineString{
			Space: ews(1, 8), Newline: ws("\n", 1, 8, 2, 1), Kind: ast.MultilineStringJSON{Value: lines},
		}},
		// JSON multilines don't escape text (so Hurl Unicode literals are not supported).
		{"```json\n{\n  \"g_clef\": \"\\u{1D11E}\"\n}\n```", ast.MultilineString{
			Space: ews(1, 8), Newline: ws("\n", 1, 8, 2, 1),
			Kind: ast.MultilineStringJSON{Value: strTmpl("{\n  \"g_clef\": \"\\u{1D11E}\"\n}\n", si(2, 1, 5, 1))},
		}},
		{"```graphql\nline1\nline2\nline3\n```", graphQL(ews(1, 11), ws("\n", 1, 11, 2, 1))},
		{"```graphql      \nline1\nline2\nline3\n```", graphQL(ws("      ", 1, 11, 1, 17), ws("\n", 1, 17, 2, 1))},
		{"```\n```", mlText(ews(1, 4), ws("\n", 1, 4, 2, 1), tmpl(0, si(2, 1, 2, 1)))},
		{"```\r\n```", mlText(ews(1, 4), ws("\r\n", 1, 4, 2, 1), tmpl(0, si(2, 1, 2, 1)))},
		{"```\n\n```", mlText(ews(1, 4), ws("\n", 1, 4, 2, 1), strTmpl("\n", si(2, 1, 3, 1)))},
		{"```\n\r\n````", mlText(ews(1, 4), ws("\n", 1, 4, 2, 1), strTmpl("\r\n", si(2, 1, 3, 1)))},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(multilineString(r)), tt.want)
	}
}

func TestMultilineStringTextWithVariablesError(t *testing.T) {
	r := reader.New("```\nline1\n{{bar\n```")
	_, err := multilineString(r)
	assertError(t, err, pos(4, 1), false, ParseErrorExpecting{Value: "}}"})
}

func TestMultilineStringFailed(t *testing.T) {
	for _, input := range []string{
		"```hexaaa\nline1\nline2\nline3\n```",
		"```aaa\nline1\nline2\nline3\n```",
	} {
		r := reader.New(input)
		if _, err := multilineString(r); err == nil {
			t.Errorf("multilineString(%q) succeeded, want error", input)
		}
	}
}

func TestMultilineStringHelloError(t *testing.T) {
	r := reader.New("```Hello World!```")
	_, err := multilineString(r)
	assertError(t, err, pos(1, 4), false, ParseErrorMultilineLanguageHint{Value: "Hello World!"})
}

func TestMultilineStringError(t *testing.T) {
	r := reader.New("xxx")
	_, err := multilineString(r)
	assertError(t, err, pos(1, 1), true, ParseErrorExpecting{Value: "```"})

	r = reader.New("```\nxxx")
	_, err = multilineString(r)
	assertError(t, err, pos(2, 4), false, ParseErrorExpecting{Value: "```"})

	r = reader.New("```xxx")
	_, err = multilineString(r)
	assertError(t, err, pos(1, 4), false, ParseErrorMultilineLanguageHint{Value: "xxx"})
}

func TestMultilineStringValue(t *testing.T) {
	r := reader.New("```")
	assertEqual(t, must(multilineStringValue(true, r)), tmpl(0, si(1, 1, 1, 1)))
	assertCursor(t, r, 3)

	r = reader.New("hello```")
	assertEqual(t, must(multilineStringValue(true, r)), strTmpl("hello", si(1, 1, 1, 6)))
	assertCursor(t, r, 8)
}

const graphQLHuman = "```graphql\nquery Human($name: String!) {\n  human(name: $name) {\n    name\n    height(unit: FOOT)\n}\n\n"

func TestMultilineStringGraphQLWithVariables(t *testing.T) {
	r := reader.New(graphQLHuman + "variables {\n  \"name\": \"Han Solo\"\n}\n```")
	query := "query Human($name: String!) {\n  human(name: $name) {\n    name\n    height(unit: FOOT)\n}\n\n"
	assertEqual(t, must(multilineString(r)), ast.MultilineString{
		Space:   ews(1, 11),
		Newline: ws("\n", 1, 11, 2, 1),
		Kind: ast.MultilineStringGraphQL{Value: ast.GraphQL{
			Value: strTmpl(query, si(2, 1, 8, 1)),
			Variables: &ast.GraphQLVariables{
				Space: ws(" ", 8, 10, 8, 11),
				Value: ast.JSONObject{
					Space0: "\n  ",
					Elements: []ast.JSONObjectElement{{
						Name:   tmpl('"', si(9, 4, 9, 8), str("name")),
						Space2: " ",
						Value:  ast.JSONString{Value: tmpl('"', si(9, 12, 9, 20), str("Han Solo"))},
						Space3: "\n",
					}},
				},
				Whitespace: ws("\n", 10, 2, 11, 1),
			},
		}},
	})
}

func TestMultilineStringGraphQLWithVariablesError(t *testing.T) {
	r := reader.New(graphQLHuman + "variables\n```")
	_, err := multilineString(r)
	assertError(t, err, pos(8, 10), false, ParseErrorGraphQLVariables{})
}

func TestMultilineStringGraphQLWithVariablesNotAnObject(t *testing.T) {
	r := reader.New(graphQLHuman + "variables [\n  \"one\",\n  \"two\",\n  \"three\"\n]\n```")
	_, err := multilineString(r)
	assertError(t, err, pos(8, 11), false, ParseErrorGraphQLVariables{})
}
