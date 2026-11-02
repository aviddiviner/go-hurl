package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
)

func TestTemplatizeEmptyString(t *testing.T) {
	elements := must(Templatize(EncodedString{SourceInfo: si(1, 1, 1, 1)}))
	assertEqual(t, elements, []ast.TemplateElement{})
}

func TestTemplatizeHelloWorld(t *testing.T) {
	// Hi\u0020{{name}}!
	encoded := EncodedString{
		SourceInfo: si(1, 1, 1, 18),
		Chars: []EncodedChar{
			{'H', "H", pos(1, 1)},
			{'i', "i", pos(1, 2)},
			{' ', `\u0020`, pos(1, 3)},
			{'{', "{", pos(1, 9)},
			{'{', "{", pos(1, 10)},
			{'n', "n", pos(1, 11)},
			{'a', "a", pos(1, 12)},
			{'m', "m", pos(1, 13)},
			{'e', "e", pos(1, 14)},
			{'}', "}", pos(1, 15)},
			{'}', "}", pos(1, 16)},
			{'!', "!", pos(1, 17)},
		},
	}
	assertEqual(t, must(Templatize(encoded)), []ast.TemplateElement{
		strSrc("Hi ", `Hi\u0020`),
		phElem("name", 1, 11),
		str("!"),
	})
}

func TestTemplatizeExpressionOnly(t *testing.T) {
	// {{x}}!
	encoded := EncodedString{
		SourceInfo: si(1, 1, 1, 7),
		Chars: []EncodedChar{
			{'{', "{", pos(1, 1)},
			{'{', "{", pos(1, 2)},
			{'x', "x", pos(1, 3)},
			{'}', "}", pos(1, 4)},
			{'}', "}", pos(1, 4)},
		},
	}
	assertEqual(t, must(Templatize(encoded)), []ast.TemplateElement{phElem("x", 1, 3)})
}

func TestTemplatizeError(t *testing.T) {
	// {{x
	encoded := EncodedString{
		SourceInfo: si(1, 1, 1, 4),
		Chars: []EncodedChar{
			{'{', "{", pos(1, 1)},
			{'{', "{", pos(1, 2)},
			{'x', "x", pos(1, 3)},
		},
	}
	_, err := Templatize(encoded)
	assertError(t, err, pos(1, 4), false, ParseErrorExpecting{Value: "}}"})
}

func TestTemplatizeEscapeBracket(t *testing.T) {
	// \{\{ is the valid string "{{"
	encoded := EncodedString{
		SourceInfo: si(1, 1, 1, 4),
		Chars: []EncodedChar{
			{'{', `\{`, pos(1, 1)},
			{'{', `\{`, pos(1, 2)},
		},
	}
	assertEqual(t, must(Templatize(encoded)), []ast.TemplateElement{strSrc("{{", `\{\{`)})
}
