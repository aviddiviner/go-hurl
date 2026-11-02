package parser

import (
	"os"
	"strings"
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func TestXMLBruteForceErrors(t *testing.T) {
	tests := []struct {
		input       string
		pos         reader.Pos
		recoverable bool
	}{
		{"", pos(1, 1), true},
		{"x", pos(1, 1), true},
		{"<<", pos(1, 2), false},
		{"<users><user /></users", pos(1, 22), false},
		{"<users aa><user /></users", pos(1, 10), false},
	}
	for _, tt := range tests {
		r := reader.New(tt.input)
		_, err := xmlParse(r)
		assertError(t, err, tt.pos, tt.recoverable, ParseErrorXML{})
	}
}

func TestXMLBruteForceOK(t *testing.T) {
	r := reader.New("<users><user /></users>")
	assertEqual(t, must(xmlParse(r)), "<users><user /></users>")
	assertCursor(t, r, 23)

	r = reader.New("<users><user /></users>xx")
	assertEqual(t, must(xmlParse(r)), "<users><user /></users>")
	assertCursor(t, r, 23)
	assertEqual(t, r.PeekN(2), "xx")

	r = reader.New(`<?xml version="1.0"?><users/>xxx`)
	assertEqual(t, must(xmlParse(r)), `<?xml version="1.0"?><users/>`)
	assertCursor(t, r, 29)
}

func TestXMLSoapUTF8(t *testing.T) {
	xml := `<?xml version='1.0' encoding='UTF-8'?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
    <soapenv:Body>
        <ns31:UploadInboundResponseElement xmlns:ns31="http://www.example.com/schema/xyzWS">
            <ns31:UploadInboundResult>&lt;?xml version="1.0" encoding="UTF-8" ?>&lt;ATKCST>&lt;Head>&lt;FunCode>9000&lt;/FunCode>&lt;Remark>接收数据成功&lt;/Remark>&lt;/Head>&lt;/ATKCST></ns31:UploadInboundResult>
        </ns31:UploadInboundResponseElement>
    </soapenv:Body>
</soapenv:Envelope>`

	for _, input := range []string{
		xml,                  // a valid complete XML
		xml + " xx xx xx xx", // with data padding
		xml + xml,            // two consecutive XML
	} {
		r := reader.New(input)
		assertEqual(t, must(xmlParse(r)), xml)
		assertCursor(t, r, 520)
	}
}

func TestXMLBooksWithEntryResponseStart(t *testing.T) {
	data, err := os.ReadFile("testdata/books.xml")
	if err != nil {
		t.Fatal(err)
	}
	xml := strings.TrimSuffix(string(data), "\n")
	r := reader.New(xml + "\nHTTP 200")
	assertEqual(t, must(xmlParse(r)), xml)
	assertCursor(t, r, 4411)
}
