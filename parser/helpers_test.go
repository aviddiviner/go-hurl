package parser

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func pos(line, column int) reader.Pos { return reader.Pos{Line: line, Column: column} }

func si(l1, c1, l2, c2 int) ast.SourceInfo { return sourceInfo(pos(l1, c1), pos(l2, c2)) }

func ws(value string, l1, c1, l2, c2 int) ast.Whitespace {
	return ast.Whitespace{Value: value, SourceInfo: si(l1, c1, l2, c2)}
}

// ews is an empty whitespace at line:column.
func ews(line, column int) ast.Whitespace { return ws("", line, column, line, column) }

func strSrc(value, source string) ast.TemplateString {
	return ast.TemplateString{Value: value, Source: source}
}

func tmpl(delimiter rune, s ast.SourceInfo, elements ...ast.TemplateElement) ast.Template {
	return ast.Template{Delimiter: delimiter, Elements: elements, SourceInfo: s}
}

// strTmpl is an undelimited template with a single string element.
func strTmpl(value string, s ast.SourceInfo) ast.Template { return tmpl(0, s, str(value)) }

// ph is a {{name}} placeholder without spaces, the name starting at
// line:column.
func ph(name string, line, column int) ast.Placeholder {
	end := column + len([]rune(name))
	return ast.Placeholder{
		Space0: ews(line, column),
		Expr: ast.Expr{
			SourceInfo: si(line, column, line, end),
			Kind:       ast.ExprVariable{Value: ast.Variable{Name: name, SourceInfo: si(line, column, line, end)}},
		},
		Space1: ews(line, end),
	}
}

func phElem(name string, line, column int) ast.TemplatePlaceholder {
	return ast.TemplatePlaceholder{Value: ph(name, line, column)}
}

// lt is a line terminator without comment.
func lt(space0, newline ast.Whitespace) ast.LineTerminator {
	return ast.LineTerminator{Space0: space0, Newline: newline}
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if path, ok := equal(reflect.ValueOf(got), reflect.ValueOf(want), ""); !ok {
		t.Fatalf("mismatch at %s\n got: %+v\nwant: %+v", path, got, want)
	}
}

func assertCursor(t *testing.T, r *reader.Reader, index int) {
	t.Helper()
	if got := r.Cursor().Index; got != index {
		t.Fatalf("cursor index = %d, want %d", got, index)
	}
}

func assertError(t *testing.T, err *ParseError, p reader.Pos, recoverable bool, kind ParseErrorKind) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
	if err.Pos != p {
		t.Errorf("error pos = %v, want %v (%v)", err.Pos, p, err)
	}
	if err.Recoverable != recoverable {
		t.Errorf("error recoverable = %v, want %v (%v)", err.Recoverable, recoverable, err)
	}
	if kind != nil {
		assertEqual(t, err.Kind, kind)
	}
}

// must returns v, panicking if err is set (for use as must(parse(r))).
func must[T any](v T, err *ParseError) T {
	if err != nil {
		panic("unexpected error: " + err.Error())
	}
	return v
}

var regexpType = reflect.TypeFor[*regexp.Regexp]()

// equal is a deep equality that treats nil and empty slices as equal and
// compares regexps by source. It returns the path of the first difference.
func equal(a, b reflect.Value, path string) (string, bool) {
	if !a.IsValid() || !b.IsValid() {
		return path, a.IsValid() == b.IsValid()
	}
	if a.Type() != b.Type() {
		return fmt.Sprintf("%s (type %s != %s)", path, a.Type(), b.Type()), false
	}
	if a.Type() == regexpType {
		if a.IsNil() || b.IsNil() {
			return path, a.IsNil() == b.IsNil()
		}
		return path, a.Interface().(*regexp.Regexp).String() == b.Interface().(*regexp.Regexp).String()
	}
	switch a.Kind() {
	case reflect.Interface, reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return path, a.IsNil() == b.IsNil()
		}
		return equal(a.Elem(), b.Elem(), path)
	case reflect.Struct:
		for i := range a.NumField() {
			if p, ok := equal(a.Field(i), b.Field(i), path+"."+a.Type().Field(i).Name); !ok {
				return p, false
			}
		}
		return path, true
	case reflect.Slice:
		if a.Len() != b.Len() {
			return fmt.Sprintf("%s (len %d != %d)", path, a.Len(), b.Len()), false
		}
		for i := range a.Len() {
			if p, ok := equal(a.Index(i), b.Index(i), fmt.Sprintf("%s[%d]", path, i)); !ok {
				return p, false
			}
		}
		return path, true
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%s (%v != %v)", path, a.Float(), b.Float()), a.Float() == b.Float()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%s (%v != %v)", path, a.Int(), b.Int()), a.Int() == b.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%s (%v != %v)", path, a.Uint(), b.Uint()), a.Uint() == b.Uint()
	case reflect.String:
		return fmt.Sprintf("%s (%q != %q)", path, a.String(), b.String()), a.String() == b.String()
	case reflect.Bool:
		return fmt.Sprintf("%s (%v != %v)", path, a.Bool(), b.Bool()), a.Bool() == b.Bool()
	}
	if a.CanInterface() {
		return fmt.Sprintf("%s (%v != %v)", path, a.Interface(), b.Interface()), reflect.DeepEqual(a.Interface(), b.Interface())
	}
	return path, true
}
