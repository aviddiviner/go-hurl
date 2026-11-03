package runner

import "github.com/aviddiviner/go-hurl/internal/xpath"

// xpathEval evaluates an XPath expression against a document, mapping the
// result to a Value (numbers are always floats). The error is an
// xpath.Error.
func xpathEval(doc *xpath.Document, expr string) (Value, error) {
	v, err := doc.EvalXPath(expr)
	if err != nil {
		return nil, err
	}
	switch v := v.(type) {
	case xpath.Bool:
		return ValueBool{bool(v)}, nil
	case xpath.Number:
		return ValueNumber{NumberFloat{float64(v)}}, nil
	case xpath.String:
		return ValueString{string(v)}, nil
	case xpath.Nodeset:
		return ValueNodeset{int(v)}, nil
	}
	return nil, xpath.ErrUnsupported
}
