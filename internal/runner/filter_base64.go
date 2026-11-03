package runner

import (
	"encoding/base64"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
)

var (
	base64Standard   = base64.StdEncoding.Strict()
	base64URLSafeRaw = base64.RawURLEncoding.Strict()
)

// decodeBase64 is a strict decoding (the Go decoder skips \r and \n, Rust's
// doesn't).
func decodeBase64(enc *base64.Encoding, s string) ([]byte, bool) {
	if strings.ContainsAny(s, "\r\n") {
		return nil, false
	}
	b, err := enc.DecodeString(s)
	return b, err == nil
}

// evalBase64Decode decodes a base64 string into bytes.
func evalBase64Decode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "base64 string", si, assert)
	}
	decoded, ok := decodeBase64(base64Standard, s.Value)
	if !ok {
		return nil, invalidInputValue("string is not base64", si, assert)
	}
	return ValueBytes{decoded}, nil
}

// evalBase64Encode encodes bytes into a base64 string.
func evalBase64Encode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	b, ok := value.(ValueBytes)
	if !ok {
		return nil, invalidInputType(value, "bytes", si, assert)
	}
	return ValueString{base64.StdEncoding.EncodeToString(b.Value)}, nil
}

// evalBase64URLSafeDecode decodes a base64 URL safe string (without padding)
// into bytes.
func evalBase64URLSafeDecode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	s, ok := value.(ValueString)
	if !ok {
		return nil, invalidInputType(value, "string", si, assert)
	}
	decoded, ok := decodeBase64(base64URLSafeRaw, s.Value)
	if !ok {
		if trimmed := strings.TrimRight(s.Value, "="); trimmed != s.Value {
			if _, ok := decodeBase64(base64URLSafeRaw, trimmed); ok {
				return nil, invalidInputValue("base64 string contains padding", si, assert)
			}
		}
		return nil, invalidInputValue("string is not base64", si, assert)
	}
	return ValueBytes{decoded}, nil
}

// evalBase64URLSafeEncode encodes bytes into a base64 URL safe string
// (without padding).
func evalBase64URLSafeEncode(value Value, si ast.SourceInfo, assert bool) (Value, *RunnerError) {
	b, ok := value.(ValueBytes)
	if !ok {
		return nil, invalidInputType(value, "bytes", si, assert)
	}
	return ValueString{base64.RawURLEncoding.EncodeToString(b.Value)}, nil
}
