package httpclient

import (
	"runtime"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/version"
)

const DefaultUserAgent = version.UserAgent

// VersionInfo describes the HTTP engine, the go-hurl equivalent of
// libcurl_version_info.
type VersionInfo struct {
	Host      string
	Libraries []string
	Features  []string
}

// EngineVersionInfo returns the Go runtime and the supported features of the
// HTTP engine, sorted case insensitively.
func EngineVersionInfo() VersionInfo {
	return VersionInfo{
		Host:      runtime.GOOS + "/" + runtime.GOARCH,
		Libraries: []string{runtime.Version()},
		Features:  []string{"HTTP2", "IPv6", "libz", "SSL", "UnixSockets"},
	}
}

// ClientOptions are the options of an HTTP transfer. String options are
// unset when empty.
type ClientOptions struct {
	// AllowReuse allows reusing connections (true by default). false forces
	// a new connection, which is not reused afterwards.
	AllowReuse      bool
	AwsSigv4        string
	CacertFile      string
	ClientCertFile  string
	ClientKeyFile   string
	Compressed      bool
	ConnectTimeout  time.Duration
	ConnectsTo      []string
	CookieInputFile string
	Digest          bool
	FollowLocation  FollowLocation
	Headers         HeaderVec
	HttpVersion     RequestedHttpVersion
	Insecure        bool
	IpResolve       IpResolve
	MaxFilesize     *uint64
	// MaxRecvSpeed is in bytes per second, 0 for no limit.
	MaxRecvSpeed uint64
	MaxRedirect  ast.Count
	// MaxSendSpeed is in bytes per second, 0 for no limit.
	MaxSendSpeed   uint64
	Negotiate      bool
	Netrc          bool
	NetrcFile      string
	NetrcOptional  bool
	NoHeaders      []string
	NoProxy        *string
	Ntlm           bool
	PathAsIs       bool
	PinnedPubKey   string
	Proxy          string
	ProxyHeaders   HeaderVec
	Resolves       []string
	SslNoRevoke    bool
	Timeout        time.Duration
	TlsMax         TlsVersion
	TlsMin         TlsVersion
	UnixSocket     string
	UseCookieStore bool
	User           *string
	UserAgent      *string
	Verbosity      logger.Verbosity
}

// DefaultClientOptions returns the default options.
func DefaultClientOptions() ClientOptions {
	return ClientOptions{
		AllowReuse:     true,
		ConnectTimeout: 300 * time.Second,
		MaxRedirect:    ast.CountFinite{Value: 50},
		Timeout:        300 * time.Second,
		UseCookieStore: true,
	}
}
