package httpclient

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// tunnelsHTTPS reports whether we open the CONNECT tunnel to proxy ourselves
// for HTTPS requests, instead of net/http. We do it for plain HTTP proxies so
// that the CONNECT exchange can be logged and the target certificate is
// verified by connTLSConfig, like for a direct connection.
func tunnelsHTTPS(proxy *url.URL) bool {
	return proxy != nil && proxy.Scheme == "http"
}

// dialTunnel connects to the proxy and opens a CONNECT tunnel to addr.
func (d *dialer) dialTunnel(ctx context.Context, tr *transferTrace, proxy *url.URL, proxyHeaders []Header, addr string) (net.Conn, error) {
	conn, err := d.dialContext(ctx, "tcp", canonicalAddr(proxy))
	if err != nil {
		return nil, err
	}
	deadline, ok := ctx.Deadline()
	if d.connectTimeout > 0 {
		if t := time.Now().Add(d.connectTimeout); !ok || t.Before(deadline) {
			deadline, ok = t, true
		}
	}
	if ok {
		conn.SetDeadline(deadline)
	}

	headers := []Header{{Name: "Host", Value: addr}}
	if u := proxy.User; u != nil {
		password, _ := u.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(u.Username() + ":" + password))
		headers = append(headers, Header{Name: "Proxy-Authorization", Value: "Basic " + auth})
	}
	headers = append(headers, Header{Name: "Proxy-Connection", Value: "Keep-Alive"})
	headers = append(headers, proxyHeaders...)

	requestLine := "CONNECT " + addr + " HTTP/1.1"
	var b strings.Builder
	b.WriteString(requestLine + "\r\n")
	for _, h := range headers {
		fmt.Fprintf(&b, "%s: %s\r\n", h.Name, h.Value)
	}
	b.WriteString("\r\n")
	tr.event(traceEvent{kind: eventConnectOut, text: requestLine, headers: headers})
	if _, err := conn.Write([]byte(b.String())); err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		conn.Close()
		return nil, err
	}
	resp.Body.Close()
	// A successful CONNECT has no body, so net/http flags it as closing.
	resp.Close = false
	tr.event(traceEvent{kind: eventConnectIn, text: statusLine(resp), headers: responseHeaderVec(resp).All()})
	tr.mu.Lock()
	tr.proxyConnectStatus = resp.StatusCode
	tr.mu.Unlock()
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("proxy CONNECT failed: %s", resp.Status)
	}
	conn.SetDeadline(time.Time{})
	return conn, nil
}
