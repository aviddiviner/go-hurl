package httpclient

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestLenientResponseHeaders(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		if _, err := http.ReadRequest(bufio.NewReader(conn)); err != nil {
			return
		}
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 5\r\n<script>x</script>: foo\r\nX-Ok: 1\r\nConnection: close\r\n\r\nhello"))
	}()

	call := execute(t, NewClient(), newSpec(t, "GET", "http://"+ln.Addr().String()+"/"), DefaultClientOptions())
	h := call.Response.Headers
	for name, want := range map[string]string{"<script>x</script>": "foo", "X-Ok": "1", "Connection": "close"} {
		if got := h.Values(name); len(got) != 1 || got[0] != want {
			t.Errorf("%s: %q", name, got)
		}
	}
	if string(call.Response.Body) != "hello" {
		t.Errorf("body %q", call.Response.Body)
	}
}

func TestVerifyPeerCommonNameFallback(t *testing.T) {
	newCert := func(dnsNames []string) (*x509.Certificate, *x509.CertPool) {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		tmpl := &x509.Certificate{
			SerialNumber:          big.NewInt(1),
			Subject:               pkix.Name{CommonName: "localhost"},
			DNSNames:              dnsNames,
			NotBefore:             time.Now().Add(-time.Hour),
			NotAfter:              time.Now().Add(time.Hour),
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
			ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			IsCA:                  true,
			BasicConstraintsValid: true,
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
		if err != nil {
			t.Fatal(err)
		}
		cert, _ := x509.ParseCertificate(der)
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		return cert, pool
	}

	cnOnly, pool := newCert(nil)
	if err := verifyPeer([]*x509.Certificate{cnOnly}, pool, "localhost"); err != nil {
		t.Errorf("CN only: %v", err)
	}
	if err := verifyPeer([]*x509.Certificate{cnOnly}, pool, "example.com"); !errors.As(err, new(x509.HostnameError)) {
		t.Errorf("CN only, other host: %v", err)
	}
	withSan, pool := newCert([]string{"example.com"})
	if err := verifyPeer([]*x509.Certificate{withSan}, pool, "localhost"); !errors.As(err, new(x509.HostnameError)) {
		t.Errorf("SAN present, CN ignored: %v", err)
	}
	if err := verifyPeer([]*x509.Certificate{withSan}, x509.NewCertPool(), "example.com"); !errors.As(err, new(x509.UnknownAuthorityError)) {
		t.Errorf("unknown authority: %v", err)
	}
}

func TestPostWithoutBody(t *testing.T) {
	var raw []string
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw = append(raw, r.Method+" "+r.Header.Get("Content-Length")+" "+strings.Join(r.TransferEncoding, ","))
	}))
	t.Cleanup(s.Close)
	c := NewClient()
	for _, method := range []string{"POST", "PUT", "PATCH", "POST"} {
		call := execute(t, c, newSpec(t, method, s.URL), DefaultClientOptions())
		if call.Request.Headers.ContainsKey("Content-Length") {
			t.Errorf("%s: %v", method, call.Request.Headers)
		}
	}
	if want := []string{"POST  ", "PUT  ", "PATCH  ", "POST  "}; !slices.Equal(raw, want) {
		t.Errorf("got %q", raw)
	}
}
