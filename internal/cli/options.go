// Package cli holds the hurl command line: options from the config file,
// environment variables and arguments, and the high level logger. Ported
// from hurl/src/cli.
package cli

import (
	"strings"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
	"github.com/aviddiviner/go-hurl/internal/version"
)

// CliOptions are all the options of the Hurl command line. Unset optional
// strings are empty, except where an empty value is meaningful (pointers).
type CliOptions struct {
	AwsSigv4              string
	CacertFile            string
	ClientCertFile        string
	ClientKeyFile         string
	ColorStdout           BoolOpt
	ColorStderr           BoolOpt
	Compressed            bool
	ConnectTimeout        time.Duration
	ConnectsTo            []string
	ContinueOnError       bool
	CookieInputFile       string
	CookieOutputFile      string
	CurlFile              string
	Delay                 time.Duration
	Digest                bool
	ErrorFormat           ErrorFormat
	FailWithBody          bool
	FileRoot              *string
	FollowLocation        bool
	FollowLocationTrusted bool
	// FromEntry is 0 when unset.
	FromEntry          int
	Headers            []string
	HtmlDir            string
	HttpVersion        HttpVersion
	NoAssert           bool
	NoJSONPathCoercion bool
	Include            bool
	InputFiles         []input.Input
	Insecure           bool
	IpResolve          IpResolve
	// Jobs is 0 when unset.
	Jobs          int
	JSONReportDir string
	JunitFile     string
	LimitRate     *uint64
	MaxFilesize   *uint64
	MaxRedirect   ast.Count
	Negotiate     bool
	Netrc         bool
	NetrcFile     string
	NetrcOptional bool
	NoCookieStore bool
	NoHeaders     []string
	NoProxy       *string
	Ntlm          bool
	Output        util.Output
	OutputType    OutputType
	Parallel      BoolOpt
	PathAsIs      bool
	PinnedPubKey  string
	Pretty        pretty.PrettyMode
	ProgressBar   BoolOpt
	Proxy         string
	ProxyHeaders  []string
	// Repeat and Retry are nil when unset.
	Repeat        ast.Count
	Resolves      []string
	Retry         ast.Count
	RetryInterval time.Duration
	Secrets       map[string]string
	SslNoRevoke   bool
	TapFile       string
	Test          bool
	Timeout       time.Duration
	TlsMax        TlsVersion
	TlsMin        TlsVersion
	// ToEntry is 0 when unset.
	ToEntry    int
	UnixSocket string
	User       *string
	UserAgent  *string
	Variables  map[string]runner.Value
	Verbosity  Verbosity
}

// BoolOpt is a boolean option that is either set explicitly, or determined
// automatically.
type BoolOpt int

const (
	BoolOptAuto BoolOpt = iota
	BoolOptFalse
	BoolOptTrue
)

func boolOptSet(b bool) BoolOpt {
	if b {
		return BoolOptTrue
	}
	return BoolOptFalse
}

// Get returns the value if it has been set, and panics otherwise.
func (b BoolOpt) Get() bool {
	if b == BoolOptAuto {
		panic("no value set")
	}
	return b == BoolOptTrue
}

// UnwrapOr returns the value if it has been set, or def.
func (b BoolOpt) UnwrapOr(def bool) bool {
	if b == BoolOptAuto {
		return def
	}
	return b == BoolOptTrue
}

// Verbosity is the log verbosity level; VerbosityNone when unset.
type Verbosity int

const (
	VerbosityNone Verbosity = iota
	VerbosityBrief
	VerbosityVerbose
	VerbosityDebug
)

func parseVerbosity(s string) (Verbosity, error) {
	switch s {
	case "brief":
		return VerbosityBrief, nil
	case "verbose":
		return VerbosityVerbose, nil
	case "debug":
		return VerbosityDebug, nil
	}
	return VerbosityNone, optionsErrorf("Invalid value '%s' for verbosity [possible values: brief, verbose, debug]", s)
}

type ErrorFormat int

const (
	ErrorFormatShort ErrorFormat = iota
	ErrorFormatLong
)

func parseErrorFormat(s string) (ErrorFormat, error) {
	switch s {
	case "long":
		return ErrorFormatLong, nil
	case "short":
		return ErrorFormatShort, nil
	}
	return ErrorFormatShort, optionsErrorf("Invalid value '%s' for error-format [possible values: long, short]", s)
}

// HttpVersion is the requested HTTP version; HttpVersionNone when unset.
type HttpVersion int

const (
	HttpVersionNone HttpVersion = iota
	HttpVersionV10
	HttpVersionV11
	HttpVersionV2
	HttpVersionV2PriorKnowledge
	HttpVersionV3
)

func (v HttpVersion) requested() httpclient.RequestedHttpVersion {
	switch v {
	case HttpVersionV10:
		return httpclient.RequestedHttpVersionHttp10
	case HttpVersionV11:
		return httpclient.RequestedHttpVersionHttp11
	case HttpVersionV2:
		return httpclient.RequestedHttpVersionHttp2
	case HttpVersionV2PriorKnowledge:
		return httpclient.RequestedHttpVersionHttp2PriorKnowledge
	case HttpVersionV3:
		return httpclient.RequestedHttpVersionHttp3
	}
	return httpclient.RequestedHttpVersionDefault
}

// IpResolve is the IP protocol used; IpResolveNone when unset.
type IpResolve int

const (
	IpResolveNone IpResolve = iota
	IpResolveIpV4
	IpResolveIpV6
)

func (r IpResolve) client() httpclient.IpResolve {
	switch r {
	case IpResolveIpV4:
		return httpclient.IpResolveIpV4
	case IpResolveIpV6:
		return httpclient.IpResolveIpV6
	}
	return httpclient.IpResolveDefault
}

// TlsVersion is a TLS version; TlsVersionNone when unset.
type TlsVersion int

const (
	TlsVersionNone TlsVersion = iota
	TlsVersionTls10
	TlsVersionTls11
	TlsVersionTls12
	TlsVersionTls13
)

func parseTlsVersion(s string) (TlsVersion, string) {
	switch s {
	case "1.0":
		return TlsVersionTls10, ""
	case "1.1":
		return TlsVersionTls11, ""
	case "1.2":
		return TlsVersionTls12, ""
	case "1.3":
		return TlsVersionTls13, ""
	}
	return TlsVersionNone, "expecting 1.0, 1.1, 1.2 or 1.3"
}

func (v TlsVersion) client() httpclient.TlsVersion {
	switch v {
	case TlsVersionTls10:
		return httpclient.TlsVersionTls10
	case TlsVersionTls11:
		return httpclient.TlsVersionTls11
	case TlsVersionTls12:
		return httpclient.TlsVersionTls12
	case TlsVersionTls13:
		return httpclient.TlsVersionTls13
	}
	return httpclient.TlsVersionDefault
}

// OutputType is what is written on standard output when a Hurl file run is
// completed.
type OutputType int

const (
	// OutputTypeResponseBody outputs the last HTTP response body.
	OutputTypeResponseBody OutputType = iota
	// OutputTypeJSON outputs the whole run as JSON.
	OutputTypeJSON
	// OutputTypeNoOutput outputs nothing.
	OutputTypeNoOutput
)

// DefaultCliOptions returns the default options.
func DefaultCliOptions() *CliOptions {
	return &CliOptions{
		ConnectTimeout: 300 * time.Second,
		MaxRedirect:    ast.CountFinite{Value: 50},
		OutputType:     OutputTypeResponseBody,
		Pretty:         pretty.Automatic,
		RetryInterval:  1000 * time.Millisecond,
		Secrets:        map[string]string{},
		Timeout:        300 * time.Second,
		Variables:      map[string]runner.Value{},
	}
}

func getVersion() string {
	info := httpclient.EngineVersionInfo()
	return version.Hurl + " (go-hurl " + version.GoHurl + ") " + strings.Join(info.Libraries, " ") + " " + info.Host +
		"\nFeatures (net/http): " + strings.Join(info.Features, " ") +
		"\nFeatures (built-in): " + strings.Join(builtinFeatures, " ")
}

// builtinFeatures are the features implemented by go-hurl itself rather
// than by net/http.
var builtinFeatures = []string{"deflate", "gzip"}

// Parse parses the Hurl command line options from the config file, the
// environment variables and the command line arguments args (args[0] is the
// program). The error is an OptionsError.
func Parse(args []string, ctx *RunContext, env *EnvVars) (*CliOptions, error) {
	options := DefaultCliOptions()
	if err := parseConfigFile(ctx.ConfigFilePath(), options); err != nil {
		return nil, err
	}
	if err := parseEnvVars(env, options); err != nil {
		return nil, err
	}
	if err := parseCliArgs(args, ctx, options); err != nil {
		return nil, err
	}
	resolveImplicit(ctx, options)
	return options, nil
}

// resolveImplicit resolves each option that hasn't been set and has an
// implicit value.
func resolveImplicit(ctx *RunContext, o *CliOptions) {
	if o.ProgressBar == BoolOptAuto {
		interactive := ctx.IsStderrTerm() && !ctx.IsCI()
		o.ProgressBar = boolOptSet(o.Test && interactive)
	}
	if o.Parallel == BoolOptAuto {
		o.Parallel = boolOptSet(o.Test)
	}
	if o.OutputType == OutputTypeResponseBody && o.Test {
		o.OutputType = OutputTypeNoOutput
	}
	if o.Pretty == pretty.Automatic && !ctx.IsStdoutTerm() {
		o.Pretty = pretty.None
	}
	if o.ColorStdout == BoolOptAuto {
		o.ColorStdout = boolOptSet(ctx.IsStdoutTerm())
	}
	if o.ColorStderr == BoolOptAuto {
		o.ColorStderr = boolOptSet(ctx.IsStderrTerm())
	}
}

// rustParent is Rust's Path::parent for a relative or absolute file path.
func rustParent(path string) string {
	p := strings.TrimRight(path, "/")
	i := strings.LastIndex(p, "/")
	switch {
	case i < 0:
		return ""
	case i == 0:
		return "/"
	}
	return strings.TrimRight(p[:i], "/")
}

// ToRunnerOptions converts these options to runner options, to run the file
// filename from the directory currentDir. The error is a CliError.
func (o *CliOptions) ToRunnerOptions(filename input.Input, currentDir string) (runner.RunnerOptions, error) {
	var fileRoot string
	switch {
	case o.FileRoot != nil:
		fileRoot = *o.FileRoot
	default:
		if f, ok := filename.Kind().(input.File); ok {
			fileRoot = rustParent(f.Path)
		} else {
			fileRoot = currentDir
		}
	}

	followLocation := httpclient.FollowLocation{}
	if o.FollowLocation {
		followLocation.Follow = true
		if o.FollowLocationTrusted {
			followLocation.Forwarding = httpclient.AllHosts
		}
	}

	var headers httpclient.HeaderVec
	for _, h := range o.Headers {
		header, ok := httpclient.ParseHeader(h)
		if !ok {
			return runner.RunnerOptions{}, CliErrorInvalidOption{Message: "Invalid header <" + h + ">, missing `:`"}
		}
		headers.Push(header)
	}
	for _, h := range o.NoHeaders {
		if strings.TrimSpace(h) == "" {
			return runner.RunnerOptions{}, CliErrorInvalidOption{Message: "Missing header name"}
		}
	}
	var proxyHeaders httpclient.HeaderVec
	for _, h := range o.ProxyHeaders {
		header, ok := httpclient.ParseHeader(h)
		if !ok {
			return runner.RunnerOptions{}, CliErrorInvalidOption{Message: "Invalid proxy header <" + h + ">, missing `:`"}
		}
		proxyHeaders.Push(header)
	}

	r := runner.DefaultRunnerOptions()
	r.AwsSigv4 = o.AwsSigv4
	r.CacertFile = o.CacertFile
	r.ClientCertFile = o.ClientCertFile
	r.ClientKeyFile = o.ClientKeyFile
	r.Delay = o.Delay
	r.Digest = o.Digest
	r.ColorStdout = o.ColorStdout.Get()
	r.Compressed = o.Compressed
	r.ConnectTimeout = o.ConnectTimeout
	r.ConnectsTo = append([]string(nil), o.ConnectsTo...)
	r.ContinueOnError = o.ContinueOnError
	r.ContextDir = util.NewContextDir(currentDir, fileRoot)
	r.CookieInputFile = o.CookieInputFile
	r.FailWithBody = o.FailWithBody
	r.FollowLocation = followLocation
	r.FromEntry = o.FromEntry
	r.Headers = headers
	r.HttpVersion = o.HttpVersion.requested()
	r.Insecure = o.Insecure
	r.IpResolve = o.IpResolve.client()
	r.MaxFilesize = o.MaxFilesize
	// Like curl, there's a single limit rate for downloads and uploads.
	if o.LimitRate != nil {
		r.MaxRecvSpeed = *o.LimitRate
		r.MaxSendSpeed = *o.LimitRate
	}
	r.MaxRedirect = o.MaxRedirect
	r.Negotiate = o.Negotiate
	r.Netrc = o.Netrc
	r.NetrcFile = o.NetrcFile
	r.NetrcOptional = o.NetrcOptional
	r.NoAssert = o.NoAssert
	r.UseJSONPathCoercion = !o.NoJSONPathCoercion
	r.NoProxy = o.NoProxy
	// As Hurl, the header names are validated but passed untrimmed.
	r.NoHeaders = append([]string(nil), o.NoHeaders...)
	r.Ntlm = o.Ntlm
	r.Output = o.Output
	r.PathAsIs = o.PathAsIs
	r.PinnedPubKey = o.PinnedPubKey
	r.Pretty = o.Pretty
	r.Proxy = o.Proxy
	r.ProxyHeaders = proxyHeaders
	r.Resolves = append([]string(nil), o.Resolves...)
	r.Retry = o.Retry
	r.RetryInterval = o.RetryInterval
	r.SslNoRevoke = o.SslNoRevoke
	r.Timeout = o.Timeout
	r.TlsMax = o.TlsMax.client()
	r.TlsMin = o.TlsMin.client()
	r.ToEntry = o.ToEntry
	r.UnixSocket = o.UnixSocket
	r.UseCookieStore = !o.NoCookieStore
	r.User = o.User
	r.UserAgent = o.UserAgent
	return r, nil
}

// ToLoggerOptions converts these options to logger options.
func (o *CliOptions) ToLoggerOptions() logger.LoggerOptions {
	verbosity := logger.VerbosityNone
	switch o.Verbosity {
	case VerbosityBrief:
		verbosity = logger.VerbosityLowVerbose
	case VerbosityVerbose:
		verbosity = logger.VerbosityVerbose
	case VerbosityDebug:
		verbosity = logger.VerbosityVeryVerbose
	}
	errorFormat := logger.ErrorFormatShort
	if o.ErrorFormat == ErrorFormatLong {
		errorFormat = logger.ErrorFormatLong
	}
	return logger.LoggerOptions{
		Color:       o.ColorStderr.Get(),
		ErrorFormat: errorFormat,
		Verbosity:   verbosity,
	}
}
