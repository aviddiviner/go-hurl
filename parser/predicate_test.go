package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestPredicateNot(t *testing.T) {
	r := reader.New("notXX")
	not, space := predicateNot(r)
	assertEqual(t, not, false)
	assertEqual(t, space, ews(1, 1))
	assertEqual(t, r.Cursor().Pos, pos(1, 1))

	r = reader.New("not XX")
	not, space = predicateNot(r)
	assertEqual(t, not, true)
	assertEqual(t, space, ws(" ", 1, 4, 1, 5))
	assertEqual(t, r.Cursor().Pos, pos(1, 5))
}

func TestPredicate(t *testing.T) {
	r := reader.New("not == true")
	assertEqual(t, must(predicate(r)), ast.Predicate{
		Not:    true,
		Space0: ws(" ", 1, 4, 1, 5),
		PredicateFunc: ast.PredicateFunc{
			SourceInfo: si(1, 5, 1, 12),
			Value:      ast.PredicateFuncEqual{Space0: ws(" ", 1, 7, 1, 8), Value: ast.PredicateBool{Value: true}},
		},
	})
}

func TestPredicateFunc(t *testing.T) {
	r := reader.New("tata == 1")
	_, err := predicateFunc(r)
	assertError(t, err, pos(1, 1), false, ParseErrorPredicate{})
}

func TestEqualPredicate(t *testing.T) {
	tests := []struct {
		input string
		want  ast.PredicateFuncValue
	}{
		{"==  true", ast.PredicateFuncEqual{Space0: ws("  ", 1, 3, 1, 5), Value: ast.PredicateBool{Value: true}}},
		{"== 1.1", ast.PredicateFuncEqual{
			Space0: ws(" ", 1, 3, 1, 4),
			Value:  ast.PredicateNumber{Value: ast.NumberFloat{Value: ast.Float{Value: 1.1, Source: "1.1"}}},
		}},
		{"== 2", ast.PredicateFuncEqual{
			Space0: ws(" ", 1, 3, 1, 4),
			Value:  ast.PredicateNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 2, Source: "2"}}},
		}},
		{`== "Bob"`, ast.PredicateFuncEqual{
			Space0: ws(" ", 1, 3, 1, 4),
			Value:  ast.PredicateString{Value: tmpl('"', si(1, 4, 1, 9), str("Bob"))},
		}},
		{"== {{count}}", ast.PredicateFuncEqual{
			Space0: ws(" ", 1, 3, 1, 4),
			Value:  ast.PredicatePlaceholder{Value: ph("count", 1, 6)},
		}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(equalPredicate(r)), tt.want)
	}
}

func TestStartWithPredicate(t *testing.T) {
	r := reader.New("startsWith 2")
	_, err := startWithPredicate(r)
	assertError(t, err, pos(1, 12), false, ParseErrorPredicateValue{})
}

func TestDatePredicate(t *testing.T) {
	r := reader.New("isDate")
	assertEqual(t, must(datePredicate(r)), ast.PredicateFuncValue(ast.PredicateFuncIsDate{}))
}

func TestPredicateValue(t *testing.T) {
	tests := []struct {
		input string
		want  ast.PredicateValue
	}{
		{"true", ast.PredicateBool{Value: true}},
		{"1", ast.PredicateNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 1, Source: "1"}}}},
		{"1.1", ast.PredicateNumber{Value: ast.NumberFloat{Value: ast.Float{Value: 1.1, Source: "1.1"}}}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(predicateValue(r)), tt.want)
	}
}

func TestPredicateValueError(t *testing.T) {
	r := reader.New("xx")
	_, err := predicateValue(r)
	assertError(t, err, pos(1, 1), false, ParseErrorPredicateValue{})

	r = reader.New(`"not_found`)
	_, err = predicateValue(r)
	assertError(t, err, pos(1, 11), false, ParseErrorExpecting{Value: `"`})
}
