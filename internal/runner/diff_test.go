package runner

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/internal/text"
)

// TestDiffSimilar checks diff against reference outputs generated with the
// Rust similar crate 3.2.1 (the version used by Hurl 8.1.0), on random
// inputs over a small vocabulary of lines (see testdata/similar_diffs.rs).
func TestDiffSimilar(t *testing.T) {
	f, err := os.Open("testdata/similar_diffs.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(gz)
	scanner.Buffer(nil, 1<<20)
	type hunk struct {
		Content    string `json:"content"`
		Start      int    `json:"start"`
		SourceLine int    `json:"source_line"`
	}
	count, failures := 0, 0
	for scanner.Scan() {
		var c struct {
			Old, New string
			Hunks    []hunk
		}
		if err := json.Unmarshal(scanner.Bytes(), &c); err != nil {
			t.Fatal(err)
		}
		count++
		var got []hunk
		for _, h := range diff(c.Old, c.New) {
			got = append(got, hunk{h.Content.Render(text.FormatPlain), h.Start, h.SourceLine})
		}
		if len(got) != len(c.Hunks) || (len(got) > 0 && !reflect.DeepEqual(got, c.Hunks)) {
			if failures++; failures <= 3 {
				t.Errorf("diff(%q, %q):\n got %+v\nwant %+v", c.Old, c.New, got, c.Hunks)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 || failures > 0 {
		t.Errorf("%d/%d cases differ", failures, count)
	}
}

const diffOld = `{
  "first_name": "John",
  "last_name": "Smith",
  "is_alive": true,
  "age": 27,
  "address": {
    "street_address": "21 2nd Street",
    "city": "New York",
    "state": "NY",
    "postal_code": "10021-3100"
  },
  "phone_numbers": [
    {
      "type": "home",
      "number": "212 555-1234"
    },
    {
      "type": "office",
      "number": "646 555-4567"
    }
  ],
  "children": [
    "Catherine",
    "Thomas",
    "Trevor"
  ],
  "spouse": null
}
`

func styled(parts ...any) text.StyledString {
	var s text.StyledString
	for i := 0; i < len(parts); i += 2 {
		s.PushWith(parts[i].(string), parts[i+1].(text.Style))
	}
	return s
}

func red() text.Style   { return text.NewStyle().Red() }
func green() text.Style { return text.NewStyle().Green() }

func TestDiff(t *testing.T) {
	tests := []struct {
		name       string
		new        string
		hunk       int
		content    text.StyledString
		start      int
		sourceLine int
	}{
		{"change line1", strings.Replace(diffOld, "{", "[", 1), 0, styled("-{\n", red(), "+[\n", green()), 0, 0},
		{"change line2", strings.Replace(diffOld, `"John"`, `"Bob"`, 1), 0, styled("-  \"first_name\": \"John\",\n", red(), "+  \"first_name\": \"Bob\",\n", green()), 1, 1},
		{"change line3", strings.Replace(diffOld, `"Smith"`, `"Smiths"`, 1), 0, styled("-  \"last_name\": \"Smith\",\n", red(), "+  \"last_name\": \"Smiths\",\n", green()), 2, 2},
		{"change line4", strings.Replace(diffOld, "27", "28", 1), 0, styled("-  \"age\": 27,\n", red(), "+  \"age\": 28,\n", green()), 4, 4},
		{"delete line3", strings.Replace(diffOld, "  \"last_name\": \"Smith\",\n", "", 1), 0, styled("-  \"last_name\": \"Smith\",\n", red()), 2, 2},
		{"add line3", strings.Replace(diffOld, "  \"last_name\"", "  \"middle_name\": \"Bob\",\n  \"last_name\"", 1), 0, styled("+  \"middle_name\": \"Bob\",\n", green()), 2, 1},
		{"change line4 line24 (1)", strings.Replace(strings.Replace(diffOld, "27", "28", 1), `"Trevor"`, `"Bob"`, 1), 0, styled("-  \"age\": 27,\n", red(), "+  \"age\": 28,\n", green()), 4, 4},
		{"change line4 line24 (2)", strings.Replace(strings.Replace(diffOld, "27", "28", 1), `"Trevor"`, `"Bob"`, 1), 1, styled("-    \"Trevor\"\n", red(), "+    \"Bob\"\n", green()), 24, 24},
	}
	for _, tt := range tests {
		hunks := diff(diffOld, tt.new)
		if len(hunks) <= tt.hunk {
			t.Errorf("%s: %d hunks", tt.name, len(hunks))
			continue
		}
		h := hunks[tt.hunk]
		if !reflect.DeepEqual(h.Content, tt.content) || h.Start != tt.start || h.SourceLine != tt.sourceLine {
			t.Errorf("%s: got %q (%d, %d); want %q (%d, %d)", tt.name, h.Content.Render(text.FormatPlain), h.Start, h.SourceLine, tt.content.Render(text.FormatPlain), tt.start, tt.sourceLine)
		}
	}
}

func TestDiffAddNewLine(t *testing.T) {
	hunks := diff("<p>Hello</p>\n", "<p>Hello</p>\n\n")
	h := hunks[0]
	if !reflect.DeepEqual(h.Content, styled("+\n", green())) || h.Start != 1 || h.SourceLine != 0 {
		t.Errorf("got %#v", h)
	}
}

func TestDiffMissingNewline(t *testing.T) {
	hunks := diff("a\nb", "a\nc")
	if len(hunks) != 1 || hunks[0].Content.Render(text.FormatPlain) != "-b\n+c\n" {
		t.Errorf("got %#v", hunks)
	}
}

func TestTokenizeLines(t *testing.T) {
	got := tokenizeLines("a\nb\r\nc\rd")
	want := []string{"a\n", "b\r\n", "c\r", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q", got)
	}
}
