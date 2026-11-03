package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/pflag"
	"golang.org/x/term"

	"github.com/aviddiviner/go-hurl/internal/input"
	"github.com/aviddiviner/go-hurl/internal/version"
)

type inputFormat int

const (
	inputHurl inputFormat = iota
	inputCurl
)

type outputFormat int

const (
	outputHurl outputFormat = iota
	outputJSON
	outputHTML
)

type options struct {
	check        bool
	color        *bool
	inPlace      bool
	inputFiles   []input.Input
	inputFormat  inputFormat
	outputFile   string
	outputFormat outputFormat
	standalone   bool
}

// optionsError is either informational (help, version: printed on stdout,
// exit 0) or an error (printed on stderr, exit 1).
type optionsError struct {
	message string
	info    bool
}

func (e *optionsError) Error() string { return e.message }

func errorf(format string, a ...any) *optionsError {
	return &optionsError{message: fmt.Sprintf(format, a...)}
}

const helpText = `Format Hurl files

Usage: hurlfmt [OPTIONS] [FILES]...

Arguments:
  [FILES]...  Set the input file to use

Options:
      --check          Run in check mode
      --color          Colorize Output
      --in-place       Modify files in place
      --in <FORMAT>    Specify input format: hurl or curl [default: hurl]
      --no-color       Do not colorize output
  -o, --output <FILE>  Write to FILE instead of stdout
      --out <FORMAT>   Specify output format: hurl, json or html [default: hurl]
      --standalone     Standalone HTML
  -h, --help           Print help
  -V, --version        Print version
`

func help() string {
	return strings.Replace(helpText, "Usage: hurlfmt ", "Usage: "+binName+" ", 1)
}

// binName is the program name shown in usage messages.
var binName = "hurlfmt"

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

const versionText = "hurlfmt " + version.Hurl + " (go-hurl " + version.GoHurl + ")\n"

// argDef describes a command line argument, mirroring Hurl's clap
// definitions (cli/options/commands.rs).
type argDef struct {
	id        string
	long      string
	short     string
	valueName string // empty for flags
	conflicts []string
}

// display renders the arg as clap does in error messages.
func (a *argDef) display() string {
	if a.valueName != "" {
		return "--" + a.long + " <" + a.valueName + ">"
	}
	return "--" + a.long
}

const filesID = "input_files"

// argDefs are in clap's definition order, which matters for suggestions.
var argDefs = []*argDef{
	{id: "check", long: "check", conflicts: []string{"output"}},
	{id: "color", long: "color", conflicts: []string{"no_color", "in_place"}},
	{id: "in_place", long: "in-place", conflicts: []string{"output", "color"}},
	{id: "input_format", long: "in", valueName: "FORMAT"},
	{id: "no_color", long: "no-color", conflicts: []string{"color"}},
	{id: "output", long: "output", short: "o", valueName: "FILE"},
	{id: "output_format", long: "out", valueName: "FORMAT", conflicts: []string{"check"}},
	{id: "standalone", long: "standalone", conflicts: []string{"no_color"}},
	{id: "help", long: "help", short: "h"},
	{id: "version", long: "version", short: "V"},
}

func findArg(pred func(*argDef) bool) *argDef {
	if i := slices.IndexFunc(argDefs, pred); i >= 0 {
		return argDefs[i]
	}
	return nil
}

func argByID(id string) *argDef { return findArg(func(a *argDef) bool { return a.id == id }) }

func conflicting(a, b string) bool {
	return slices.Contains(argByID(a).conflicts, b) || slices.Contains(argByID(b).conflicts, a)
}

var (
	errHelp     = errors.New("help")
	errVersion  = errors.New("version")
	errMultiple = errors.New("multiple")
)

type unexpectedValueError struct{ value string }

func (e *unexpectedValueError) Error() string { return "unexpected value " + e.value }

type looksLikeFlagError struct{ value string }

func (e *looksLikeFlagError) Error() string { return "looks like a flag " + e.value }

// flagPresent is the NoOptDefVal of our flags, so that an explicit value
// (--check=true) can be told apart from the flag alone.
const flagPresent = "\x00"

// argParser records the explicit args in command line order, as clap's matcher
// does.
type argParser struct {
	used   []string
	values map[string]string
}

type flagValue struct {
	p   *argParser
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
	case slices.Contains(v.p.used, v.def.id):
		return errMultiple
	}
	v.p.used = append(v.p.used, v.def.id)
	return nil
}

type stringValue struct {
	p   *argParser
	def *argDef
}

func (v *stringValue) String() string { return "" }
func (v *stringValue) Type() string   { return "string" }

func (v *stringValue) Set(s string) error {
	// clap doesn't take a value that looks like a flag.
	if strings.HasPrefix(s, "-") && s != "-" {
		return &looksLikeFlagError{s}
	}
	if slices.Contains(v.p.used, v.def.id) {
		return errMultiple
	}
	v.p.used = append(v.p.used, v.def.id)
	v.p.values[v.def.id] = s
	return nil
}

func (p *argParser) has(id string) bool { return slices.Contains(p.used, id) }

func (p *argParser) get(id string) (string, bool) {
	v, ok := p.values[id]
	return v, ok
}

// usage renders clap's usage line for the used ids.
func usage(used []string) string {
	s := "Usage: " + binName + " "
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

func clapError(msg string, used []string) *optionsError {
	return &optionsError{message: "error: " + msg + "\n\n" + usage(used) + "\n\n" + tryHelp}
}

func clapErrorNoUsage(msg string) *optionsError {
	return &optionsError{message: "error: " + msg + "\n\n" + tryHelp}
}

func unknownArgument(name, suggestion string, used []string) *optionsError {
	msg := "error: unexpected argument '" + name + "' found\n\n"
	if suggestion != "" {
		msg += "  tip: a similar argument exists: '" + suggestion + "'\n\n"
	} else {
		msg += "  tip: to pass '" + name + "' as a value, use '-- " + name + "'\n\n"
	}
	return &optionsError{message: msg + usage(used) + "\n\n" + tryHelp}
}

func (p *argParser) unknownLong(name string, files []string) *optionsError {
	used := slices.Clone(p.used)
	if len(files) > 0 {
		used = append(used, filesID)
	}
	// As clap's did_you_mean: the most similar long, the last one on a tie.
	var best *argDef
	bestScore := 0.0
	for _, a := range argDefs {
		if score := jaro(name, a.long); score > 0.7 && score >= bestScore {
			best, bestScore = a, score
		}
	}
	if best == nil {
		return unknownArgument("--"+name, "", used)
	}
	return unknownArgument("--"+name, "--"+best.long, append(used, best.id))
}

// parseArgs parses the command line arguments. args[0] is the program,
// whose file name is used in usage messages, as clap does.
func parseArgs(args []string) (*options, error) {
	if len(args) > 0 {
		if name := fileName(args[0]); name != "" {
			binName = name
		}
		args = args[1:]
	}
	p := &argParser{values: map[string]string{}}
	fs := pflag.NewFlagSet("hurlfmt", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}
	fs.SortFlags = false
	for _, a := range argDefs {
		var value pflag.Value
		if a.valueName == "" {
			value = &flagValue{p: p, def: a}
		} else {
			value = &stringValue{p: p, def: a}
		}
		f := fs.VarPF(value, a.long, a.short, "")
		if a.valueName == "" {
			f.NoOptDefVal = flagPresent
		}
	}

	if err := fs.Parse(args); err != nil {
		return nil, p.mapError(err, fs.Args())
	}
	files := fs.Args()
	if err := p.validateConflicts(len(files) > 0); err != nil {
		return nil, err
	}
	return p.options(files)
}

func (p *argParser) mapError(err error, files []string) error {
	var notExist *pflag.NotExistError
	var required *pflag.ValueRequiredError
	var invalid *pflag.InvalidValueError
	var syntax *pflag.InvalidSyntaxError
	switch {
	case errors.As(err, &notExist):
		if notExist.GetSpecifiedShortnames() != "" {
			return unknownArgument("-"+notExist.GetSpecifiedName(), "", nil)
		}
		return p.unknownLong(notExist.GetSpecifiedName(), files)
	case errors.As(err, &required):
		def := findArg(func(a *argDef) bool { return a.long == required.GetFlag().Name })
		return clapErrorNoUsage("a value is required for '" + def.display() + "' but none was supplied")
	case errors.As(err, &invalid):
		def := findArg(func(a *argDef) bool { return a.long == invalid.GetFlag().Name })
		var unexpected *unexpectedValueError
		var looksLikeFlag *looksLikeFlagError
		switch {
		case errors.Is(err, errHelp):
			return &optionsError{message: help(), info: true}
		case errors.Is(err, errVersion):
			return &optionsError{message: versionText, info: true}
		case errors.Is(err, errMultiple):
			return clapError("the argument '"+def.display()+"' cannot be used multiple times", nil)
		case errors.As(err, &unexpected):
			used := append(slices.Clone(p.used), def.id)
			msg := fmt.Sprintf("unexpected value '%s' for '%s' found; no more were expected", unexpected.value, def.display())
			return clapError(msg, used)
		case errors.As(err, &looksLikeFlag):
			return p.flagAsValue(def, looksLikeFlag.value, files)
		}
	case errors.As(err, &syntax):
		return unknownArgument(syntax.GetSpecifiedFlag(), "", nil)
	}
	return errorf("error: %s", err)
}

// flagAsValue handles an option followed by something that looks like a
// flag: clap reports the flag if it's unknown, or a missing value.
func (p *argParser) flagAsValue(def *argDef, value string, files []string) error {
	if long, ok := strings.CutPrefix(value, "--"); ok {
		name, _, _ := strings.Cut(long, "=")
		if findArg(func(a *argDef) bool { return a.long == name }) == nil && name != "" {
			return p.unknownLong(name, files)
		}
	} else {
		short := string([]rune(value)[1])
		if findArg(func(a *argDef) bool { return a.short == short }) == nil {
			return unknownArgument("-"+short, "", nil)
		}
	}
	return clapErrorNoUsage("a value is required for '" + def.display() + "' but none was supplied")
}

// validateConflicts reports the first conflict, as clap does: for each
// present arg in command line order, the present args it conflicts with.
func (p *argParser) validateConflicts(hasFiles bool) error {
	for _, id := range p.used {
		var others []string
		for _, other := range p.used {
			if other != id && conflicting(id, other) {
				others = append(others, other)
			}
		}
		if len(others) == 0 {
			continue
		}
		var used []string
		for _, u := range p.used {
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
		return clapError(msg, used)
	}
	return nil
}

func (p *argParser) inputFormat() (inputFormat, error) {
	v, ok := p.get("input_format")
	if !ok {
		v = "hurl"
	}
	switch v {
	case "hurl":
		return inputHurl, nil
	case "curl":
		return inputCurl, nil
	}
	return 0, errorf("Invalid input format %s", v)
}

func (p *argParser) outputFormat() (outputFormat, error) {
	v, ok := p.get("output_format")
	if !ok {
		v = "hurl"
	}
	switch v {
	case "hurl":
		return outputHurl, nil
	case "json":
		return outputJSON, nil
	case "html":
		return outputHTML, nil
	}
	return 0, errorf("Invalid output format %s", v)
}

func (p *argParser) options(files []string) (*options, error) {
	opts := &options{check: p.has("check")}

	if p.has("color") {
		opts.color = new(true)
	} else if p.has("no_color") || p.has("in_place") {
		opts.color = new(false)
	}

	if p.has("in_place") {
		format, err := p.inputFormat()
		if err != nil {
			return nil, err
		}
		if format != inputHurl {
			return nil, errorf("You can use --in-place only hurl format!")
		}
		if len(files) == 0 {
			return nil, errorf("You can not use --in-place with standard input stream!")
		}
		opts.inPlace = true
	}

	for _, filename := range files {
		if _, err := os.Stat(filename); err != nil {
			return nil, errorf("error: Cannot access '%s': No such file or directory", filename)
		}
		opts.inputFiles = append(opts.inputFiles, input.New(filename))
	}
	if len(opts.inputFiles) == 0 && !term.IsTerminal(int(os.Stdin.Fd())) {
		in, err := input.FromStdin()
		if err != nil {
			return nil, errorf("%s", err)
		}
		opts.inputFiles = append(opts.inputFiles, in)
	}

	var err error
	if opts.inputFormat, err = p.inputFormat(); err != nil {
		return nil, err
	}
	opts.outputFile, _ = p.get("output")
	if opts.outputFormat, err = p.outputFormat(); err != nil {
		return nil, err
	}
	if p.has("standalone") {
		if v, _ := p.get("output_format"); v != "html" {
			return nil, errorf("use --standalone option only with html output")
		}
		opts.standalone = true
	}

	if len(opts.inputFiles) == 0 {
		return nil, errorf("%s", help())
	}
	return opts, nil
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
	m := float64(matches)
	return (m/float64(len(ar)) + m/float64(len(br)) + (m-float64(transpositions/2))/m) / 3
}
