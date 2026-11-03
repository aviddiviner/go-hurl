package runner

import (
	"strconv"

	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

// HTTPResponse is the payload of ValueHTTPResponse.
type HTTPResponse struct {
	location *httpclient.Url
	status   uint32
}

// NewHTTPResponse returns an HTTP response, given an optional redirection
// location and a status code.
func NewHTTPResponse(location *httpclient.Url, status uint32) HTTPResponse {
	return HTTPResponse{location: location, status: status}
}

// Location returns the absolute redirection URL, or nil.
func (r HTTPResponse) Location() *httpclient.Url { return r.location }

// Status returns the HTTP status code.
func (r HTTPResponse) Status() uint32 { return r.status }

func (r HTTPResponse) String() string {
	status := strconv.FormatUint(uint64(r.status), 10)
	if r.location == nil {
		return "Response(location=None, status=" + status + ")"
	}
	return "Response(location=" + r.location.String() + ", status=" + status + ")"
}
