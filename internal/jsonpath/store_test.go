package jsonpath

import "testing"

const storeJSON = `{
  "book": [
    {
      "category": "reference",
      "author": "Nigel Rees",
      "title": "Sayings of the Century",
      "price": 8.95
    },
    {
      "category": "fiction",
      "author": "Evelyn Waugh",
      "title": "Sword of Honour",
      "price": 12.99
    },
    {
      "category": "fiction",
      "author": "Herman Melville",
      "title": "Moby Dick",
      "isbn": "0-553-21311-3",
      "price": 8.99
    },
    {
      "category": "fiction",
      "author": "J. R. R. Tolkien",
      "title": "The Lord of the Rings",
      "isbn": "0-395-19395-8",
      "price": 22.99
    }
  ],
  "bicycle": {
    "color": "red",
    "price": 19.95
  }
}`

func storeEval(t *testing.T, query string) NodeList {
	t.Helper()
	q, err := Parse(query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return q.Eval(j(storeJSON))
}

func TestStoreNonEmptyResults(t *testing.T) {
	store := j(storeJSON).(map[string]any)
	book := store["book"].([]any)
	book0, book1, book2, book3 := book[0], book[1], book[2], book[3]
	bicycle := store["bicycle"]

	tests := []struct {
		query string
		want  NodeList
	}{
		{"$", NodeList{store}},
		{"$['book']", NodeList{book}},
		{"$.book", NodeList{book}},
		{"$.book[0]", NodeList{book0}},
		{"$.book[0].author", js(`"Nigel Rees"`)},
		{"$.*", NodeList{bicycle, book}},
		{"$.book[:2]", NodeList{book0, book1}},
		{"$.book[0,1]", NodeList{book0, book1}},
		{"$.book[?@.isbn]", NodeList{book2, book3}},
		{"$.book[?@.price<10]", NodeList{book0, book2}},
		{"$.book[*].author", js(`"Nigel Rees"`, `"Evelyn Waugh"`, `"Herman Melville"`, `"J. R. R. Tolkien"`)},
		{"$..author", js(`"Nigel Rees"`, `"Evelyn Waugh"`, `"Herman Melville"`, `"J. R. R. Tolkien"`)},
		{"$..book[2]", NodeList{book2}},
		{"$..book[2].title", js(`"Moby Dick"`)},
		{"$..book[-1]", NodeList{book3}},
		{"$..book[-1:].title", js(`"The Lord of the Rings"`)},
		{"$..book[:2]", NodeList{book0, book1}},
		{"$..book[0,1]", NodeList{book0, book1}},
		{"$..book[-4, -3]", NodeList{book0, book1}},
		{"$.bicycle.price", js(`19.95`)},
	}
	for _, tt := range tests {
		assertNodes(t, storeEval(t, tt.query), tt.want)
	}

	counts := map[string]int{
		"$..book[?(@.author != 'Charles Dickens')]": 4,
		"$..book[?(@.author != 'Nigel Rees')]":      3,
		"$..price":                                  5,
	}
	for query, want := range counts {
		if got := len(storeEval(t, query)); got != want {
			t.Errorf("%s: got %d nodes, want %d", query, got, want)
		}
	}
}

func TestStoreEmptyNodelist(t *testing.T) {
	for _, query := range []string{"$.book[4]", "$.book[-5]", "$[0]", "$.book[*].not_exist"} {
		if got := storeEval(t, query); got == nil || len(got) != 0 {
			t.Errorf("%s: got %v", query, got)
		}
	}
}
