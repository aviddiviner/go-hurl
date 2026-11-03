package httpclient

import (
	"bytes"
	"encoding/hex"
	"net"
	"slices"
	"strings"
	"sync/atomic"

	"golang.org/x/net/http/httpguts"
)

// rawHeaderPrefix marks a response header whose name net/http would reject
// (curl accepts any name): the real name follows, hex encoded.
const rawHeaderPrefix = "X-Go-Hurl-Raw-"

// lenientConn is a plain-text HTTP/1.x connection that rewrites response
// header lines with an invalid field name to rawHeaderPrefix + hex(name),
// so that net/http parses them instead of failing the transfer.
type lenientConn struct {
	net.Conn
	pending atomic.Bool // a request was written, so a response head follows

	inHead        bool
	status        bool // the next line is a status line
	informational bool
	line          []byte
	out           []byte
	err           error
}

func (c *lenientConn) Write(p []byte) (int, error) {
	c.pending.Store(true)
	return c.Conn.Write(p)
}

func (c *lenientConn) Read(p []byte) (int, error) {
	for len(c.out) == 0 {
		if err := c.err; err != nil {
			c.err = nil
			return 0, err
		}
		n, err := c.Conn.Read(p)
		// The transport may already be reading when the request is written.
		if c.pending.Swap(false) && !c.inHead {
			c.inHead, c.status = true, true
		}
		if !c.inHead {
			return n, err
		}
		c.feed(p[:n])
		if err != nil {
			c.out, c.line, c.inHead, c.err = append(c.out, c.line...), nil, false, err
		}
	}
	n := copy(p, c.out)
	c.out = c.out[n:]
	return n, nil
}

func (c *lenientConn) feed(data []byte) {
	for len(data) > 0 && c.inHead {
		i := bytes.IndexByte(data, '\n')
		chunk := data
		if i >= 0 {
			chunk = data[:i+1]
		}
		c.line = append(c.line, chunk...)
		data = data[len(chunk):]
		// Not a response head (e.g. the rest of a body sent while the
		// request was still being written): pass everything through.
		if n := min(len(c.line), 5); c.status && string(c.line[:n]) != "HTTP/"[:n] {
			c.out = append(c.out, c.line...)
			c.line, c.inHead = c.line[:0], false
			break
		}
		if i >= 0 {
			c.endLine()
		}
	}
	c.out = append(c.out, data...)
}

func (c *lenientConn) endLine() {
	line := c.line
	switch {
	case c.status:
		fields := bytes.Fields(line)
		c.status = false
		c.informational = len(fields) > 1 && len(fields[1]) == 3 && fields[1][0] == '1'
	case len(bytes.TrimRight(line, "\r\n")) == 0:
		c.inHead, c.status = c.informational, c.informational
		c.informational = false
	default:
		line = escapeHeaderName(line)
	}
	c.out = append(c.out, line...)
	c.line = c.line[:0]
}

func escapeHeaderName(line []byte) []byte {
	if line[0] == ' ' || line[0] == '\t' {
		return line
	}
	name, rest, ok := bytes.Cut(line, []byte(":"))
	if !ok || len(name) == 0 || httpguts.ValidHeaderFieldName(string(name)) {
		return line
	}
	return slices.Concat([]byte(rawHeaderPrefix+hex.EncodeToString(name)+":"), rest)
}

func unescapeHeaderName(name string) string {
	if s, ok := strings.CutPrefix(name, rawHeaderPrefix); ok {
		if b, err := hex.DecodeString(s); err == nil {
			return string(b)
		}
	}
	return name
}
