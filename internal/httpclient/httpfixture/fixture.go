// Package httpfixture provides sample requests and responses for tests, a
// port of hurl/src/http/tests/mod.rs.
package httpfixture

import (
	"github.com/aviddiviner/go-hurl/internal/httpclient"
)

func mustUrl(s string) httpclient.Url {
	u, err := httpclient.ParseUrl(s)
	if err != nil {
		panic(err)
	}
	return u
}

func defaultResponse() httpclient.Response {
	return httpclient.Response{
		Version: httpclient.HttpVersionHttp10,
		Status:  200,
		Url:     mustUrl("http://localhost"),
	}
}

func HelloHttpRequest() httpclient.RequestSpec {
	r := httpclient.NewRequestSpec()
	r.Url = mustUrl("http://localhost:8000/hello")
	return r
}

func JsonHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Body = []byte(`
{
  "success":false,
  "errors": [
    { "id": "error1"},
    {"id": "error2"}
  ],
  "duration": 1.5
}
`)
	return r
}

func htmlUtf8Headers() httpclient.HeaderVec {
	return httpclient.NewHeaderVec(
		httpclient.NewHeader("Content-Type", "text/html; charset=utf-8"),
		httpclient.NewHeader("Content-Length", "12"),
	)
}

func XmlTwoUsersHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Headers = htmlUtf8Headers()
	r.Body = []byte(`
<?xml version="1.0"?>
<users>
  <user id="1">Bob</user>
  <user id="2">Bill</user>
</users>
`)
	return r
}

func XmlThreeUsersHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Headers = htmlUtf8Headers()
	r.Body = []byte(`
<?xml version="1.0"?>
<users>
  <user id="1">Bob</user>
  <user id="2">Bill</user>
  <user id="3">Bruce</user>
</users>
`)
	return r
}

func HelloHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Headers = htmlUtf8Headers()
	r.Body = []byte("Hello World!")
	return r
}

func BytesHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Headers = httpclient.NewHeaderVec(
		httpclient.NewHeader("Content-Type", "application/octet-stream"),
		httpclient.NewHeader("Content-Length", "1"),
	)
	r.Body = []byte{255}
	return r
}

func HtmlHttpResponse() httpclient.Response {
	r := defaultResponse()
	r.Headers = httpclient.NewHeaderVec(httpclient.NewHeader("Content-Type", "application/octet-stream"))
	r.Body = []byte(`<html><head><meta charset="UTF-8"></head><body><br></body></html>`)
	return r
}

func QueryHttpRequest() httpclient.RequestSpec {
	r := httpclient.NewRequestSpec()
	r.Url = mustUrl("http://localhost:8000/querystring-params")
	r.Querystring = []httpclient.Param{
		httpclient.NewParam("param1", "value1"),
		httpclient.NewParam("param2", "a b"),
	}
	return r
}

func CustomHttpRequest() httpclient.RequestSpec {
	r := httpclient.NewRequestSpec()
	r.Url = mustUrl("http://localhost/custom")
	r.Headers = httpclient.NewHeaderVec(
		httpclient.NewHeader("User-Agent", "iPhone"),
		httpclient.NewHeader("Foo", "Bar"),
	)
	r.Cookies = []httpclient.RequestCookie{
		{Name: "theme", Value: "light"},
		{Name: "sessionToken", Value: "abc123"},
	}
	return r
}
