package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func TestOptionInsecure(t *testing.T) {
	r := reader.New("insecure: true")
	assertEqual(t, must(entryOption(r)), ast.EntryOption{
		Space0:          ews(1, 1),
		Space1:          ews(1, 9),
		Space2:          ws(" ", 1, 10, 1, 11),
		Kind:            ast.OptionInsecure{Value: ast.BooleanLiteral{Value: true}},
		LineTerminator0: lt(ews(1, 15), ews(1, 15)),
	})
}

func TestOptionInsecureError(t *testing.T) {
	r := reader.New("insecure: error")
	_, err := entryOption(r)
	if err == nil || err.Recoverable {
		t.Fatalf("expected non-recoverable error, got %v", err)
	}
}

func TestOptionNoJSONPathCoercion(t *testing.T) {
	r := reader.New("no-jsonpath-coercion: true")
	assertEqual(t, must(entryOption(r)).Kind, ast.OptionKind(ast.OptionNoJSONPathCoercion{Value: ast.BooleanLiteral{Value: true}}))
}

func TestOptionCACert(t *testing.T) {
	r := reader.New("cacert: /home/foo/cert.pem")
	assertEqual(t, must(entryOption(r)), ast.EntryOption{
		Space0:          ews(1, 1),
		Space1:          ews(1, 7),
		Space2:          ws(" ", 1, 8, 1, 9),
		Kind:            ast.OptionCACertificate{Value: strTmpl("/home/foo/cert.pem", si(1, 9, 1, 27))},
		LineTerminator0: lt(ews(1, 27), ews(1, 27)),
	})
}

func TestOptionCACertError(t *testing.T) {
	r := reader.New("cacert: ###")
	_, err := entryOption(r)
	if err == nil || err.Recoverable {
		t.Fatalf("expected non-recoverable error, got %v", err)
	}
}

func TestOptionCert(t *testing.T) {
	r := reader.New("/etc/client-cert.pem #foo")
	assertEqual(t, must(optionParsers["cert"](r)), ast.OptionKind(ast.OptionClientCert{Value: strTmpl("/etc/client-cert.pem", si(1, 1, 1, 21))}))
}

func TestOptionRetryError(t *testing.T) {
	r := reader.New("retry: ###")
	_, err := entryOption(r)
	assertError(t, err, pos(1, 8), false, ParseErrorExpecting{Value: "integer >= -1"})
}

func TestVariableDefinition(t *testing.T) {
	r := reader.New("a=1")
	assertEqual(t, must(variableDefinition(r)), ast.VariableDefinition{
		SourceInfo: si(1, 1, 1, 4),
		Name:       "a",
		Space0:     ews(1, 2),
		Space1:     ews(1, 3),
		Value:      ast.VariableNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 1, Source: "1"}}},
	})
}

func TestVariableValue(t *testing.T) {
	tests := []struct {
		input string
		want  ast.VariableValue
	}{
		{"null", ast.VariableNull{}},
		{"true", ast.VariableBool{Value: true}},
		{"1", ast.VariableNumber{Value: ast.NumberInteger{Value: ast.I64{Value: 1, Source: "1"}}}},
		{"toto", ast.VariableString{Value: strTmpl("toto", si(1, 1, 1, 5))}},
		{`"123"`, ast.VariableString{Value: tmpl('"', si(1, 1, 1, 6), str("123"))}},
		// A literal prefix followed by more chars is an unquoted string.
		{"11aa", ast.VariableString{Value: strTmpl("11aa", si(1, 1, 1, 5))}},
		{"0.5x", ast.VariableString{Value: strTmpl("0.5x", si(1, 1, 1, 5))}},
		{"true_is_true", ast.VariableString{Value: strTmpl("true_is_true", si(1, 1, 1, 13))}},
		{"nullable", ast.VariableString{Value: strTmpl("nullable", si(1, 1, 1, 9))}},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		assertEqual(t, must(variableValue(r)), tt.want)
	}
}
