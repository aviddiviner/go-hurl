package parser

import (
	"fmt"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

// entryOption parses an option in an [Options] section.
func entryOption(r *reader.Reader) (ast.EntryOption, *ParseError) {
	lts, err := optionalLineTerminators(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	start := r.Cursor()
	// '_' is accepted even though no option name has it, so that we get
	// into the option name check and a better error ('max_redirs' vs
	// 'max-redirs').
	option := r.ReadWhile(func(c rune) bool { return isASCIIAlphanumeric(c) || c == '-' || c == '.' || c == '_' })
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	if err := tryLiteral(":", r); err != nil {
		return ast.EntryOption{}, err
	}
	space2, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	parse, ok := optionParsers[option]
	if !ok {
		return ast.EntryOption{}, newError(start.Pos, false, ParseErrorInvalidOption{Value: option})
	}
	kind, err := parse(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	lt0, err := lineTerminator(r)
	if err != nil {
		return ast.EntryOption{}, err
	}
	return ast.EntryOption{
		LineTerminators: lts,
		Space0:          space0,
		Space1:          space1,
		Space2:          space2,
		Kind:            kind,
		LineTerminator0: lt0,
	}, nil
}

type optionFunc = func(*reader.Reader) (ast.OptionKind, *ParseError)

// optionOf adapts a value parser f into an option parser.
func optionOf[T any](f func(*reader.Reader) (T, *ParseError), kind func(T) ast.OptionKind) optionFunc {
	return func(r *reader.Reader) (ast.OptionKind, *ParseError) {
		v, err := f(r)
		if err != nil {
			return nil, err
		}
		return kind(v), nil
	}
}

func nonRecoverable[T any](f func(*reader.Reader) (T, *ParseError)) func(*reader.Reader) (T, *ParseError) {
	return func(r *reader.Reader) (T, *ParseError) { return nonRecover(f, r) }
}

var optionParsers map[string]optionFunc

func init() {
	boolOpt := nonRecoverable(booleanOption)
	countOpt := nonRecoverable(countOption)
	optionParsers = map[string]optionFunc{
		"aws-sigv4":       optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionAWSSigV4{Value: v} }),
		"cacert":          optionOf(filename, func(v ast.Template) ast.OptionKind { return ast.OptionCACertificate{Value: v} }),
		"cert":            optionOf(filenamePassword, func(v ast.Template) ast.OptionKind { return ast.OptionClientCert{Value: v} }),
		"compressed":      optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionCompressed{Value: v} }),
		"connect-to":      optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionConnectTo{Value: v} }),
		"connect-timeout": optionOf(durationOption, func(v ast.DurationOption) ast.OptionKind { return ast.OptionConnectTimeout{Value: v} }),
		"delay":           optionOf(durationOption, func(v ast.DurationOption) ast.OptionKind { return ast.OptionDelay{Value: v} }),
		"digest":          optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionDigest{Value: v} }),
		"fail-with-body":  optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionFailWithBody{Value: v} }),
		"insecure":        optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionInsecure{Value: v} }),
		"header":          optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionHeader{Value: v} }),
		"http1.0":         optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionHTTP10{Value: v} }),
		"http1.1":         optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionHTTP11{Value: v} }),
		"http2-prior-knowledge": optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind {
			return ast.OptionHTTP2PriorKnowledge{Value: v}
		}),
		"http2":      optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionHTTP2{Value: v} }),
		"http3":      optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionHTTP3{Value: v} }),
		"ipv4":       optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionIPv4{Value: v} }),
		"ipv6":       optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionIPv6{Value: v} }),
		"key":        optionOf(filename, func(v ast.Template) ast.OptionKind { return ast.OptionClientKey{Value: v} }),
		"limit-rate": optionOf(nonRecoverable(naturalOption), func(v ast.NaturalOption) ast.OptionKind { return ast.OptionLimitRate{Value: v} }),
		"location":   optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionFollowLocation{Value: v} }),
		"location-trusted": optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind {
			return ast.OptionFollowLocationTrusted{Value: v}
		}),
		"max-redirs":     optionOf(countOpt, func(v ast.CountOption) ast.OptionKind { return ast.OptionMaxRedirect{Value: v} }),
		"max-time":       optionOf(durationOption, func(v ast.DurationOption) ast.OptionKind { return ast.OptionMaxTime{Value: v} }),
		"negotiate":      optionOf(booleanOption, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionNegotiate{Value: v} }),
		"netrc":          optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionNetRc{Value: v} }),
		"netrc-file":     optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionNetRcFile{Value: v} }),
		"netrc-optional": optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionNetRcOptional{Value: v} }),
		"no-header":      optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionNoHeader{Value: v} }),
		"no-jsonpath-coercion": optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind {
			return ast.OptionNoJSONPathCoercion{Value: v}
		}),
		"ntlm":           optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionNtlm{Value: v} }),
		"output":         optionOf(filename, func(v ast.Template) ast.OptionKind { return ast.OptionOutput{Value: v} }),
		"path-as-is":     optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionPathAsIs{Value: v} }),
		"pinnedpubkey":   optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionPinnedPublicKey{Value: v} }),
		"proxy":          optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionProxy{Value: v} }),
		"repeat":         optionOf(countOpt, func(v ast.CountOption) ast.OptionKind { return ast.OptionRepeat{Value: v} }),
		"resolve":        optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionResolve{Value: v} }),
		"retry":          optionOf(countOpt, func(v ast.CountOption) ast.OptionKind { return ast.OptionRetry{Value: v} }),
		"retry-interval": optionOf(nonRecoverable(durationOption), func(v ast.DurationOption) ast.OptionKind { return ast.OptionRetryInterval{Value: v} }),
		"skip":           optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionSkip{Value: v} }),
		"tls-max":        optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionTLSMax{Value: v} }),
		"tls-min":        optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionTLSMin{Value: v} }),
		"unix-socket":    optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionUnixSocket{Value: v} }),
		"user":           optionOf(unquotedTemplate, func(v ast.Template) ast.OptionKind { return ast.OptionUser{Value: v} }),
		"variable":       optionOf(variableDefinition, func(v ast.VariableDefinition) ast.OptionKind { return ast.OptionVariable{Value: v} }),
		"verbose":        optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionVerbose{Value: v} }),
		"verbosity":      optionVerbosity,
		"very-verbose":   optionOf(boolOpt, func(v ast.BooleanOption) ast.OptionKind { return ast.OptionVeryVerbose{Value: v} }),
	}
}

func optionVerbosity(r *reader.Reader) (ast.OptionKind, *ParseError) {
	start := r.Cursor()
	switch r.ReadWhile(isASCIIAlphabetic) {
	case "brief":
		return ast.OptionVerbosity{Value: ast.VerbosityBrief}, nil
	case "verbose":
		return ast.OptionVerbosity{Value: ast.VerbosityVerbose}, nil
	case "debug":
		return ast.OptionVerbosity{Value: ast.VerbosityDebug}, nil
	}
	r.Seek(start)
	return nil, newError(start.Pos, false, ParseErrorExpecting{Value: "brief|verbose|debug"})
}

func count(r *reader.Reader) (ast.Count, *ParseError) {
	start := r.Cursor()
	v, err := nonRecover(integer, r)
	if err != nil {
		return nil, err
	}
	switch {
	case v.Value == -1:
		return ast.CountInfinite{}, nil
	case v.Value >= 0:
		return ast.CountFinite{Value: int(v.Value)}, nil
	}
	return nil, newError(start.Pos, false, ParseErrorExpecting{Value: "Expecting a count value"})
}

// placeholderOption parses a placeholder, after the literal parser failed.
func placeholderOption(r *reader.Reader, start reader.Cursor, expecting string) (ast.Placeholder, *ParseError) {
	r.Seek(start)
	p, err := placeholder(r)
	if err != nil {
		return ast.Placeholder{}, newError(err.Pos, false, ParseErrorExpecting{Value: expecting})
	}
	return p, nil
}

func booleanOption(r *reader.Reader) (ast.BooleanOption, *ParseError) {
	start := r.Cursor()
	if v, err := boolean(r); err == nil {
		return ast.BooleanLiteral{Value: v}, nil
	}
	p, err := placeholderOption(r, start, "true|false")
	if err != nil {
		return nil, err
	}
	return ast.BooleanPlaceholder{Value: p}, nil
}

func naturalOption(r *reader.Reader) (ast.NaturalOption, *ParseError) {
	start := r.Cursor()
	if v, err := natural(r); err == nil {
		return ast.NaturalLiteral{Value: v}, nil
	}
	p, err := placeholderOption(r, start, "integer >= 0")
	if err != nil {
		return nil, err
	}
	return ast.NaturalPlaceholder{Value: p}, nil
}

func countOption(r *reader.Reader) (ast.CountOption, *ParseError) {
	start := r.Cursor()
	if v, err := count(r); err == nil {
		return ast.CountLiteral{Value: v}, nil
	}
	p, err := placeholderOption(r, start, "integer >= -1")
	if err != nil {
		return nil, err
	}
	return ast.CountPlaceholder{Value: p}, nil
}

func durationOption(r *reader.Reader) (ast.DurationOption, *ParseError) {
	start := r.Cursor()
	v, err := duration(r)
	if err == nil {
		return ast.DurationLiteral{Value: v}, nil
	}
	if !err.Recoverable {
		return nil, err
	}
	p, err := placeholderOption(r, start, "integer")
	if err != nil {
		return nil, err
	}
	return ast.DurationPlaceholder{Value: p}, nil
}

func variableDefinition(r *reader.Reader) (ast.VariableDefinition, *ParseError) {
	start := r.Cursor()
	name, err := optionVariableName(r)
	if err != nil {
		return ast.VariableDefinition{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.VariableDefinition{}, err
	}
	if err := literal("=", r); err != nil {
		return ast.VariableDefinition{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.VariableDefinition{}, err
	}
	value, err := variableValue(r)
	if err != nil {
		return ast.VariableDefinition{}, err
	}
	return ast.VariableDefinition{
		SourceInfo: sourceInfo(start.Pos, r.Cursor().Pos),
		Name:       name,
		Space0:     space0,
		Space1:     space1,
		Value:      value,
	}, nil
}

func optionVariableName(r *reader.Reader) (string, *ParseError) {
	start := r.Cursor()
	name := r.ReadWhile(isVariableChar)
	if name == "" {
		return "", newError(start.Pos, false, ParseErrorExpecting{Value: "variable name"})
	}
	if ast.IsVariableReserved(name) {
		return "", newError(start.Pos, false, ParseErrorVariable{Value: fmt.Sprintf("conflicts with the %s function, use a different name", name)})
	}
	return name, nil
}

// literalValue checks that a null, boolean or number literal spans the whole
// variable value. When more chars follow (as in `11aa` or `true_is_true`),
// the literal is only the start of an unquoted string, so the error is
// recoverable to fall back to it.
func literalValue(r *reader.Reader, value ast.VariableValue) (ast.VariableValue, *ParseError) {
	c, ok := r.Peek()
	if !ok || c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '#' {
		return value, nil
	}
	return nil, newError(r.Cursor().Pos, true, ParseErrorExpecting{Value: "variable value"})
}

func variableValue(r *reader.Reader) (ast.VariableValue, *ParseError) {
	v, err := choice([]func(*reader.Reader) (ast.VariableValue, *ParseError){
		func(r *reader.Reader) (ast.VariableValue, *ParseError) {
			if err := null(r); err != nil {
				return nil, err
			}
			return literalValue(r, ast.VariableNull{})
		},
		func(r *reader.Reader) (ast.VariableValue, *ParseError) {
			b, err := boolean(r)
			if err != nil {
				return nil, err
			}
			return literalValue(r, ast.VariableBool{Value: b})
		},
		func(r *reader.Reader) (ast.VariableValue, *ParseError) {
			n, err := number(r)
			if err != nil {
				return nil, err
			}
			return literalValue(r, ast.VariableNumber{Value: n})
		},
		func(r *reader.Reader) (ast.VariableValue, *ParseError) {
			t, err := quotedTemplate(r)
			if err != nil {
				return nil, err
			}
			return ast.VariableString{Value: t}, nil
		},
		func(r *reader.Reader) (ast.VariableValue, *ParseError) {
			t, err := unquotedTemplate(r)
			if err != nil {
				return nil, err
			}
			return ast.VariableString{Value: t}, nil
		},
	}, r)
	if err != nil {
		return nil, newError(err.Pos, false, ParseErrorExpecting{Value: "variable value"})
	}
	return v, nil
}
