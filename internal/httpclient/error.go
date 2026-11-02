package httpclient

import "fmt"

// HttpError is an error of the HTTP client. Variants are the HttpError*
// types.
type HttpError interface {
	error
	httpError()
	// Description is the short description of the error, used as title
	// when rendering a runner error.
	Description() string
	// Message is the detailed message of the error.
	Message() string
}

type HttpErrorCouldNotParseCookieExpires struct{ Value string }

type HttpErrorCouldNotParseResponse struct{}

type HttpErrorCouldNotUncompressResponse struct{ Detail string }

type HttpErrorInvalidCharset struct{ Charset string }

type HttpErrorInvalidDecoding struct{ Charset string }

// HttpErrorLibcurl is a transfer error. go-hurl maps Go transport errors
// onto libcurl error codes and messages, so errors read the same as Hurl's.
type HttpErrorLibcurl struct {
	Code   int
	Detail string
}

type HttpErrorLibcurlUnknownOption struct {
	Option         string
	MinimumVersion string
}

type HttpErrorNoPrimaryIp struct{}

type HttpErrorTooManyRedirect struct{}

type HttpErrorUnsupportedContentEncoding struct{ Detail string }

type HttpErrorUnsupportedHttpVersion struct{ Version RequestedHttpVersion }

// HttpErrorInvalidUrl is an invalid request URL.
type HttpErrorInvalidUrl struct {
	Url    string
	Reason string
}

// HttpErrorAllowedResponseSizeExceeded is raised when the maximum response
// size has been exceeded.
type HttpErrorAllowedResponseSizeExceeded struct{ MaxSize uint64 }

// HttpErrorUnsupportedFeature is a Hurl feature that go-hurl doesn't
// support (NTLM, Negotiate, AWS SigV4, digest, encrypted client keys...).
type HttpErrorUnsupportedFeature struct{ Feature string }

func (HttpErrorCouldNotParseCookieExpires) httpError()  {}
func (HttpErrorCouldNotParseResponse) httpError()       {}
func (HttpErrorCouldNotUncompressResponse) httpError()  {}
func (HttpErrorInvalidCharset) httpError()              {}
func (HttpErrorInvalidDecoding) httpError()             {}
func (HttpErrorLibcurl) httpError()                     {}
func (HttpErrorLibcurlUnknownOption) httpError()        {}
func (HttpErrorNoPrimaryIp) httpError()                 {}
func (HttpErrorTooManyRedirect) httpError()             {}
func (HttpErrorUnsupportedContentEncoding) httpError()  {}
func (HttpErrorUnsupportedHttpVersion) httpError()      {}
func (HttpErrorInvalidUrl) httpError()                  {}
func (HttpErrorAllowedResponseSizeExceeded) httpError() {}
func (HttpErrorUnsupportedFeature) httpError()          {}

const descHttpConnection = "HTTP connection"

func (HttpErrorCouldNotParseCookieExpires) Description() string  { return descHttpConnection }
func (HttpErrorCouldNotParseResponse) Description() string       { return descHttpConnection }
func (HttpErrorCouldNotUncompressResponse) Description() string  { return "Decompression error" }
func (HttpErrorInvalidCharset) Description() string              { return "Invalid charset" }
func (HttpErrorInvalidDecoding) Description() string             { return "Invalid decoding" }
func (HttpErrorLibcurl) Description() string                     { return descHttpConnection }
func (HttpErrorLibcurlUnknownOption) Description() string        { return descHttpConnection }
func (HttpErrorNoPrimaryIp) Description() string                 { return descHttpConnection }
func (HttpErrorTooManyRedirect) Description() string             { return descHttpConnection }
func (HttpErrorUnsupportedContentEncoding) Description() string  { return "Decompression error" }
func (HttpErrorUnsupportedHttpVersion) Description() string      { return "Unsupported HTTP version" }
func (HttpErrorInvalidUrl) Description() string                  { return "Invalid URL" }
func (HttpErrorAllowedResponseSizeExceeded) Description() string { return descHttpConnection }
func (HttpErrorUnsupportedFeature) Description() string          { return "Unsupported feature" }

func (e HttpErrorCouldNotParseCookieExpires) Message() string {
	return fmt.Sprintf("could not parse Cookie Expires attribute value <%s>", e.Value)
}
func (HttpErrorCouldNotParseResponse) Message() string { return "could not parse Response" }
func (e HttpErrorCouldNotUncompressResponse) Message() string {
	return "could not uncompress response with " + e.Detail
}
func (e HttpErrorInvalidCharset) Message() string {
	return fmt.Sprintf("the charset '%s' is not valid", e.Charset)
}
func (e HttpErrorInvalidDecoding) Message() string {
	return fmt.Sprintf("could not decode response body with charset '%s'", e.Charset)
}
func (e HttpErrorLibcurl) Message() string {
	return fmt.Sprintf("(%d) %s", e.Code, e.Detail)
}
func (e HttpErrorLibcurlUnknownOption) Message() string {
	return fmt.Sprintf("Option %s requires libcurl version %s or higher", e.Option, e.MinimumVersion)
}
func (HttpErrorNoPrimaryIp) Message() string     { return "No primary IP found in response" }
func (HttpErrorTooManyRedirect) Message() string { return "too many redirect" }
func (e HttpErrorUnsupportedContentEncoding) Message() string {
	return fmt.Sprintf("compression %s is not supported", e.Detail)
}

// Message differs from Hurl ("... is not supported, check --version"): Go's
// net/http can't do HTTP/1.0 or HTTP/3 at all, so we say so explicitly.
func (e HttpErrorUnsupportedHttpVersion) Message() string {
	return fmt.Sprintf("%s is not supported by go-hurl", e.Version)
}
func (e HttpErrorInvalidUrl) Message() string {
	return fmt.Sprintf("invalid URL <%s> (%s)", e.Url, e.Reason)
}
func (e HttpErrorAllowedResponseSizeExceeded) Message() string {
	return fmt.Sprintf("exceeded the maximum allowed file size (%d bytes)", e.MaxSize)
}
func (e HttpErrorUnsupportedFeature) Message() string {
	return e.Feature + " is not supported by go-hurl"
}

func (e HttpErrorCouldNotParseCookieExpires) Error() string  { return e.Message() }
func (e HttpErrorCouldNotParseResponse) Error() string       { return e.Message() }
func (e HttpErrorCouldNotUncompressResponse) Error() string  { return e.Message() }
func (e HttpErrorInvalidCharset) Error() string              { return e.Message() }
func (e HttpErrorInvalidDecoding) Error() string             { return e.Message() }
func (e HttpErrorLibcurl) Error() string                     { return e.Message() }
func (e HttpErrorLibcurlUnknownOption) Error() string        { return e.Message() }
func (e HttpErrorNoPrimaryIp) Error() string                 { return e.Message() }
func (e HttpErrorTooManyRedirect) Error() string             { return e.Message() }
func (e HttpErrorUnsupportedContentEncoding) Error() string  { return e.Message() }
func (e HttpErrorUnsupportedHttpVersion) Error() string      { return e.Message() }
func (e HttpErrorInvalidUrl) Error() string                  { return e.Message() }
func (e HttpErrorAllowedResponseSizeExceeded) Error() string { return e.Message() }
func (e HttpErrorUnsupportedFeature) Error() string          { return e.Message() }

func httpErrorFromUrlError(e *UrlError) HttpError {
	return HttpErrorInvalidUrl{Url: e.Url, Reason: e.Reason}
}
