package httpclient

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// transportConfig holds the options that shape a Transport. Clients cache
// one Transport per distinct config, so connections are reused between
// requests sharing the same options, like libcurl's connection cache.
type transportConfig struct {
	cacertFile     string
	clientCertFile string
	clientKeyFile  string
	connectTimeout time.Duration
	connectsTo     string
	httpVersion    RequestedHttpVersion
	insecure       bool
	ipResolve      IpResolve
	noProxy        string
	hasNoProxy     bool
	pinnedPubKey   string
	proxy          string
	proxyHeaders   string
	resolves       string
	tlsMax         TlsVersion
	tlsMin         TlsVersion
	unixSocket     string
}

func newTransportConfig(o ClientOptions) transportConfig {
	c := transportConfig{
		cacertFile:     o.CacertFile,
		clientCertFile: o.ClientCertFile,
		clientKeyFile:  o.ClientKeyFile,
		connectTimeout: o.ConnectTimeout,
		connectsTo:     strings.Join(o.ConnectsTo, "\n"),
		httpVersion:    o.HttpVersion,
		insecure:       o.Insecure,
		ipResolve:      o.IpResolve,
		pinnedPubKey:   o.PinnedPubKey,
		proxy:          o.Proxy,
		resolves:       strings.Join(o.Resolves, "\n"),
		tlsMax:         o.TlsMax,
		tlsMin:         o.TlsMin,
		unixSocket:     o.UnixSocket,
	}
	if o.NoProxy != nil {
		c.noProxy, c.hasNoProxy = *o.NoProxy, true
	}
	var ph []string
	for _, h := range o.ProxyHeaders.All() {
		ph = append(ph, h.String())
	}
	c.proxyHeaders = strings.Join(ph, "\n")
	return c
}

// newTransport creates a Transport for options. Redirects and compression
// are handled by the client, so compression is disabled here.
func newTransport(o ClientOptions, allowReuse bool) (*http.Transport, HttpError) {
	tlsConfig, err := newTLSConfig(o)
	if err != nil {
		return nil, err
	}
	d := &dialer{
		connectTimeout: o.ConnectTimeout,
		ipResolve:      o.IpResolve,
		unixSocket:     o.UnixSocket,
	}
	for _, s := range o.ConnectsTo {
		if c, ok := parseConnectTo(s); ok {
			d.connectsTo = append(d.connectsTo, c)
		}
	}
	for _, s := range o.Resolves {
		if r, ok := parseResolve(s); ok {
			d.resolves = append(d.resolves, r)
		}
	}
	t := &http.Transport{
		Proxy:                 proxyFunc(o),
		DialContext:           d.dialContext,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   o.ConnectTimeout,
		DisableCompression:    true,
		DisableKeepAlives:     !allowReuse,
		ExpectContinueTimeout: time.Second,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       118 * time.Second,
	}
	t.OnProxyConnectResponse = func(ctx context.Context, _ *url.URL, _ *http.Request, res *http.Response) error {
		if tr := traceFromContext(ctx); tr != nil {
			tr.mu.Lock()
			tr.proxyConnectStatus = res.StatusCode
			tr.mu.Unlock()
		}
		return nil
	}
	if o.ProxyHeaders.Len() > 0 {
		t.ProxyConnectHeader = http.Header{}
		for _, h := range o.ProxyHeaders.All() {
			t.ProxyConnectHeader[h.Name] = append(t.ProxyConnectHeader[h.Name], h.Value)
		}
	}
	var protocols http.Protocols
	switch o.HttpVersion {
	case RequestedHttpVersionHttp11:
		protocols.SetHTTP1(true)
	case RequestedHttpVersionHttp2PriorKnowledge:
		protocols.SetHTTP2(true)
		protocols.SetUnencryptedHTTP2(true)
	default:
		protocols.SetHTTP1(true)
		protocols.SetHTTP2(true)
	}
	t.Protocols = &protocols
	return t, nil
}

func tlsVersion(v TlsVersion) uint16 {
	switch v {
	case TlsVersionTls10:
		return tls.VersionTLS10
	case TlsVersionTls11:
		return tls.VersionTLS11
	case TlsVersionTls12:
		return tls.VersionTLS12
	case TlsVersionTls13:
		return tls.VersionTLS13
	}
	return 0
}

func newTLSConfig(o ClientOptions) (*tls.Config, HttpError) {
	c := &tls.Config{
		InsecureSkipVerify: o.Insecure,
		MinVersion:         tlsVersion(o.TlsMin),
		MaxVersion:         tlsVersion(o.TlsMax),
	}
	if o.CacertFile != "" {
		data, err := os.ReadFile(o.CacertFile)
		if err != nil {
			return nil, HttpErrorLibcurl{Code: 77, Detail: fmt.Sprintf("error setting certificate file: %s", o.CacertFile)}
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, HttpErrorLibcurl{Code: 77, Detail: fmt.Sprintf("error setting certificate file: %s", o.CacertFile)}
		}
		c.RootCAs = pool
	}
	if o.ClientCertFile != "" {
		cert, err := loadClientCert(o.ClientCertFile, o.ClientKeyFile)
		if err != nil {
			return nil, err
		}
		c.Certificates = []tls.Certificate{cert}
	}
	if o.PinnedPubKey != "" {
		verify, err := pinnedPubKeyVerifier(o.PinnedPubKey)
		if err != nil {
			return nil, err
		}
		c.VerifyConnection = verify
	}
	return c, nil
}

// loadClientCert loads a PEM client certificate and an unencrypted PEM key
// (from keyFile, or the certificate file itself).
func loadClientCert(certFile, keyFile string) (tls.Certificate, HttpError) {
	certPath, _, hasPassword := parseCertPassword(certFile)
	if hasPassword {
		return tls.Certificate{}, HttpErrorUnsupportedFeature{Feature: "Client certificate password (--cert <file>:<password>)"}
	}
	if keyFile == "" {
		keyFile = certPath
	}
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return tls.Certificate{}, HttpErrorLibcurl{Code: 58, Detail: fmt.Sprintf("could not load PEM client certificate from %s", certPath)}
	}
	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return tls.Certificate{}, HttpErrorLibcurl{Code: 58, Detail: fmt.Sprintf("unable to set private key file: '%s' type PEM", keyFile)}
	}
	if isEncryptedKey(keyPEM) {
		return tls.Certificate{}, HttpErrorUnsupportedFeature{Feature: "Encrypted client key"}
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return tls.Certificate{}, HttpErrorLibcurl{Code: 58, Detail: fmt.Sprintf("could not load PEM client certificate from %s (%v)", certPath, err)}
	}
	return cert, nil
}

func isEncryptedKey(data []byte) bool {
	for {
		var block *pem.Block
		block, data = pem.Decode(data)
		if block == nil {
			return false
		}
		if block.Type == "ENCRYPTED PRIVATE KEY" || strings.Contains(block.Headers["Proc-Type"], "ENCRYPTED") {
			return true
		}
	}
}

// parseCertPassword splits a --cert value into the certificate file name
// and an optional password. In the certificate part, ':' is escaped as "\:"
// and '\' as "\\".
func parseCertPassword(certAndPass string) (cert string, password string, hasPassword bool) {
	var c, p strings.Builder
	parseCert := true
	runes := []rune(certAndPass)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if !parseCert {
			p.WriteRune(r)
			continue
		}
		switch r {
		case '\\':
			if i+1 < len(runes) {
				i++
				c.WriteRune(runes[i])
			} else {
				c.WriteRune('\\')
			}
		case ':':
			parseCert = false
		default:
			c.WriteRune(r)
		}
	}
	return c.String(), p.String(), !parseCert
}

var errPinnedPubKey = errors.New("SSL: public key does not match pinned public key")

// pinnedPubKeyVerifier checks the server public key against a
// --pinnedpubkey value: "sha256//<base64>;sha256//<base64>..." hashes, or a
// PEM/DER public key file.
func pinnedPubKeyVerifier(value string) (func(tls.ConnectionState) error, HttpError) {
	var hashes []string
	var spki []byte
	if strings.HasPrefix(value, "sha256//") {
		for h := range strings.SplitSeq(value, ";") {
			if s, ok := strings.CutPrefix(strings.TrimSpace(h), "sha256//"); ok {
				hashes = append(hashes, s)
			}
		}
	} else {
		data, err := os.ReadFile(value)
		if err != nil {
			return nil, HttpErrorLibcurl{Code: 90, Detail: errPinnedPubKey.Error()}
		}
		if block, _ := pem.Decode(data); block != nil {
			data = block.Bytes
		}
		spki = data
	}
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return errPinnedPubKey
		}
		raw := cs.PeerCertificates[0].RawSubjectPublicKeyInfo
		if spki != nil {
			if string(spki) == string(raw) {
				return nil
			}
			return errPinnedPubKey
		}
		sum := sha256.Sum256(raw)
		got := base64.StdEncoding.EncodeToString(sum[:])
		for _, h := range hashes {
			if h == got {
				return nil
			}
		}
		return errPinnedPubKey
	}, nil
}

// proxyFunc returns the proxy selection function: the --proxy option, or
// the environment (http_proxy, https_proxy, no_proxy...), filtered by
// --noproxy.
func proxyFunc(o ClientOptions) func(*http.Request) (*url.URL, error) {
	noProxy := func(*url.URL) bool { return false }
	if o.NoProxy != nil {
		list := *o.NoProxy
		noProxy = func(u *url.URL) bool { return matchNoProxy(list, u.Hostname()) }
	}
	if o.Proxy != "" {
		proxy := o.Proxy
		if !strings.Contains(proxy, "://") {
			proxy = "http://" + proxy
		}
		return func(r *http.Request) (*url.URL, error) {
			if noProxy(r.URL) {
				return nil, nil
			}
			return url.Parse(proxy)
		}
	}
	return func(r *http.Request) (*url.URL, error) {
		if o.NoProxy != nil {
			if noProxy(r.URL) {
				return nil, nil
			}
		}
		return http.ProxyFromEnvironment(r)
	}
}

// matchNoProxy implements curl's --noproxy matching: a comma separated list
// of hosts ("*" matches all), matching the host or its subdomains, or IP
// addresses / CIDR ranges.
func matchNoProxy(list, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	ip := net.ParseIP(host)
	for entry := range strings.SplitSeq(list, ",") {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "" {
			continue
		}
		if entry == "*" {
			return true
		}
		if ip != nil {
			if _, cidr, err := net.ParseCIDR(entry); err == nil {
				if cidr.Contains(ip) {
					return true
				}
				continue
			}
			if e := net.ParseIP(strings.Trim(entry, "[]")); e != nil && e.Equal(ip) {
				return true
			}
			continue
		}
		entry = strings.TrimSuffix(strings.TrimPrefix(entry, "."), ".")
		if host == entry || strings.HasSuffix(host, "."+entry) {
			return true
		}
	}
	return false
}

type connectTo struct {
	host1, port1, host2, port2 string
}

// parseConnectTo parses "HOST1:PORT1:HOST2:PORT2" (IPv6 hosts in brackets,
// empty parts allowed).
func parseConnectTo(s string) (connectTo, bool) {
	parts, ok := splitHostPorts(s, 4)
	if !ok {
		return connectTo{}, false
	}
	return connectTo{parts[0], parts[1], parts[2], parts[3]}, true
}

type resolveEntry struct {
	host  string
	port  string
	addrs []string
}

// parseResolve parses "[+]HOST:PORT:ADDR[,ADDR]...".
func parseResolve(s string) (resolveEntry, bool) {
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "-") {
		return resolveEntry{}, false
	}
	parts, ok := splitHostPorts(s, 3)
	if !ok {
		return resolveEntry{}, false
	}
	var addrs []string
	for a := range strings.SplitSeq(parts[2], ",") {
		a = strings.Trim(strings.TrimSpace(a), "[]")
		if a != "" {
			addrs = append(addrs, a)
		}
	}
	return resolveEntry{host: parts[0], port: parts[1], addrs: addrs}, len(addrs) > 0
}

// splitHostPorts splits s on ':' into n parts, keeping bracketed IPv6
// addresses together. The last part is the remainder.
func splitHostPorts(s string, n int) ([]string, bool) {
	var parts []string
	for len(parts) < n-1 {
		if strings.HasPrefix(s, "[") {
			end := strings.Index(s, "]")
			if end < 0 {
				return nil, false
			}
			part := s[1:end]
			s = s[end+1:]
			if !strings.HasPrefix(s, ":") {
				return nil, false
			}
			parts = append(parts, part)
			s = s[1:]
			continue
		}
		i := strings.IndexByte(s, ':')
		if i < 0 {
			return nil, false
		}
		parts = append(parts, s[:i])
		s = s[i+1:]
	}
	return append(parts, strings.Trim(s, "[]")), true
}

type dialer struct {
	connectTimeout time.Duration
	ipResolve      IpResolve
	unixSocket     string
	connectsTo     []connectTo
	resolves       []resolveEntry
}

// dialError is a connection error, mapped to libcurl errors by the client.
type dialError struct {
	host    string
	port    string
	proxy   bool
	dns     bool
	timeout bool
	elapsed time.Duration
	err     error
}

func (e *dialError) Error() string { return e.err.Error() }
func (e *dialError) Unwrap() error { return e.err }

func (d *dialer) dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	start := time.Now()
	tr := traceFromContext(ctx)
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	isProxy := tr != nil && tr.proxyAddr != "" && addr == tr.proxyAddr

	if d.connectTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, d.connectTimeout)
		defer cancel()
	}
	fail := func(err error, dns bool) error {
		return &dialError{
			host: host, port: port, proxy: isProxy, dns: dns, elapsed: time.Since(start), err: err,
			timeout: errors.Is(err, context.DeadlineExceeded) || isTimeout(err),
		}
	}

	if d.unixSocket != "" {
		tr.chatter(fmt.Sprintf("  Trying %s:%s...", d.unixSocket, port))
		var nd net.Dialer
		conn, err := nd.DialContext(ctx, "unix", d.unixSocket)
		if err != nil {
			return nil, fail(err, false)
		}
		tr.connected(start)
		tr.chatter(fmt.Sprintf("Connected to %s (%s) port %s", host, d.unixSocket, port))
		return conn, nil
	}

	connectHost, connectPort := host, port
	if !isProxy {
		for _, c := range d.connectsTo {
			if (c.host1 == "" || strings.EqualFold(c.host1, host)) && (c.port1 == "" || c.port1 == port) {
				if c.host2 != "" {
					connectHost = c.host2
				}
				if c.port2 != "" {
					connectPort = c.port2
				}
				tr.chatter(fmt.Sprintf("Connecting to hostname: %s", connectHost))
				break
			}
		}
	}

	var ips []net.IP
	if ip := net.ParseIP(connectHost); ip != nil {
		ips = []net.IP{ip}
	} else {
		for _, r := range d.resolves {
			if (r.host == "*" || strings.EqualFold(r.host, connectHost)) && r.port == connectPort {
				for _, a := range r.addrs {
					if ip := net.ParseIP(a); ip != nil {
						ips = append(ips, ip)
					}
				}
				break
			}
		}
		if ips == nil {
			addrs, err := net.DefaultResolver.LookupIPAddr(ctx, connectHost)
			if err != nil {
				return nil, fail(err, true)
			}
			for _, a := range addrs {
				ips = append(ips, a.IP)
			}
		}
		tr.resolved(start, connectHost, connectPort, ips)
	}

	var candidates []net.IP
	for _, ip := range ips {
		isV4 := ip.To4() != nil
		if (d.ipResolve == IpResolveIpV4 && !isV4) || (d.ipResolve == IpResolveIpV6 && isV4) {
			continue
		}
		candidates = append(candidates, ip)
	}
	if len(candidates) == 0 {
		return nil, fail(&net.DNSError{Err: "no such host", Name: connectHost, IsNotFound: true}, true)
	}

	var lastErr error
	for _, ip := range candidates {
		target := net.JoinHostPort(ip.String(), connectPort)
		tr.chatter(fmt.Sprintf("  Trying %s...", target))
		var nd net.Dialer
		conn, err := nd.DialContext(ctx, network, target)
		if err == nil {
			tr.connected(start)
			tr.chatter(fmt.Sprintf("Connected to %s (%s) port %s", host, ip, connectPort))
			return conn, nil
		}
		tr.chatter(fmt.Sprintf("connect to %s port %s failed: %s", ip, connectPort, dialReason(err)))
		lastErr = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, fail(lastErr, false)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

func dialReason(err error) string {
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Err != nil {
		var se *os.SyscallError
		if errors.As(oe.Err, &se) {
			return capitalize(se.Err.Error())
		}
		return oe.Err.Error()
	}
	return err.Error()
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
