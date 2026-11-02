package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func TestPlaceholder(t *testing.T) {
	r := reader.New("{{ name}}")
	want := ph("name", 1, 4)
	want.Space0 = ws(" ", 1, 3, 1, 4)
	assertEqual(t, must(placeholder(r)), want)

	for _, input := range []string{"{{host>}}", "{{host"} {
		r := reader.New(input)
		_, err := placeholder(r)
		assertError(t, err, pos(1, 7), false, ParseErrorExpecting{Value: "}}"})
	}
}
