package parser

import (
	"testing"

	"github.com/aviddiviner/go-hurl/reader"
)

func TestFilenameWithPassword(t *testing.T) {
	r := reader.New(`file\:123:pwd\:\#:`)
	assertEqual(t, must(filenamePassword(r)), tmpl(0, si(1, 1, 1, 19), strSrc(`file\:123:pwd\:#:`, `file\:123:pwd\:\#:`)))
	assertCursor(t, r, 18)

	r = reader.New("/etc/client-cert.pem #foo")
	assertEqual(t, must(filenamePassword(r)), strTmpl("/etc/client-cert.pem", si(1, 1, 1, 21)))
	assertCursor(t, r, 20)
}
