package runner

import "github.com/aviddiviner/go-hurl/internal/xpath"

// bodyCache holds the parsed structured data (XML/JSON) of an HTTP response
// body. It lives for a given request, so that several queries of the same
// type share a single parsed document.
type bodyCache struct {
	xml     *xpath.Document
	json    any
	hasJSON bool
}

func (c *bodyCache) setXML(doc *xpath.Document) { c.xml = doc }

func (c *bodyCache) setJSON(json any) {
	c.json = json
	c.hasJSON = true
}
