package runner

import (
	"fmt"
	"time"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/httpclient"
	"github.com/aviddiviner/go-hurl/internal/logger"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// entryOptions returns new runner options from the optional [Options]
// section of entry and the default runnerOptions. variables are updated by
// variable options.
func entryOptions(entry *ast.Entry, runnerOptions RunnerOptions, variables *VariableSet, l *logger.Logger) (RunnerOptions, *RunnerError) {
	// When used globally (on the command line), --output writes the last
	// successful response; entries don't output their response by default.
	o := runnerOptions.clone()
	o.Output = nil
	options := entry.Request.Options()
	if len(options) == 0 {
		return o, nil
	}

	l.Debug("")
	l.DebugImportant("Entry options:")

	for _, option := range options {
		var err *RunnerError
		switch k := option.Kind.(type) {
		case ast.OptionAWSSigV4:
			o.AwsSigv4, err = evalTemplate(k.Value, variables)
		case ast.OptionCACertificate:
			o.CacertFile, err = evalTemplate(k.Value, variables)
		case ast.OptionClientCert:
			o.ClientCertFile, err = evalTemplate(k.Value, variables)
		case ast.OptionClientKey:
			o.ClientKeyFile, err = evalTemplate(k.Value, variables)
		case ast.OptionCompressed:
			o.Compressed, err = evalBooleanOption(k.Value, variables)
		case ast.OptionConnectTo:
			var v string
			if v, err = evalTemplate(k.Value, variables); err == nil {
				o.ConnectsTo = append(o.ConnectsTo, v)
			}
		case ast.OptionConnectTimeout:
			o.ConnectTimeout, err = evalDurationOption(k.Value, variables, ast.DurationUnitMilliSecond)
		case ast.OptionDelay:
			o.Delay, err = evalDurationOption(k.Value, variables, ast.DurationUnitMilliSecond)
		case ast.OptionDigest:
			o.Digest, err = evalBooleanOption(k.Value, variables)
		case ast.OptionHeader:
			var h httpclient.Header
			if h, err = evalHeaderOption(k.Value, variables); err == nil {
				o.Headers.Push(h)
			}
		// HTTP version options are flags: true is the same as the command
		// line option, false downgrades to the lower HTTP version. As libcurl
		// prioritizes reusing connections over the requested HTTP version,
		// connections aren't reused when a version is set per request.
		case ast.OptionHTTP10:
			err = evalHTTPVersionOption(&o, k.Value, variables, httpclient.RequestedHttpVersionHttp10, o.HttpVersion)
		case ast.OptionHTTP11:
			err = evalHTTPVersionOption(&o, k.Value, variables, httpclient.RequestedHttpVersionHttp11, httpclient.RequestedHttpVersionHttp10)
		case ast.OptionHTTP2:
			err = evalHTTPVersionOption(&o, k.Value, variables, httpclient.RequestedHttpVersionHttp2, httpclient.RequestedHttpVersionHttp11)
		case ast.OptionHTTP2PriorKnowledge:
			err = evalHTTPVersionOption(&o, k.Value, variables, httpclient.RequestedHttpVersionHttp2PriorKnowledge, httpclient.RequestedHttpVersionHttp11)
		case ast.OptionHTTP3:
			err = evalHTTPVersionOption(&o, k.Value, variables, httpclient.RequestedHttpVersionHttp3, httpclient.RequestedHttpVersionHttp2)
		case ast.OptionFailWithBody:
			o.FailWithBody, err = evalBooleanOption(k.Value, variables)
		case ast.OptionFollowLocation:
			var v bool
			if v, err = evalBooleanOption(k.Value, variables); err == nil {
				o.FollowLocation = httpclient.FollowLocation{Follow: v, Forwarding: httpclient.OnlyInitialHost}
			}
		case ast.OptionFollowLocationTrusted:
			var v bool
			if v, err = evalBooleanOption(k.Value, variables); err == nil && v {
				o.FollowLocation = httpclient.FollowLocation{Follow: true, Forwarding: httpclient.AllHosts}
			}
		case ast.OptionInsecure:
			o.Insecure, err = evalBooleanOption(k.Value, variables)
		case ast.OptionIPv4:
			var v bool
			if v, err = evalBooleanOption(k.Value, variables); err == nil {
				o.IpResolve = ipResolve(v, httpclient.IpResolveIpV4, httpclient.IpResolveIpV6)
			}
		case ast.OptionIPv6:
			var v bool
			if v, err = evalBooleanOption(k.Value, variables); err == nil {
				o.IpResolve = ipResolve(v, httpclient.IpResolveIpV6, httpclient.IpResolveIpV4)
			}
		case ast.OptionLimitRate:
			var v uint64
			if v, err = evalNaturalOption(k.Value, variables); err == nil {
				o.MaxSendSpeed = v
				o.MaxRecvSpeed = v
			}
		case ast.OptionMaxRedirect:
			o.MaxRedirect, err = evalCountOption(k.Value, variables)
		case ast.OptionMaxTime:
			o.Timeout, err = evalDurationOption(k.Value, variables, ast.DurationUnitMilliSecond)
		case ast.OptionNegotiate:
			o.Negotiate, err = evalBooleanOption(k.Value, variables)
		case ast.OptionNetRc:
			o.Netrc, err = evalBooleanOption(k.Value, variables)
		case ast.OptionNetRcFile:
			o.NetrcFile, err = evalTemplate(k.Value, variables)
		case ast.OptionNetRcOptional:
			o.NetrcOptional, err = evalBooleanOption(k.Value, variables)
		case ast.OptionNoHeader:
			var v string
			if v, err = evalTemplate(k.Value, variables); err == nil {
				o.NoHeaders = append(o.NoHeaders, v)
			}
		case ast.OptionNoJSONPathCoercion:
			var v bool
			if v, err = evalBooleanOption(k.Value, variables); err == nil {
				o.UseJSONPathCoercion = !v
			}
		case ast.OptionNtlm:
			o.Ntlm, err = evalBooleanOption(k.Value, variables)
		case ast.OptionOutput:
			var filename string
			if filename, err = evalTemplate(k.Value, variables); err == nil {
				output := util.NewOutput(filename)
				// Pretty printing is disabled when writing to a file.
				if _, ok := output.(util.OutputFile); ok {
					o.Pretty = pretty.None
				}
				o.Output = output
			}
		case ast.OptionPathAsIs:
			o.PathAsIs, err = evalBooleanOption(k.Value, variables)
		case ast.OptionPinnedPublicKey:
			o.PinnedPubKey, err = evalTemplate(k.Value, variables)
		case ast.OptionProxy:
			o.Proxy, err = evalTemplate(k.Value, variables)
		case ast.OptionRepeat:
			o.Repeat, err = evalCountOption(k.Value, variables)
		case ast.OptionResolve:
			var v string
			if v, err = evalTemplate(k.Value, variables); err == nil {
				o.Resolves = append(o.Resolves, v)
			}
		case ast.OptionRetry:
			o.Retry, err = evalCountOption(k.Value, variables)
		case ast.OptionRetryInterval:
			o.RetryInterval, err = evalDurationOption(k.Value, variables, ast.DurationUnitMilliSecond)
		case ast.OptionSkip:
			o.Skip, err = evalBooleanOption(k.Value, variables)
		case ast.OptionTLSMax:
			o.TlsMax, err = evalTLSVersionOption(k.Value, "tls-max", variables)
		case ast.OptionTLSMin:
			o.TlsMin, err = evalTLSVersionOption(k.Value, "tls-min", variables)
		case ast.OptionUnixSocket:
			o.UnixSocket, err = evalTemplate(k.Value, variables)
		case ast.OptionUser:
			var v string
			if v, err = evalTemplate(k.Value, variables); err == nil {
				o.User = &v
			}
		case ast.OptionVariable:
			var v Value
			if v, err = evalVariableValue(k.Value.Value, variables); err == nil {
				variables.Insert(k.Value.Name, v)
			}
		case ast.OptionVerbosity:
			// Verbosity options are processed earlier, as they impact logging;
			// we only check their templates here.
		case ast.OptionVerbose:
			_, err = evalBooleanOption(k.Value, variables)
		case ast.OptionVeryVerbose:
			_, err = evalBooleanOption(k.Value, variables)
		}
		if err != nil {
			return o, err
		}
		l.Debug(option.Kind.String())
	}
	return o, nil
}

func evalHTTPVersionOption(o *RunnerOptions, value ast.BooleanOption, variables *VariableSet, version, downgrade httpclient.RequestedHttpVersion) *RunnerError {
	v, err := evalBooleanOption(value, variables)
	if err != nil {
		return err
	}
	if v {
		o.HttpVersion = version
	} else {
		o.HttpVersion = downgrade
	}
	o.AllowReuse = false
	return nil
}

func ipResolve(v bool, ifTrue, ifFalse httpclient.IpResolve) httpclient.IpResolve {
	if v {
		return ifTrue
	}
	return ifFalse
}

func evalTLSVersionOption(value ast.Template, name string, variables *VariableSet) (httpclient.TlsVersion, *RunnerError) {
	s, err := evalTemplate(value, variables)
	if err != nil {
		return httpclient.TlsVersionDefault, err
	}
	v, perr := httpclient.ParseTlsVersion(s)
	if perr != nil {
		kind := RunnerErrorInvalidOptionValue{Name: name, Value: s, Message: perr.Error()}
		return httpclient.TlsVersionDefault, NewRunnerError(value.SourceInfo, kind, false)
	}
	return v, nil
}

// entryVerbosity returns the verbosity of entry, overridden by its options,
// or defaultVerbosity.
func entryVerbosity(entry *ast.Entry, defaultVerbosity logger.Verbosity, variables *VariableSet) (logger.Verbosity, *RunnerError) {
	verbosity := defaultVerbosity
	for _, option := range entry.Request.Options() {
		switch k := option.Kind.(type) {
		case ast.OptionVerbosity:
			switch k.Value {
			case ast.VerbosityBrief:
				verbosity = logger.VerbosityLowVerbose
			case ast.VerbosityVerbose:
				verbosity = logger.VerbosityVerbose
			case ast.VerbosityDebug:
				verbosity = logger.VerbosityVeryVerbose
			}
		case ast.OptionVerbose:
			v, err := evalBooleanOption(k.Value, variables)
			if err != nil {
				return verbosity, err
			}
			verbosity = logger.VerbosityNone
			if v {
				verbosity = logger.VerbosityVerbose
			}
		case ast.OptionVeryVerbose:
			v, err := evalBooleanOption(k.Value, variables)
			if err != nil {
				return verbosity, err
			}
			verbosity = logger.VerbosityNone
			if v {
				verbosity = logger.VerbosityVeryVerbose
			}
		}
	}
	return verbosity, nil
}

// evalHeaderOption evaluates a template into a header.
func evalHeaderOption(header ast.Template, variables *VariableSet) (httpclient.Header, *RunnerError) {
	s, err := evalTemplate(header, variables)
	if err != nil {
		return httpclient.Header{}, err
	}
	h, ok := httpclient.ParseHeader(s)
	if !ok {
		kind := RunnerErrorInvalidOptionValue{Name: "header", Value: s, Message: "missing `:`"}
		return httpclient.Header{}, NewRunnerError(header.SourceInfo, kind, false)
	}
	return h, nil
}

func invalidType(expr ast.Expr, value, expecting string) *RunnerError {
	return NewRunnerError(expr.SourceInfo, RunnerErrorExpressionInvalidType{Value: value, Expecting: expecting}, false)
}

func evalBooleanOption(option ast.BooleanOption, variables *VariableSet) (bool, *RunnerError) {
	switch o := option.(type) {
	case ast.BooleanLiteral:
		return o.Value, nil
	case ast.BooleanPlaceholder:
		v, err := evalExpr(o.Value.Expr, variables)
		if err != nil {
			return false, err
		}
		if b, ok := v.(ValueBool); ok {
			return b.Value, nil
		}
		return false, invalidType(o.Value.Expr, Repr(v), "boolean")
	}
	panic("unknown boolean option")
}

func evalNaturalOption(option ast.NaturalOption, variables *VariableSet) (uint64, *RunnerError) {
	switch o := option.(type) {
	case ast.NaturalLiteral:
		return o.Value.Value, nil
	case ast.NaturalPlaceholder:
		v, err := evalExpr(o.Value.Expr, variables)
		if err != nil {
			return 0, err
		}
		n, ok := integerValue(v)
		if !ok {
			return 0, invalidType(o.Value.Expr, Repr(v), "integer")
		}
		if n <= 0 {
			return 0, invalidType(o.Value.Expr, fmt.Sprintf("integer <%d>", n), "integer > 0")
		}
		return uint64(n), nil
	}
	panic("unknown natural option")
}

func evalCountOption(option ast.CountOption, variables *VariableSet) (ast.Count, *RunnerError) {
	switch o := option.(type) {
	case ast.CountLiteral:
		return o.Value, nil
	case ast.CountPlaceholder:
		v, err := evalExpr(o.Value.Expr, variables)
		if err != nil {
			return nil, err
		}
		n, ok := integerValue(v)
		switch {
		case !ok:
			return nil, invalidType(o.Value.Expr, Repr(v), "integer")
		case n == -1:
			return ast.CountInfinite{}, nil
		case n >= 0:
			return ast.CountFinite{Value: int(n)}, nil
		}
		return nil, invalidType(o.Value.Expr, fmt.Sprintf("integer <%d>", n), "integer >= -1")
	}
	panic("unknown count option")
}

func durationUnitMillis(unit ast.DurationUnit) uint64 {
	switch unit {
	case ast.DurationUnitSecond:
		return 1000
	case ast.DurationUnitMinute:
		return 1000 * 60
	case ast.DurationUnitHour:
		return 1000 * 60 * 60
	}
	return 1
}

// evalDurationOption evaluates a duration option, using defaultUnit when
// the unit isn't explicit.
func evalDurationOption(option ast.DurationOption, variables *VariableSet, defaultUnit ast.DurationUnit) (time.Duration, *RunnerError) {
	var millis uint64
	switch o := option.(type) {
	case ast.DurationLiteral:
		unit := defaultUnit
		if o.Value.Unit != nil {
			unit = *o.Value.Unit
		}
		millis = o.Value.Value.Value * durationUnitMillis(unit)
	case ast.DurationPlaceholder:
		v, err := evalExpr(o.Value.Expr, variables)
		if err != nil {
			return 0, err
		}
		n, ok := integerValue(v)
		if !ok {
			return 0, invalidType(o.Value.Expr, Repr(v), "positive integer")
		}
		if n < 0 {
			return 0, invalidType(o.Value.Expr, fmt.Sprintf("integer <%d>", n), "positive integer")
		}
		millis = uint64(n) * durationUnitMillis(defaultUnit)
	}
	return time.Duration(millis) * time.Millisecond, nil
}

func integerValue(v Value) (int64, bool) {
	n, ok := v.(ValueNumber)
	if !ok {
		return 0, false
	}
	i, ok := n.Value.(NumberInteger)
	return i.Value, ok
}

func evalVariableValue(value ast.VariableValue, variables *VariableSet) (Value, *RunnerError) {
	switch v := value.(type) {
	case ast.VariableNull:
		return ValueNull{}, nil
	case ast.VariableBool:
		return NewBool(v.Value), nil
	case ast.VariableNumber:
		return ValueNumber{evalASTNumber(v.Value)}, nil
	case ast.VariableString:
		s, err := evalTemplate(v.Value, variables)
		if err != nil {
			return nil, err
		}
		return NewString(s), nil
	}
	panic("unknown variable value")
}
