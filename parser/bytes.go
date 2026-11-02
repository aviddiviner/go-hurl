package parser

import (
	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func bytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	return choice([]func(*reader.Reader) (ast.Bytes, *ParseError){
		multilineStringBytes,
		stringBytes,
		jsonBytes,
		xmlBytes,
		base64Bytes,
		hexBytes,
		fileBytes,
	}, r)
}

func xmlBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := xmlParse(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesXML{Value: v}, nil
}

func jsonBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := parseJSON(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesJSON{Value: v}, nil
}

func fileBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := file(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesFile{Value: v}, nil
}

func base64Bytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := base64(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesBase64{Value: v}, nil
}

func hexBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := hex(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesHex{Value: v}, nil
}

func multilineStringBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := multilineString(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesMultilineString{Value: v}, nil
}

func stringBytes(r *reader.Reader) (ast.Bytes, *ParseError) {
	v, err := backtickTemplate(r)
	if err != nil {
		return nil, err
	}
	return ast.BytesOnelineString{Value: v}, nil
}
