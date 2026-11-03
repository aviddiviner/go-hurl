package runner

import (
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// evalMultipartParam evaluates a multipart param, like:
//
//	[Multipart]
//	field1: value1
//	field2: file,example.txt;
//	field3: file,example.zip; application/zip
func evalMultipartParam(param ast.MultipartParam, variables *VariableSet, contextDir util.ContextDir) (httpclient.MultipartParam, *RunnerError) {
	switch p := param.(type) {
	case ast.MultipartParamParam:
		name, err := evalTemplate(p.Value.Key, variables)
		if err != nil {
			return nil, err
		}
		value, err := evalTemplate(p.Value.Value, variables)
		if err != nil {
			return nil, err
		}
		return httpclient.NewParam(name, value), nil
	case ast.MultipartParamFilenameParam:
		fp, err := evalFileParam(p.Value, contextDir, variables)
		if err != nil {
			return nil, err
		}
		return fp, nil
	}
	panic("unknown multipart param")
}

func evalFileParam(param ast.FilenameParam, contextDir util.ContextDir, variables *VariableSet) (httpclient.FileParam, *RunnerError) {
	name, err := evalTemplate(param.Key, variables)
	if err != nil {
		return httpclient.FileParam{}, err
	}
	filename, err := evalTemplate(param.Value.Filename, variables)
	if err != nil {
		return httpclient.FileParam{}, err
	}
	data, err := evalFile(param.Value.Filename, variables, contextDir)
	if err != nil {
		return httpclient.FileParam{}, err
	}
	contentType, err := evalContentType(param.Value, variables)
	if err != nil {
		return httpclient.FileParam{}, err
	}
	return httpclient.FileParam{Name: name, Filename: filename, Data: data, ContentType: contentType}, nil
}

func evalContentType(value ast.FilenameValue, variables *VariableSet) (string, *RunnerError) {
	if value.ContentType != nil {
		return evalTemplate(*value.ContentType, variables)
	}
	filename, err := evalTemplate(value.Filename, variables)
	if err != nil {
		return "", err
	}
	switch fileExtension(filename) {
	case "gif":
		return "image/gif", nil
	case "jpg", "jpeg":
		return "image/jpeg", nil
	case "png":
		return "image/png", nil
	case "svg":
		return "image/svg+xml", nil
	case "txt":
		return "text/plain", nil
	case "htm", "html":
		return "text/html", nil
	case "pdf":
		return "application/pdf", nil
	case "xml":
		return "application/xml", nil
	}
	return "application/octet-stream", nil
}

// fileExtension is Rust's Path::extension: the part of the file name after
// its last dot, none for a file name starting with its only dot.
func fileExtension(path string) string {
	path = strings.TrimRight(path, "/")
	name := path[strings.LastIndexAny(path, "/")+1:]
	if i := strings.LastIndex(name, "."); i > 0 {
		return name[i+1:]
	}
	return ""
}
