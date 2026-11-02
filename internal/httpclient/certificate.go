package httpclient

import (
	"crypto/x509"
	"crypto/x509/pkix"

	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// Certificate is an SSL/TLS certificate. Each attribute is optional, so we
// can test invalid certificates (i.e. a certificate without serial number).
type Certificate struct {
	subject        *string
	issuer         *string
	startDate      *time.Time
	expireDate     *time.Time
	serialNumber   *string
	subjectAltName *string
	value          *string
}

func NewCertificate(subject, issuer *string, startDate, expireDate *time.Time, serialNumber, subjectAltName, value *string) *Certificate {
	return &Certificate{subject, issuer, startDate, expireDate, serialNumber, subjectAltName, value}
}

func (c *Certificate) Subject() *string        { return c.subject }
func (c *Certificate) Issuer() *string         { return c.issuer }
func (c *Certificate) StartDate() *time.Time   { return c.startDate }
func (c *Certificate) ExpireDate() *time.Time  { return c.expireDate }
func (c *Certificate) SerialNumber() *string   { return c.serialNumber }
func (c *Certificate) SubjectAltName() *string { return c.subjectAltName }

// Value returns the PEM value of the certificate.
func (c *Certificate) Value() *string { return c.value }

// certInfo is the certificate information in libcurl's CURLINFO_CERTINFO
// format: a list of "name:content" entries.
type certInfo struct {
	data  []string
	value *string
}

// newCertInfo renders an X.509 certificate like libcurl's OpenSSL backend.
func newCertInfo(cert *x509.Certificate) certInfo {
	data := []string{
		"Subject:" + opensslName(cert.Subject),
		"Issuer:" + opensslName(cert.Issuer),
		"Serial Number:" + hexSerial(cert.SerialNumber.Bytes()),
		"Start date:" + cert.NotBefore.UTC().Format("2006-01-02 15:04:05") + " GMT",
		"Expire date:" + cert.NotAfter.UTC().Format("2006-01-02 15:04:05") + " GMT",
	}
	if san := subjectAltName(cert); san != "" {
		data = append(data, "X509v3 Subject Alternative Name:"+san)
	}
	value := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}))
	data = append(data, "Cert:"+value)
	return certInfo{data: data, value: &value}
}

var opensslShortNames = map[string]string{
	"2.5.4.3":                    "CN",
	"2.5.4.4":                    "SN",
	"2.5.4.5":                    "serialNumber",
	"2.5.4.6":                    "C",
	"2.5.4.7":                    "L",
	"2.5.4.8":                    "ST",
	"2.5.4.9":                    "street",
	"2.5.4.10":                   "O",
	"2.5.4.11":                   "OU",
	"2.5.4.12":                   "title",
	"2.5.4.17":                   "postalCode",
	"2.5.4.42":                   "GN",
	"0.9.2342.19200300.100.1.1":  "UID",
	"0.9.2342.19200300.100.1.25": "DC",
	"1.2.840.113549.1.9.1":       "emailAddress",
}

// opensslName formats a distinguished name like OpenSSL's one line format:
// "C = US, O = Example, CN = localhost", in certificate order.
func opensslName(name pkix.Name) string {
	var parts []string
	for _, atv := range name.Names {
		key, ok := opensslShortNames[atv.Type.String()]
		if !ok {
			key = atv.Type.String()
		}
		parts = append(parts, key+" = "+fmt.Sprint(atv.Value))
	}
	return strings.Join(parts, ", ")
}

func hexSerial(b []byte) string {
	if len(b) == 0 {
		return "00"
	}
	return hex.EncodeToString(b)
}

func subjectAltName(cert *x509.Certificate) string {
	var parts []string
	for _, d := range cert.DNSNames {
		parts = append(parts, "DNS:"+d)
	}
	for _, ip := range cert.IPAddresses {
		parts = append(parts, "IP address:"+opensslIP(ip))
	}
	for _, e := range cert.EmailAddresses {
		parts = append(parts, "email:"+e)
	}
	for _, u := range cert.URIs {
		parts = append(parts, "URI:"+u.String())
	}
	return strings.Join(parts, ", ")
}

// opensslIP formats IPv6 addresses uncompressed, without leading zeros
// (0:0:0:0:0:0:0:1), like OpenSSL.
func opensslIP(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String()
	}
	var groups []string
	for i := 0; i < len(ip); i += 2 {
		groups = append(groups, strconv.FormatUint(uint64(ip[i])<<8|uint64(ip[i+1]), 16))
	}
	return strings.Join(groups, ":")
}

const (
	subjectAttribute        = "subject"
	issuerAttribute         = "issuer"
	startDateAttribute      = "start date"
	expireDateAttribute     = "expire date"
	serialNumberAttribute   = "serial number"
	subjectAltNameAttribute = "x509v3 subject alternative name"
)

var certAttributes = []string{
	subjectAttribute, issuerAttribute, startDateAttribute, expireDateAttribute,
	serialNumberAttribute, subjectAltNameAttribute,
}

// certificateFromCertInfo parses certificate information. It supports
// different formats: attribute name "Start date" vs "Start Date", date
// "Jan 10 08:29:52 2023 GMT" vs "2023-01-10 08:29:52 GMT".
func certificateFromCertInfo(info certInfo) *Certificate {
	attributes := parseCertAttributes(info.data)
	return &Certificate{
		subject:        parseCertString(attributes, subjectAttribute),
		issuer:         parseCertString(attributes, issuerAttribute),
		startDate:      parseCertDate(attributes, startDateAttribute),
		expireDate:     parseCertDate(attributes, expireDateAttribute),
		serialNumber:   parseSerialNumber(attributes),
		subjectAltName: parseCertString(attributes, subjectAltNameAttribute),
		value:          info.value,
	}
}

func parseCertString(attributes map[string]string, name string) *string {
	if v, ok := attributes[name]; ok {
		return &v
	}
	return nil
}

func parseCertDate(attributes map[string]string, name string) *time.Time {
	v, ok := attributes[name]
	if !ok {
		return nil
	}
	d, err := parseCertDateValue(v)
	if err != nil {
		return nil
	}
	return &d
}

func parseCertDateValue(value string) (time.Time, error) {
	if d, err := time.Parse("Jan _2 15:04:05 2006 GMT", value); err == nil {
		return d, nil
	}
	d, err := time.Parse("2006-01-02 15:04:05 GMT", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("can not parse date <%s>", value)
	}
	return d, nil
}

// parseSerialNumber normalizes the serial number: libcurl gives either
// "AA:BB:CC" or "AABBCC".
func parseSerialNumber(attributes map[string]string) *string {
	v, ok := attributes[serialNumberAttribute]
	if !ok {
		return nil
	}
	var parts []string
	if strings.Contains(v, ":") {
		for p := range strings.SplitSeq(v, ":") {
			if p != "" {
				parts = append(parts, p)
			}
		}
	} else {
		runes := []rune(v)
		for i := 0; i < len(runes); i += 2 {
			parts = append(parts, string(runes[i:min(i+2, len(runes))]))
		}
	}
	s := strings.Join(parts, ":")
	return &s
}

func parseCertAttributes(data []string) map[string]string {
	m := map[string]string{}
	for _, s := range data {
		name, value, ok := strings.Cut(s, ":")
		if !ok {
			continue
		}
		name = strings.ToLower(name)
		for _, a := range certAttributes {
			if a == name {
				m[a] = value
			}
		}
	}
	return m
}
