package httpclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http/httptrace"
	"strings"
	"sync"
	"time"
)

// transferTrace records what happens during a transfer, from httptrace
// events and our dialer: timings (relative to the transfer start, like
// libcurl), the request headers really written, the connection, and the
// "**" debug lines of very verbose mode. Callbacks run on transport
// goroutines; everything is replayed on the caller goroutine afterwards.
type transferTrace struct {
	mu    sync.Mutex
	start time.Time
	// proxyAddr is the address of the proxy used for this request, if any.
	proxyAddr string

	nameLookup    time.Duration
	connect       time.Duration
	appConnect    time.Duration
	preTransfer   time.Duration
	startTransfer time.Duration

	events      []traceEvent
	reqHeaders  []Header
	h2          bool
	wroteHeader bool
	gotConn     bool
	remoteAddr  net.Addr
	// proxyConnectStatus is the status of the proxy CONNECT response.
	proxyConnectStatus int
}

type traceEventKind int

const (
	eventChatter traceEventKind = iota
	// eventHeaders marks the point where request headers were written.
	eventHeaders
	// eventRequestSent marks the point where the request was fully written.
	eventRequestSent
)

type traceEvent struct {
	kind traceEventKind
	text string
}

type traceKey struct{}

func traceFromContext(ctx context.Context) *transferTrace {
	tr, _ := ctx.Value(traceKey{}).(*transferTrace)
	return tr
}

func (t *transferTrace) since() time.Duration { return time.Since(t.start) }

func (t *transferTrace) chatter(s string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.events = append(t.events, traceEvent{kind: eventChatter, text: s})
}

func (t *transferTrace) resolved(start time.Time, host, port string, ips []net.IP) {
	if t == nil {
		return
	}
	var v4, v6 []string
	for _, ip := range ips {
		if ip.To4() != nil {
			v4 = append(v4, ip.String())
		} else {
			v6 = append(v6, ip.String())
		}
	}
	list := func(l []string) string {
		if len(l) == 0 {
			return "(none)"
		}
		return strings.Join(l, ", ")
	}
	t.mu.Lock()
	t.nameLookup = t.since()
	t.mu.Unlock()
	t.chatter(fmt.Sprintf("Host %s:%s was resolved.", host, port))
	t.chatter("IPv6: " + list(v6))
	t.chatter("IPv4: " + list(v4))
}

func (t *transferTrace) connected(time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connect = t.since()
	if t.nameLookup == 0 {
		t.nameLookup = t.connect
	}
}

func (t *transferTrace) clientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		GotConn: func(info httptrace.GotConnInfo) {
			t.mu.Lock()
			t.gotConn = true
			t.remoteAddr = info.Conn.RemoteAddr()
			t.preTransfer = t.since()
			t.mu.Unlock()
			if info.Reused {
				t.chatter("Re-using existing connection")
			}
		},
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			if err != nil {
				return
			}
			t.mu.Lock()
			t.appConnect = t.since()
			t.mu.Unlock()
			t.chatter(fmt.Sprintf("SSL connection using %s / %s", tlsVersionName(cs.Version), tls.CipherSuiteName(cs.CipherSuite)))
			if cs.NegotiatedProtocol != "" {
				t.chatter("ALPN: server accepted " + cs.NegotiatedProtocol)
			}
		},
		WroteHeaderField: func(key string, value []string) {
			t.mu.Lock()
			defer t.mu.Unlock()
			if key == "Host" || key == ":authority" {
				// A new request on the wire (Go may retry on a reused
				// connection): start over.
				t.reqHeaders = nil
				t.h2 = key == ":authority"
			}
			if strings.HasPrefix(key, ":") && key != ":authority" {
				return
			}
			for _, v := range value {
				t.reqHeaders = append(t.reqHeaders, Header{Name: key, Value: v})
			}
		},
		WroteHeaders: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			if !t.wroteHeader {
				t.wroteHeader = true
				t.events = append(t.events, traceEvent{kind: eventHeaders})
			}
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err != nil {
				return
			}
			t.mu.Lock()
			t.events = append(t.events, traceEvent{kind: eventRequestSent})
			t.mu.Unlock()
			t.chatter("Request completely sent off")
		},
		GotFirstResponseByte: func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			t.startTransfer = t.since()
		},
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLSv1.0"
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS13:
		return "TLSv1.3"
	}
	return tls.VersionName(v)
}
