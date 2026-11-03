package cli

import (
	"math"
	"strings"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/pretty"
)

// EnvVars holds all the environment variables at the start of the program.
type EnvVars struct {
	all map[string]string
	// hurl are the variables with the HURL_ prefix.
	hurl map[string]string
}

const (
	hurlPrefix          = "HURL_"
	hurlSecretPrefix    = "HURL_SECRET_"
	hurlVariablePrefix  = "HURL_VARIABLE_"
	hurlConnectTimeout  = "HURL_CONNECT_TIMEOUT"
	hurlDelay           = "HURL_DELAY"
	hurlErrorFormat     = "HURL_ERROR_FORMAT"
	hurlHeader          = "HURL_HEADER"
	hurlJobs            = "HURL_JOBS"
	hurlLimitRate       = "HURL_LIMIT_RATE"
	hurlLocation        = "HURL_LOCATION"
	hurlLocationTrusted = "HURL_LOCATION_TRUSTED"
	hurlMaxFilesize     = "HURL_MAX_FILESIZE"
	hurlMaxRedirs       = "HURL_MAX_REDIRS"
	hurlMaxTime         = "HURL_MAX_TIME"
	hurlNoHeader        = "HURL_NO_HEADER"
	hurlProxyHeader     = "HURL_PROXY_HEADER"
	hurlRetry           = "HURL_RETRY"
	hurlRetryInterval   = "HURL_RETRY_INTERVAL"
	hurlTlsMax          = "HURL_TLS_MAX"
	hurlTlsMin          = "HURL_TLS_MIN"
	hurlVerbosity       = "HURL_VERBOSITY"
)

// NewEnvVars captures the environment variables env.
func NewEnvVars(env map[string]string) *EnvVars {
	hurl := map[string]string{}
	for k, v := range env {
		if strings.HasPrefix(k, hurlPrefix) {
			hurl[k] = v
		}
	}
	return &EnvVars{all: env, hurl: hurl}
}

func (e *EnvVars) Home() (string, bool) {
	v, ok := e.all["HOME"]
	return v, ok
}

func (e *EnvVars) XdgConfigHome() (string, bool) {
	v, ok := e.all["XDG_CONFIG_HOME"]
	return v, ok
}

// IsCI reports whether we run in a CI context (GitHub Actions, GitLab
// CI/CD...).
func (e *EnvVars) IsCI() bool {
	_, ci := e.all["CI"]
	_, tf := e.all["TF_BUILD"]
	return ci || tf
}

func (e *EnvVars) get(name string) (string, bool) {
	v, ok := e.hurl[name]
	return v, ok
}

// getBool returns the boolean value of name: "1" or "true", "0" or "false"
// (case insensitive), and false if unset or invalid.
func (e *EnvVars) getBool(name string) (value, ok bool) {
	v, found := e.hurl[name]
	if !found {
		return false, false
	}
	switch strings.ToLower(v) {
	case "1", "true":
		return true, true
	case "0", "false":
		return false, true
	}
	return false, false
}

// color returns the value of HURL_COLOR.
func (e *EnvVars) color() (bool, bool) { return e.getBool("HURL_COLOR") }

// noColor returns true if NO_COLOR is set (and not empty), else the value
// of HURL_NO_COLOR.
func (e *EnvVars) noColor() (bool, bool) {
	if v, ok := e.all["NO_COLOR"]; ok {
		// According to the NO_COLOR spec, any presence of the variable should
		// disable color, but Hurl < 7.1.0 required it to be non empty.
		if v != "" {
			return true, true
		}
		return false, false
	}
	return e.getBool("HURL_NO_COLOR")
}

// prefixed returns the variables with the given prefix, keyed by their
// stripped (non empty) names.
func (e *EnvVars) prefixed(prefix string) map[string]string {
	out := map[string]string{}
	for name, value := range e.hurl {
		if stripped, ok := strings.CutPrefix(name, prefix); ok && stripped != "" {
			out[stripped] = value
		}
	}
	return out
}

// variables returns the HURL_VARIABLE_foo variables, keyed by foo.
func (e *EnvVars) variables() map[string]string { return e.prefixed(hurlVariablePrefix) }

// secrets returns the HURL_SECRET_foo variables, keyed by foo.
func (e *EnvVars) secrets() map[string]string { return e.prefixed(hurlSecretPrefix) }

func errFromEnv(err error, env string) error {
	if e, ok := err.(OptionsErrorError); ok {
		return OptionsErrorError{Message: e.Message + " (" + env + " environment variable)"}
	}
	if _, ok := err.(OptionsError); ok {
		return err
	}
	return OptionsErrorError{Message: err.Error() + " (" + env + " environment variable)"}
}

// parseEnvVars updates options with the Hurl configuration in environment
// variables.
func parseEnvVars(env *EnvVars, o *CliOptions) error {
	if noColor, ok := env.noColor(); ok {
		o.ColorStdout, o.ColorStderr = boolOptSet(!noColor), boolOptSet(!noColor)
	} else if color, ok := env.color(); ok {
		o.ColorStdout, o.ColorStderr = boolOptSet(color), boolOptSet(color)
	}
	if v, ok := env.getBool("HURL_COMPRESSED"); ok {
		o.Compressed = v
	}
	if v, ok := env.get(hurlConnectTimeout); ok {
		d, err := durationFromStr(v, ast.DurationUnitSecond)
		if err != nil {
			return errFromEnv(err, hurlConnectTimeout)
		}
		o.ConnectTimeout = d
	}
	if v, ok := env.getBool("HURL_FAIL_WITH_BODY"); ok {
		o.FailWithBody = v
	}
	if v, ok := env.getBool("HURL_CONTINUE_ON_ERROR"); ok {
		o.ContinueOnError = v
	}
	if v, ok := env.get(hurlDelay); ok {
		d, err := durationFromStr(v, ast.DurationUnitMilliSecond)
		if err != nil {
			return errFromEnv(err, hurlDelay)
		}
		o.Delay = d
	}
	if v, ok := env.get(hurlErrorFormat); ok {
		f, err := parseErrorFormat(v)
		if err != nil {
			return errFromEnv(err, hurlErrorFormat)
		}
		o.ErrorFormat = f
	}
	if v, ok := env.get(hurlHeader); ok {
		headers := strings.Split(v, "|")
		for _, h := range headers {
			if !strings.Contains(h, ":") {
				return errFromEnv(optionsErrorf("Invalid header <%s>, missing `:`", h), hurlHeader)
			}
		}
		o.Headers = append(o.Headers, headers...)
	}
	o.HttpVersion = envHttpVersion(env, o.HttpVersion)
	o.IpResolve = envIpResolve(env, o.IpResolve)
	if v, ok := env.getBool("HURL_NO_ASSERT"); ok {
		o.NoAssert = v
	}
	if v, ok := env.getBool("HURL_NO_COOKIE_STORE"); ok {
		o.NoCookieStore = v
	}
	if v, ok := env.get(hurlNoHeader); ok {
		var noHeaders []string
		for h := range strings.SplitSeq(v, "|") {
			h = strings.TrimSpace(h)
			if h == "" {
				return errFromEnv(optionsErrorf("Missing header name"), hurlNoHeader)
			}
			noHeaders = append(noHeaders, h)
		}
		o.NoHeaders = append(o.NoHeaders, noHeaders...)
	}
	if v, ok := env.getBool("HURL_NO_JSONPATH_COERCION"); ok {
		o.NoJSONPathCoercion = v
	}
	if v, ok := env.getBool("HURL_NO_OUTPUT"); ok && v {
		o.OutputType = OutputTypeNoOutput
	}

	location, hasLocation := env.getBool(hurlLocation)
	trusted, hasTrusted := env.getBool(hurlLocationTrusted)
	switch {
	case hasLocation && location:
		o.FollowLocation = true
	case hasLocation && hasTrusted && trusted:
		return optionsErrorf("Invalid environment variables configuration %s %s", hurlLocation, hurlLocationTrusted)
	case hasLocation:
		o.FollowLocation = false
	case hasTrusted && trusted:
		o.FollowLocation = true
	}
	if hasTrusted {
		o.FollowLocationTrusted = trusted
	}

	if v, ok := env.getBool("HURL_INSECURE"); ok {
		o.Insecure = v
	}
	if v, ok := env.get(hurlJobs); ok {
		n, err := rustParseInt(v, 0, math.MaxInt64)
		if err != nil {
			return errFromEnv(err, hurlJobs)
		}
		o.Jobs = int(n)
	}
	if v, ok := env.get(hurlLimitRate); ok {
		n, err := rustParseUint64(v)
		if err != nil {
			return errFromEnv(err, hurlLimitRate)
		}
		o.LimitRate = &n
	}
	if v, ok := env.get(hurlMaxFilesize); ok {
		n, err := rustParseUint64(v)
		if err != nil {
			return errFromEnv(err, hurlMaxFilesize)
		}
		o.MaxFilesize = &n
	}
	if v, ok := env.get(hurlMaxRedirs); ok {
		c, err := parseCount(v)
		if err != nil {
			return errFromEnv(err, hurlMaxRedirs)
		}
		o.MaxRedirect = c
	}
	if v, ok := env.getBool("HURL_PARALLEL"); ok {
		o.Parallel = boolOptSet(v)
	}
	pv, hasPretty := env.getBool("HURL_PRETTY")
	npv, hasNoPretty := env.getBool("HURL_NO_PRETTY")
	switch {
	case hasPretty && pv:
		o.Pretty = pretty.Force
	case hasPretty:
		o.Pretty = pretty.None
	case hasNoPretty && npv:
		o.Pretty = pretty.None
	case hasNoPretty:
		o.Pretty = pretty.Force
	}
	if v, ok := env.getBool("HURL_PROGRESS_BAR"); ok {
		o.ProgressBar = boolOptSet(v)
	}
	if v, ok := env.get(hurlProxyHeader); ok {
		headers := strings.Split(v, "|")
		for _, h := range headers {
			if !strings.Contains(h, ":") {
				return errFromEnv(optionsErrorf("Invalid proxy header <%s>, missing `:`", h), hurlProxyHeader)
			}
		}
		o.ProxyHeaders = append(o.ProxyHeaders, headers...)
	}
	if v, ok := env.get(hurlRetry); ok {
		c, err := parseCount(v)
		if err != nil {
			return errFromEnv(err, hurlRetry)
		}
		o.Retry = c
	}
	if v, ok := env.get(hurlRetryInterval); ok {
		d, err := durationFromStr(v, ast.DurationUnitMilliSecond)
		if err != nil {
			return errFromEnv(err, hurlRetryInterval)
		}
		o.RetryInterval = d
	}
	for name, value := range env.secrets() {
		v, err := parseVariableValue(value, typeString)
		if err != nil {
			return err
		}
		if err := addSecret(o.Secrets, name, v); err != nil {
			return err
		}
	}
	if v, ok := env.get(hurlMaxTime); ok {
		d, err := durationFromStr(v, ast.DurationUnitSecond)
		if err != nil {
			return errFromEnv(err, hurlMaxTime)
		}
		o.Timeout = d
	}
	if v, ok := env.get(hurlTlsMax); ok {
		t, msg := parseTlsVersion(v)
		if msg != "" {
			return errFromEnv(optionsErrorf("%s", msg), hurlTlsMax)
		}
		o.TlsMax = t
	}
	if v, ok := env.get(hurlTlsMin); ok {
		t, msg := parseTlsVersion(v)
		if msg != "" {
			return errFromEnv(optionsErrorf("%s", msg), hurlTlsMin)
		}
		o.TlsMin = t
	}
	if v, ok := env.get("HURL_USER"); ok {
		o.User = &v
	}
	if v, ok := env.get("HURL_USER_AGENT"); ok {
		o.UserAgent = &v
	}
	for name, value := range env.variables() {
		v, err := parseVariableValue(value, typeInferred)
		if err != nil {
			return err
		}
		o.Variables[name] = v
	}
	if v, ok := env.getBool("HURL_VERBOSE"); ok && v {
		o.Verbosity = VerbosityVerbose
	} else if v, ok := env.getBool("HURL_VERY_VERBOSE"); ok && v {
		o.Verbosity = VerbosityDebug
	} else if v, ok := env.get(hurlVerbosity); ok {
		verbosity, err := parseVerbosity(v)
		if err != nil {
			return errFromEnv(err, hurlVerbosity)
		}
		o.Verbosity = verbosity
	}
	if v, ok := env.getBool("HURL_TEST"); ok {
		o.Test = v
	}
	return nil
}

// parseCount parses an i32 count, -1 being infinite.
func parseCount(s string) (ast.Count, error) {
	n, err := rustParseInt(s, math.MinInt32, math.MaxInt32)
	if err != nil {
		return nil, err
	}
	return ast.CountFromInt32(int32(n))
}

func envHttpVersion(env *EnvVars, def HttpVersion) HttpVersion {
	if v, ok := env.getBool("HURL_HTTP3"); ok {
		if v {
			return HttpVersionV3
		}
		return HttpVersionV2
	}
	if v, ok := env.getBool("HURL_HTTP2_PRIOR_KNOWLEDGE"); ok {
		if v {
			return HttpVersionV2PriorKnowledge
		}
		return HttpVersionV11
	}
	if v, ok := env.getBool("HURL_HTTP2"); ok {
		if v {
			return HttpVersionV2
		}
		return HttpVersionV11
	}
	if v, ok := env.getBool("HURL_HTTP11"); ok {
		if v {
			return HttpVersionV11
		}
		return HttpVersionV10
	}
	if v, ok := env.getBool("HURL_HTTP10"); ok && v {
		return HttpVersionV10
	}
	return def
}

func envIpResolve(env *EnvVars, def IpResolve) IpResolve {
	if v, ok := env.getBool("HURL_IPV6"); ok {
		if v {
			return IpResolveIpV6
		}
		return IpResolveIpV4
	}
	if v, ok := env.getBool("HURL_IPV4"); ok {
		if v {
			return IpResolveIpV4
		}
		return IpResolveIpV6
	}
	return def
}
