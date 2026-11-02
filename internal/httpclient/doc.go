// Package httpclient is Hurl's HTTP layer (hurl/src/http), ported from
// libcurl to net/http.
//
// Each Client keeps a cookie jar (a port of libcurl's cookie engine) and one
// http.Transport per distinct set of transport options, so connections are
// reused like libcurl's connection cache. net/http never follows
// redirections: ExecuteWithRedirect does, as Hurl does.
//
// libcurl features map onto Go as follows:
//
//   - --resolve, --connect-to, --unix-socket, --ipv4/--ipv6,
//     --connect-timeout: the Transport's DialContext.
//   - --insecure, --cacert, --cert/--key (unencrypted PEM), --tls-min/max,
//     --pinnedpubkey: tls.Config (VerifyConnection for pinning).
//   - --proxy, --noproxy, --proxy-header: Transport.Proxy and
//     ProxyConnectHeader (plus the headers on plain HTTP proxied requests).
//   - --http1.1, --http2, --http2-prior-knowledge: Transport.Protocols.
//   - --max-time: a context deadline over the whole transfer.
//   - --limit-rate, --max-filesize: wrapped readers.
//   - Timings, primary IP and the "**" debug lines: httptrace events and our
//     dialer. Certificates: tls.ConnectionState, rendered like libcurl's
//     OpenSSL certinfo.
//   - Transfer errors: mapped onto libcurl error codes and messages
//     (HttpErrorLibcurl), so they read like Hurl's.
//
// HTTP/1.0, HTTP/3, NTLM, Negotiate, digest, AWS SigV4 and encrypted client
// keys fail with an explicit "not supported by go-hurl" error. Brotli
// responses are reported as an unsupported content encoding.
package httpclient
