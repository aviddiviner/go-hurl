package runner

import (
	"slices"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// RunnerOptions are the options of a run, set globally (from the command
// line) and overridden per entry by [Options] sections. Build them with
// DefaultRunnerOptions and set the fields you need.
//
// As in httpclient.ClientOptions, unset string options are empty; Rust's
// Option<Count> is a nil ast.Count and FromEntry/ToEntry are 0 when unset.
type RunnerOptions struct {
	AllowReuse      bool
	AwsSigv4        string
	CacertFile      string
	ClientCertFile  string
	ClientKeyFile   string
	ColorStdout     bool
	Compressed      bool
	ConnectTimeout  time.Duration
	ConnectsTo      []string
	ContextDir      util.ContextDir
	ContinueOnError bool
	CookieInputFile string
	Delay           time.Duration
	Digest          bool
	FailWithBody    bool
	FollowLocation  httpclient.FollowLocation
	// FromEntry is the 1-based index of the first entry to run, 0 for the
	// first one.
	FromEntry   int
	Headers     httpclient.HeaderVec
	HttpVersion httpclient.RequestedHttpVersion
	Insecure    bool
	IpResolve   httpclient.IpResolve
	MaxFilesize *uint64
	// MaxRecvSpeed and MaxSendSpeed are in bytes per second, 0 for no limit.
	MaxRecvSpeed  uint64
	MaxRedirect   ast.Count
	MaxSendSpeed  uint64
	Negotiate     bool
	Netrc         bool
	NetrcFile     string
	NetrcOptional bool
	NoAssert      bool
	NoHeaders     []string
	NoProxy       *string
	Ntlm          bool
	Output        Output
	PathAsIs      bool
	PinnedPubKey  string
	Pretty        pretty.PrettyMode
	Proxy         string
	ProxyHeaders  httpclient.HeaderVec
	Repeat        ast.Count
	Resolves      []string
	Retry         ast.Count
	RetryInterval time.Duration
	Skip          bool
	SslNoRevoke   bool
	Timeout       time.Duration
	TlsMax        httpclient.TlsVersion
	TlsMin        httpclient.TlsVersion
	// ToEntry is the 1-based index of the last entry to run, 0 for the last
	// one.
	ToEntry             int
	UnixSocket          string
	UseCookieStore      bool
	UseJSONPathCoercion bool
	User                *string
	UserAgent           *string
}

// DefaultRunnerOptions returns the default runner options.
func DefaultRunnerOptions() RunnerOptions {
	return RunnerOptions{
		AllowReuse:          true,
		ColorStdout:         true,
		ConnectTimeout:      300 * time.Second,
		MaxRedirect:         ast.CountFinite{Value: 50},
		Pretty:              pretty.Automatic,
		RetryInterval:       1000 * time.Millisecond,
		Timeout:             300 * time.Second,
		UseCookieStore:      true,
		UseJSONPathCoercion: true,
	}
}

// clone returns a copy of o that doesn't share its slices.
func (o RunnerOptions) clone() RunnerOptions {
	o.ConnectsTo = slices.Clone(o.ConnectsTo)
	o.Headers = o.Headers.Clone()
	o.NoHeaders = slices.Clone(o.NoHeaders)
	o.ProxyHeaders = o.ProxyHeaders.Clone()
	o.Resolves = slices.Clone(o.Resolves)
	return o
}

// clientOptions returns the HTTP client options of these runner options.
func (o RunnerOptions) clientOptions(verbosity logger.Verbosity) httpclient.ClientOptions {
	return httpclient.ClientOptions{
		AllowReuse:      o.AllowReuse,
		AwsSigv4:        o.AwsSigv4,
		CacertFile:      o.CacertFile,
		ClientCertFile:  o.ClientCertFile,
		ClientKeyFile:   o.ClientKeyFile,
		Compressed:      o.Compressed,
		ConnectTimeout:  o.ConnectTimeout,
		ConnectsTo:      slices.Clone(o.ConnectsTo),
		CookieInputFile: o.CookieInputFile,
		Digest:          o.Digest,
		FollowLocation:  o.FollowLocation,
		Headers:         o.Headers.Clone(),
		HttpVersion:     o.HttpVersion,
		Insecure:        o.Insecure,
		IpResolve:       o.IpResolve,
		MaxFilesize:     o.MaxFilesize,
		MaxRecvSpeed:    o.MaxRecvSpeed,
		MaxRedirect:     o.MaxRedirect,
		MaxSendSpeed:    o.MaxSendSpeed,
		Negotiate:       o.Negotiate,
		Netrc:           o.Netrc,
		NetrcFile:       o.NetrcFile,
		NetrcOptional:   o.NetrcOptional,
		NoHeaders:       slices.Clone(o.NoHeaders),
		NoProxy:         o.NoProxy,
		Ntlm:            o.Ntlm,
		PathAsIs:        o.PathAsIs,
		PinnedPubKey:    o.PinnedPubKey,
		Proxy:           o.Proxy,
		ProxyHeaders:    o.ProxyHeaders.Clone(),
		Resolves:        slices.Clone(o.Resolves),
		SslNoRevoke:     o.SslNoRevoke,
		Timeout:         o.Timeout,
		TlsMax:          o.TlsMax,
		TlsMin:          o.TlsMin,
		UnixSocket:      o.UnixSocket,
		UseCookieStore:  o.UseCookieStore,
		User:            o.User,
		UserAgent:       o.UserAgent,
		Verbosity:       verbosity,
	}
}
