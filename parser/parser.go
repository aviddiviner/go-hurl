// Package parser parses Hurl files into the lossless ast, ported from
// hurl_core's parser module. It is a scannerless recursive descent parser
// over a reader.Reader.
package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// ParseHurlFile parses the content of a Hurl file.
func ParseHurlFile(s string) (*ast.HurlFile, *ParseError) {
	r := reader.New(s)
	f, err := hurlFile(r)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ParseJSON parses a JSON value (with Hurl placeholders) from r.
func ParseJSON(r *reader.Reader) (ast.JSONValue, *ParseError) { return parseJSON(r) }

// ParseJSONBoolean parses a JSON boolean literal from r.
func ParseJSONBoolean(r *reader.Reader) (ast.JSONValue, *ParseError) { return booleanValue(r) }

// ParseJSONNull parses a JSON null literal from r.
func ParseJSONNull(r *reader.Reader) (ast.JSONValue, *ParseError) { return nullValue(r) }

// ParseJSONNumber parses a JSON number literal from r.
func ParseJSONNumber(r *reader.Reader) (ast.JSONValue, *ParseError) { return numberValue(r) }

func sourceInfo(start, end reader.Pos) ast.SourceInfo {
	return ast.SourceInfo{Start: start, End: end}
}
