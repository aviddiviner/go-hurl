package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/pretty"
	"github.com/aviddiviner/go-hurl/internal/runner"
	"github.com/aviddiviner/go-hurl/internal/util"
)

// helpWidth returns the width used to wrap the help, as clap: the width of
// the terminal on standard output, else $COLUMNS, else 100.
var helpWidth = func() int {
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		return w
	}
	if w, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && w >= 0 {
		return w
	}
	return 100
}

// unsupported are the flags of features dropped by go-hurl: they are
// accepted by the parser, but fail at startup.
var unsupported = []string{"aws_sigv4", "digest", "http10", "http3", "negotiate", "ntlm"}

var (
	errHelp     = errors.New("help")
	errVersion  = errors.New("version")
	errMultiple = errors.New("multiple")
)

type unexpectedValueError struct{ value string }

func (e *unexpectedValueError) Error() string { return "unexpected value " + e.value }

type looksLikeFlagError struct{ value string }

func (e *looksLikeFlagError) Error() string { return "looks like a flag " + e.value }

// invalidValueError is a value not in the possible values.
type invalidValueError struct{ value string }

func (e *invalidValueError) Error() string { return "invalid value " + e.value }

// valueValidationError is a value rejected by the value parser.
type valueValidationError struct {
	value  string
	source error
}

func (e *valueValidationError) Error() string { return "invalid value " + e.value }

// flagPresent is the NoOptDefVal of our flags, so that an explicit value
// (--test=true) can be told apart from the flag alone.
const flagPresent = "\x00"

// argMatches records the explicit args in command line order, as clap's
// matcher does.
type argMatches struct {
	used   []string
	values map[string][]string
	// attached are the values given with the option in the same argument
	// (--header=-x), which clap takes even if they look like flags.
	attached map[string]int
	binName  string
}

type flagValue struct {
	m   *argMatches
	def *argDef
}

func (v *flagValue) String() string { return "" }
func (v *flagValue) Type() string   { return "bool" }

func (v *flagValue) Set(s string) error {
	switch {
	case s != flagPresent:
		return &unexpectedValueError{s}
	case v.def.id == "help":
		return errHelp
	case v.def.id == "version":
		return errVersion
	case slices.Contains(v.m.used, v.def.id):
		return errMultiple
	}
	v.m.used = append(v.m.used, v.def.id)
	return nil
}

type stringValue struct {
	m   *argMatches
	def *argDef
}

func (v *stringValue) String() string { return "" }
func (v *stringValue) Type() string   { return "string" }

func (v *stringValue) Set(s string) error {
	if v.m.attached[s] > 0 {
		v.m.attached[s]--
	} else if strings.HasPrefix(s, "-") && s != "-" && !(v.def.allowNegative && isNegativeNumber(s)) {
		// clap doesn't take a value that looks like a flag.
		return &looksLikeFlagError{s}
	}
	if !v.def.multiple && slices.Contains(v.m.used, v.def.id) {
		return errMultiple
	}
	if len(v.def.possibleValues) > 0 && !slices.Contains(v.def.possibleValues, s) {
		return &invalidValueError{s}
	}
	if v.def.parser != nil {
		if err := v.def.parser(s); err != nil {
			return &valueValidationError{s, err}
		}
	}
	if !slices.Contains(v.m.used, v.def.id) {
		v.m.used = append(v.m.used, v.def.id)
	}
	v.m.values[v.def.id] = append(v.m.values[v.def.id], s)
	return nil
}

// isNegativeNumber is clap_lex's ParsedArg::is_negative_number.
func isNegativeNumber(s string) bool {
	arg, ok := strings.CutPrefix(s, "-")
	if !ok {
		return false
	}
	seenDot := false
	posE := -1
	for i := range len(arg) {
		c := arg[i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' && !seenDot && posE < 0 && i > 0:
			seenDot = true
		case (c == 'e' || c == 'E') && posE < 0 && i > 0:
			posE = i
		default:
			return false
		}
	}
	return posE < 0 || posE != len(arg)-1
}

func (m *argMatches) has(id string) bool { return slices.Contains(m.used, id) }

func (m *argMatches) get(id string) (string, bool) {
	v := m.values[id]
	if len(v) == 0 {
		return "", false
	}
	return v[len(v)-1], true
}

func (m *argMatches) getAll(id string) []string { return m.values[id] }

func findArg(pred func(*argDef) bool) *argDef {
	if i := slices.IndexFunc(argDefs, pred); i >= 0 {
		return argDefs[i]
	}
	return nil
}

func argByID(id string) *argDef { return findArg(func(a *argDef) bool { return a.id == id }) }

func argByLong(long string) *argDef {
	return findArg(func(a *argDef) bool { return a.long == long || (a.alias != "" && a.alias == long) })
}

func conflicting(a, b string) bool {
	return slices.Contains(argByID(a).conflicts, b) || slices.Contains(argByID(b).conflicts, a)
}

// usage renders clap's usage line for the used ids.
func (m *argMatches) usage(used []string) string {
	s := "Usage: " + m.binName + " "
	if len(used) == 0 {
		s += "[OPTIONS] "
	}
	var seen []string
	for _, id := range used {
		if id == filesID || slices.Contains(seen, id) {
			continue
		}
		seen = append(seen, id)
		s += argByID(id).display() + " "
	}
	if slices.Contains(used, filesID) {
		return s + "<FILES>..."
	}
	return s + "[FILES]..."
}

const tryHelp = "For more information, try '--help'.\n"

// The clap errors are returned without their "error: " prefix, as Hurl
// does (OptionsErrorError adds it back).
func (m *argMatches) clapError(msg string, used []string) error {
	return OptionsErrorError{Message: msg + "\n\n" + m.usage(used) + "\n\n" + tryHelp}
}

func clapErrorNoUsage(msg string) error {
	return OptionsErrorError{Message: msg + "\n\n" + tryHelp}
}

func (m *argMatches) unknownArgument(name, suggestion string, used []string) error {
	msg := "unexpected argument '" + name + "' found\n\n"
	if suggestion != "" {
		msg += "  tip: a similar argument exists: '" + suggestion + "'\n\n"
	} else {
		msg += "  tip: to pass '" + name + "' as a value, use '-- " + name + "'\n\n"
	}
	return OptionsErrorError{Message: msg + m.usage(used) + "\n\n" + tryHelp}
}

// didYouMean returns the most similar candidate (the last one on a tie), as
// clap's suggestions::did_you_mean.
func didYouMean(value string, candidates []string) string {
	best := ""
	bestScore := 0.0
	for _, c := range candidates {
		if score := jaro(value, c); score > 0.7 && score >= bestScore {
			best, bestScore = c, score
		}
	}
	return best
}

func (m *argMatches) unknownLong(name string, files []string) error {
	used := slices.Clone(m.used)
	if len(files) > 0 {
		used = append(used, filesID)
	}
	var longs []string
	for _, a := range argDefs {
		longs = append(longs, a.long)
		if a.alias != "" {
			longs = append(longs, a.alias)
		}
	}
	best := didYouMean(name, longs)
	if best == "" {
		return m.unknownArgument("--"+name, "", used)
	}
	return m.unknownArgument("--"+name, "--"+best, append(used, argByLong(best).id))
}

func newFlagSet(m *argMatches) *pflag.FlagSet {
	fs := pflag.NewFlagSet("hurl", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	fs.SortFlags = false
	for _, a := range argDefs {
		names := []string{a.long}
		if a.alias != "" {
			names = append(names, a.alias)
		}
		for i, name := range names {
			var value pflag.Value
			if a.isFlag() {
				value = &flagValue{m: m, def: a}
			} else {
				value = &stringValue{m: m, def: a}
			}
			short := a.short
			if i > 0 {
				short = ""
			}
			f := fs.VarPF(value, name, short, "")
			if a.isFlag() {
				f.NoOptDefVal = flagPresent
			}
		}
	}
	return fs
}

// recordAttached records the values attached to their option in the same
// argument (--name=value, -Hvalue).
func (m *argMatches) recordAttached(args []string) {
	for _, arg := range args {
		if arg == "--" {
			return
		}
		if long, ok := strings.CutPrefix(arg, "--"); ok {
			if _, value, ok := strings.Cut(long, "="); ok {
				m.attached[value]++
			}
			continue
		}
		short, ok := strings.CutPrefix(arg, "-")
		if !ok {
			continue
		}
		for i, c := range short {
			def := findArg(func(a *argDef) bool { return a.short == string(c) })
			if def == nil || def.isFlag() {
				continue
			}
			value := short[i+1:]
			value = strings.TrimPrefix(value, "=")
			if value != "" {
				m.attached[value]++
			}
			break
		}
	}
}

func (m *argMatches) mapError(err error, files []string, color bool) error {
	var notExist *pflag.NotExistError
	var required *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &notExist):
		if notExist.GetSpecifiedShortnames() != "" {
			return m.unknownArgument("-"+notExist.GetSpecifiedName(), "", nil)
		}
		return m.unknownLong(notExist.GetSpecifiedName(), files)
	case errors.As(err, &required):
		def := argByLong(required.GetFlag().Name)
		return clapErrorNoUsage("a value is required for '" + def.display() + "' but none was supplied")
	case errors.As(err, &invalid):
		def := argByLong(invalid.GetFlag().Name)
		var unexpected *unexpectedValueError
		var looksLikeFlag *looksLikeFlagError
		var invalidValue *invalidValueError
		var validation *valueValidationError
		switch {
		case errors.Is(err, errHelp):
			return OptionsErrorDisplayHelp{Message: renderHelp(m.binName, helpWidth(), color)}
		case errors.Is(err, errVersion):
			return OptionsErrorDisplayVersion{Message: "hurl " + getVersion() + "\n"}
		case errors.Is(err, errMultiple):
			return m.clapError("the argument '"+def.display()+"' cannot be used multiple times", nil)
		case errors.As(err, &unexpected):
			used := append(slices.Clone(m.used), def.id)
			msg := fmt.Sprintf("unexpected value '%s' for '%s' found; no more were expected", unexpected.value, def.display())
			return m.clapError(msg, used)
		case errors.As(err, &looksLikeFlag):
			return m.flagAsValue(def, looksLikeFlag.value, files)
		case errors.As(err, &invalidValue):
			var msg string
			if invalidValue.value == "" {
				msg = "a value is required for '" + def.display() + "' but none was supplied"
			} else {
				msg = "invalid value '" + invalidValue.value + "' for '" + def.display() + "'"
			}
			msg += "\n  [possible values: " + strings.Join(def.possibleValues, ", ") + "]"
			if s := didYouMean(invalidValue.value, def.possibleValues); s != "" {
				msg += "\n\n  tip: a similar value exists: '" + s + "'"
			}
			return clapErrorNoUsage(msg)
		case errors.As(err, &validation):
			return clapErrorNoUsage("invalid value '" + validation.value + "' for '" + def.display() + "': " + validation.source.Error())
		}
	case errors.As(err, &syntax):
		return m.unknownArgument(syntax.GetSpecifiedFlag(), "", nil)
	}
	return optionsErrorf("%s", err)
}

// flagAsValue handles an option followed by something that looks like a
// flag: clap reports the flag if it's unknown, or a missing value.
func (m *argMatches) flagAsValue(def *argDef, value string, files []string) error {
	if long, ok := strings.CutPrefix(value, "--"); ok {
		name, _, _ := strings.Cut(long, "=")
		if name != "" && argByLong(name) == nil {
			return m.unknownLong(name, files)
		}
	} else {
		short := string([]rune(value)[1])
		if findArg(func(a *argDef) bool { return a.short == short }) == nil {
			return m.unknownArgument("-"+short, "", nil)
		}
	}
	return clapErrorNoUsage("a value is required for '" + def.display() + "' but none was supplied")
}

// validateConflicts reports the first conflict, as clap does: for each
// present arg in command line order, the present args it conflicts with.
func (m *argMatches) validateConflicts(hasFiles bool) error {
	for _, id := range m.used {
		var others []string
		for _, other := range m.used {
			if other != id && conflicting(id, other) {
				others = append(others, other)
			}
		}
		if len(others) == 0 {
			continue
		}
		var used []string
		for _, u := range m.used {
			if !slices.Contains(others, u) {
				used = append(used, u)
			}
		}
		if hasFiles {
			used = append(used, filesID)
		}
		msg := "the argument '" + argByID(id).display() + "' cannot be used with"
		if len(others) == 1 {
			msg += " '" + argByID(others[0]).display() + "'"
		} else {
			msg += ":"
			for _, o := range others {
				msg += "\n  " + argByID(o).display()
			}
		}
		return m.clapError(msg, used)
	}
	return nil
}

// validateSupported fails on the first flag of a feature dropped by go-hurl.
func (m *argMatches) validateSupported() error {
	for _, id := range m.used {
		if slices.Contains(unsupported, id) {
			return optionsErrorf("--%s is not supported by go-hurl", argByID(id).long)
		}
		if id == "client_cert_file" {
			cert, _ := m.get(id)
			if !isFile(cert) && certHasPassword(cert) {
				return optionsErrorf("--cert with a password is not supported by go-hurl")
			}
		}
	}
	return nil
}

// certHasPassword reports whether a --cert value has a password, as curl
// parses <certificate>[:<password>] (a backslash escapes the next char).
func certHasPassword(cert string) bool {
	for i := 0; i < len(cert); i++ {
		switch cert[i] {
		case '\\':
			i++
		case ':':
			return true
		}
	}
	return false
}

// fileName returns the file name of path, as Rust's Path::file_name, or "".
func fileName(path string) string {
	var last string
	for c := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if c != "" && (c != "." || last == "") {
			last = c
		}
	}
	if last == "." || last == ".." {
		return ""
	}
	return last
}

// parseCliArgs updates options from the command line arguments args
// (args[0] is the program).
func parseCliArgs(args []string, ctx *RunContext, o *CliOptions) error {
	m := &argMatches{values: map[string][]string{}, attached: map[string]int{}, binName: "hurl"}
	if len(args) > 0 {
		if name := fileName(args[0]); name != "" {
			m.binName = name
		}
		args = args[1:]
	}
	colorStdout := o.ColorStdout.UnwrapOr(ctx.IsStdoutTerm())

	m.recordAttached(args)
	fs := newFlagSet(m)
	if err := fs.Parse(args); err != nil {
		return m.mapError(err, fs.Args(), colorStdout)
	}
	files := fs.Args()
	if err := m.validateConflicts(len(files) > 0); err != nil {
		return err
	}
	if err := m.validateSupported(); err != nil {
		return err
	}

	// Without any input file (from standard input or the command line
	// arguments), we print the help and exit.
	if len(files) == 0 && !m.has("glob") && ctx.IsStdinTerm() {
		return OptionsErrorNoInput{Message: renderHelp(m.binName, helpWidth(), colorStdout)}
	}
	if err := m.parseMatches(files, ctx, o); err != nil {
		return err
	}
	if len(o.InputFiles) == 0 {
		return optionsErrorf("No input files provided")
	}
	return nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// rustJoin joins like Rust's Path::join, without cleaning the path.
func rustJoin(dir, name string) string {
	if dir == "" {
		return name
	}
	if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

// parseMatches is Hurl's parse_arg_matches: options set on the command line
// override o. The options are evaluated in Hurl's order, which is the order
// of the errors.
func (m *argMatches) parseMatches(files []string, ctx *RunContext, o *CliOptions) error {
	if v, ok := m.get("aws_sigv4"); ok {
		o.AwsSigv4 = v
	}
	if v, ok := m.get("cacert_file"); ok {
		if !pathExists(v) {
			return optionsErrorf("Input file %s does not exist", v)
		}
		o.CacertFile = v
	}
	for _, id := range []string{"client_cert_file", "client_key_file"} {
		v, ok := m.get(id)
		if !ok {
			continue
		}
		if !isFile(v) {
			return optionsErrorf("File %s does not exist", v)
		}
		if id == "client_cert_file" {
			o.ClientCertFile = v
		} else {
			o.ClientKeyFile = v
		}
	}
	if m.has("no_color") {
		o.ColorStdout, o.ColorStderr = BoolOptFalse, BoolOptFalse
	} else if m.has("color") {
		o.ColorStdout, o.ColorStderr = BoolOptTrue, BoolOptTrue
	}
	if m.has("compressed") {
		o.Compressed = true
	}
	if v, ok := m.get("connect_timeout"); ok {
		d, err := durationFromStr(v, ast.DurationUnitSecond)
		if err != nil {
			return err
		}
		o.ConnectTimeout = d
	}
	if v := m.getAll("connect_to"); v != nil {
		o.ConnectsTo = v
	}
	if m.has("continue_on_error") {
		o.ContinueOnError = true
	}
	if v, ok := m.get("cookies_input_file"); ok {
		o.CookieInputFile = v
	}
	if v, ok := m.get("cookies_output_file"); ok {
		o.CookieOutputFile = v
	}
	if v, ok := m.get("curl"); ok {
		o.CurlFile = v
	}
	if v, ok := m.get("delay"); ok {
		d, err := durationFromStr(v, ast.DurationUnitMilliSecond)
		if err != nil {
			return err
		}
		o.Delay = d
	}
	if m.has("digest") {
		o.Digest = true
	}
	if v, ok := m.get("error_format"); ok {
		f, err := parseErrorFormat(v)
		if err != nil {
			return err
		}
		o.ErrorFormat = f
	}
	if m.has("fail_with_body") {
		o.FailWithBody = true
	}
	if v, ok := m.get("file_root"); ok {
		o.FileRoot = &v
	}
	if m.has("follow_location") || m.has("follow_location_trusted") {
		o.FollowLocation = true
	}
	if m.has("follow_location_trusted") {
		o.FollowLocationTrusted = true
	}
	if v, ok := m.get("from_entry"); ok {
		o.FromEntry, _ = strconv.Atoi(v)
	}
	o.Headers = append(o.Headers, m.getAll("header")...)
	if v, ok := m.get("report_html"); ok {
		dir, err := reportDir(v, "HTML")
		if err != nil {
			return err
		}
		o.HtmlDir = dir
	}
	switch {
	case m.has("http3"):
		o.HttpVersion = HttpVersionV3
	case m.has("http2_prior_knowledge"):
		o.HttpVersion = HttpVersionV2PriorKnowledge
	case m.has("http2"):
		o.HttpVersion = HttpVersionV2
	case m.has("http11"):
		o.HttpVersion = HttpVersionV11
	case m.has("http10"):
		o.HttpVersion = HttpVersionV10
	}
	if m.has("no_assert") {
		o.NoAssert = true
	}
	if m.has("no_jsonpath_coercion") {
		o.NoJSONPathCoercion = true
	}
	if m.has("include") {
		o.Include = true
	}
	inputs, err := m.inputFiles(files, ctx)
	if err != nil {
		return err
	}
	o.InputFiles = inputs
	if m.has("insecure") {
		o.Insecure = true
	}
	if m.has("ipv6") {
		o.IpResolve = IpResolveIpV6
	} else if m.has("ipv4") {
		o.IpResolve = IpResolveIpV4
	}
	if v, ok := m.get("jobs"); ok {
		o.Jobs, _ = strconv.Atoi(v)
	}
	if v, ok := m.get("report_json"); ok {
		dir, err := reportDir(v, "JSON")
		if err != nil {
			return err
		}
		o.JSONReportDir = dir
	}
	if v, ok := m.get("report_junit"); ok {
		o.JunitFile = v
	}
	if v, ok := m.get("limit_rate"); ok {
		n, _ := rustParseUint64(v)
		o.LimitRate = &n
	}
	if v, ok := m.get("max_filesize"); ok {
		n, _ := rustParseUint64(v)
		o.MaxFilesize = &n
	}
	if v, ok := m.get("max_redirects"); ok {
		c, err := countArg(v, "--max-redirs <NUM>")
		if err != nil {
			return err
		}
		o.MaxRedirect = c
	}
	if m.has("negotiate") {
		o.Negotiate = true
	}
	if m.has("netrc") {
		o.Netrc = true
	}
	if v, ok := m.get("netrc_file"); ok {
		if !isFile(v) {
			return optionsErrorf("File %s does not exist", v)
		}
		o.NetrcFile = v
	}
	if m.has("netrc_optional") {
		o.NetrcOptional = true
	}
	if m.has("no_cookie_store") {
		o.NoCookieStore = true
	}
	o.NoHeaders = append(o.NoHeaders, m.getAll("no_header")...)
	if v, ok := m.get("no_proxy"); ok {
		o.NoProxy = &v
	}
	if m.has("ntlm") {
		o.Ntlm = true
	}
	if m.has("parallel") {
		o.Parallel = BoolOptTrue
	}
	if m.has("path_as_is") {
		o.PathAsIs = true
	}
	if v, ok := m.get("pinned_pub_key"); ok {
		o.PinnedPubKey = v
	}
	if m.has("progress_bar") {
		o.ProgressBar = BoolOptTrue
	}
	if m.has("pretty") {
		o.Pretty = pretty.Force
	} else if m.has("no_pretty") {
		o.Pretty = pretty.None
	}
	if v, ok := m.get("proxy"); ok {
		o.Proxy = v
	}
	o.ProxyHeaders = append(o.ProxyHeaders, m.getAll("proxy_header")...)
	if v, ok := m.get("output"); ok {
		o.Output = util.NewOutput(v)
	}
	if m.has("json") {
		o.OutputType = OutputTypeJSON
	} else if m.has("no_output") {
		o.OutputType = OutputTypeNoOutput
	}
	if v, ok := m.get("repeat"); ok {
		c, err := countArg(v, "--repeat <NUM>")
		if err != nil {
			return err
		}
		o.Repeat = c
	}
	if v := m.getAll("resolve"); v != nil {
		o.Resolves = v
	}
	if v, ok := m.get("retry"); ok {
		c, err := countArg(v, "--retry <NUM>")
		if err != nil {
			return err
		}
		o.Retry = c
	}
	if v, ok := m.get("retry_interval"); ok {
		d, err := durationFromStr(v, ast.DurationUnitMilliSecond)
		if err != nil {
			return err
		}
		o.RetryInterval = d
	}
	if err := m.secrets(o.Secrets); err != nil {
		return err
	}
	if m.has("ssl_no_revoke") {
		o.SslNoRevoke = true
	}
	if v, ok := m.get("report_tap"); ok {
		o.TapFile = v
	}
	if m.has("test") {
		o.Test = true
	}
	if v, ok := m.get("max_time"); ok {
		d, err := durationFromStr(v, ast.DurationUnitSecond)
		if err != nil {
			return err
		}
		o.Timeout = d
	}
	if v, ok := m.get("tls_max"); ok {
		o.TlsMax, _ = parseTlsVersion(v)
	}
	if v, ok := m.get("tls_min"); ok {
		o.TlsMin, _ = parseTlsVersion(v)
	}
	if v, ok := m.get("to_entry"); ok {
		o.ToEntry, _ = strconv.Atoi(v)
	}
	if v, ok := m.get("unix_socket"); ok {
		o.UnixSocket = v
	}
	if v, ok := m.get("user"); ok {
		o.User = &v
	}
	if v, ok := m.get("user_agent"); ok {
		o.UserAgent = &v
	}
	if err := m.variables(o.Variables); err != nil {
		return err
	}
	switch {
	case m.has("verbose"):
		o.Verbosity = VerbosityVerbose
	case m.has("very_verbose"):
		o.Verbosity = VerbosityDebug
	default:
		if v, ok := m.get("verbosity"); ok {
			o.Verbosity, _ = parseVerbosity(v)
		}
	}
	return nil
}

func countArg(v, display string) (ast.Count, error) {
	n, _ := strconv.Atoi(v)
	c, err := ast.CountFromInt32(int32(n))
	if err != nil {
		return nil, optionsErrorf("invalid value '%d' for '%s' (%s)", n, display, err)
	}
	return c, nil
}

// reportDir checks a report directory, creating it if it doesn't exist.
func reportDir(dir, kind string) (string, error) {
	if !pathExists(dir) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", optionsErrorf("%s dir %s can not be created", kind, dir)
		}
		return dir, nil
	}
	if isDir(dir) {
		return dir, nil
	}
	return "", optionsErrorf("%s is not a valid directory", dir)
}

// inputFiles returns the input files from the positional arguments and the
// globs, or standard input.
func (m *argMatches) inputFiles(files []string, ctx *RunContext) ([]input.Input, error) {
	var inputs []input.Input
	for _, filename := range files {
		if !pathExists(filename) {
			return nil, OptionsErrorInvalidInputFile{Path: filename}
		}
		if isFile(filename) {
			inputs = append(inputs, input.New(filename))
		} else if isDir(filename) {
			var err error
			if inputs, err = walkHurlFiles(filename, inputs); err != nil {
				return nil, err
			}
		}
	}
	for _, expr := range m.getAll("glob") {
		paths, err := glob(expr)
		if err != nil {
			return nil, optionsErrorf("Failed to read glob pattern")
		}
		if len(paths) == 0 {
			return nil, OptionsErrorInvalidInputFile{Path: expr}
		}
		for _, p := range paths {
			inputs = append(inputs, input.New(p))
		}
	}
	if len(inputs) == 0 && !ctx.IsStdinTerm() {
		in, err := input.FromStdin()
		if err != nil {
			return nil, optionsErrorf("%s", err)
		}
		inputs = append(inputs, in)
	}
	return inputs, nil
}

// walkHurlFiles walks dir recursively and appends the .hurl files to
// inputs. Entries are visited in name order (Rust uses the OS order).
func walkHurlFiles(dir string, inputs []input.Input) ([]input.Input, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, OptionsErrorInvalidInputFile{Path: dir}
	}
	for _, e := range entries {
		path := rustJoin(dir, e.Name())
		if isDir(path) {
			if inputs, err = walkHurlFiles(path, inputs); err != nil {
				return nil, err
			}
		} else if i := strings.LastIndex(e.Name(), "."); i > 0 && e.Name()[i+1:] == "hurl" {
			inputs = append(inputs, input.New(path))
		}
	}
	return inputs, nil
}

func (m *argMatches) secrets(secrets map[string]string) error {
	for _, f := range m.getAll("secrets_file") {
		err := readVariablesFile(f, typeString, func(name string, value runner.Value) error {
			return addSecret(secrets, name, value)
		})
		if err != nil {
			return err
		}
	}
	for _, s := range m.getAll("secret") {
		name, value, err := parseVariable(s, typeString)
		if err != nil {
			return err
		}
		if err := addSecret(secrets, name, value); err != nil {
			return err
		}
	}
	return nil
}

func (m *argMatches) variables(variables map[string]runner.Value) error {
	for _, f := range m.getAll("variables_file") {
		err := readVariablesFile(f, typeInferred, func(name string, value runner.Value) error {
			variables[name] = value
			return nil
		})
		if err != nil {
			return err
		}
	}
	for _, s := range m.getAll("variable") {
		name, value, err := parseVariable(s, typeInferred)
		if err != nil {
			return err
		}
		variables[name] = value
	}
	return nil
}

// jaro is the Jaro similarity of a and b, as in Rust's strsim::jaro.
func jaro(a, b string) float64 {
	ar, br := []rune(a), []rune(b)
	if len(ar) == 0 && len(br) == 0 {
		return 1
	}
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	window := max(max(len(ar), len(br))/2-1, 0)
	aFlags := make([]bool, len(ar))
	bFlags := make([]bool, len(br))
	matches := 0
	for i, ca := range ar {
		for j := max(i-window, 0); j < min(i+window+1, len(br)); j++ {
			if !bFlags[j] && br[j] == ca {
				aFlags[i], bFlags[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	transpositions, k := 0, 0
	for i := range ar {
		if !aFlags[i] {
			continue
		}
		for !bFlags[k] {
			k++
		}
		if ar[i] != br[k] {
			transpositions++
		}
		k++
	}
	mf := float64(matches)
	return (mf/float64(len(ar)) + mf/float64(len(br)) + (mf-float64(transpositions/2))/mf) / 3
}
