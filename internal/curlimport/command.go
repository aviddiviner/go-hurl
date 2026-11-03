package curlimport

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// A small clap-like command line parser, faithful to the subset of clap
// behaviour (and error messages) that Hurl's curl import relies on.

type arg struct {
	id          string
	long        string
	short       rune
	valueName   string // empty for flags
	appendVals  bool
	allowHyphen bool
	overrides   string
	parse       func(string) (string, error)
}

func (a *arg) takesValue() bool { return a.valueName != "" }

// String renders the arg as clap does in error messages: "--header <NAME:VALUE>".
func (a *arg) String() string {
	if a.takesValue() {
		return "--" + a.long + " <" + a.valueName + ">"
	}
	return "--" + a.long
}

type command struct {
	name       string
	args       []*arg
	positional string // id of the single optional positional
	help       string
}

type matches struct {
	ids    []string // explicitly present ids, in first-seen order
	values map[string][]string
}

func (m *matches) has(id string) bool { return slices.Contains(m.ids, id) }

func (m *matches) get(id string) (string, bool) {
	if v := m.values[id]; len(v) > 0 {
		return v[0], true
	}
	return "", false
}

func (m *matches) remove(id string) {
	m.ids = slices.DeleteFunc(m.ids, func(s string) bool { return s == id })
	delete(m.values, id)
}

// helpError is returned for -h/--help; its message is the help text.
type helpError struct{ help string }

func (e *helpError) Error() string { return e.help }

func (c *command) findLong(name string) *arg {
	for _, a := range c.args {
		if a.long == name {
			return a
		}
	}
	return nil
}

func (c *command) findShort(r rune) *arg {
	for _, a := range c.args {
		if a.short == r {
			return a
		}
	}
	return nil
}

// usage renders clap's "smart usage" for the used ids.
func (c *command) usage(used []string) string {
	s := "Usage: " + c.name + " "
	if len(used) == 0 {
		s += "[OPTIONS] "
	}
	var seen []string
	for _, id := range used {
		if id == c.positional || slices.Contains(seen, id) {
			continue
		}
		seen = append(seen, id)
		for _, a := range append(slices.Clone(c.args), helpArg) {
			if a.id == id {
				s += a.String() + " "
			}
		}
	}
	if slices.Contains(used, c.positional) {
		s += "<" + c.positional + ">"
	} else {
		s += "[" + c.positional + "]"
	}
	return s
}

const tryHelp = "For more information, try '--help'.\n"

func (c *command) errorWithUsage(msg string, used []string) error {
	return errors.New("error: " + msg + "\n\n" + c.usage(used) + "\n\n" + tryHelp)
}

func errorNoUsage(msg string) error {
	return errors.New("error: " + msg + "\n\n" + tryHelp)
}

func (c *command) unknownArgument(name string, suggestion string, trailingTip bool, used []string) error {
	msg := "error: unexpected argument '" + name + "' found\n\n"
	if suggestion != "" {
		msg += "  tip: a similar argument exists: '" + suggestion + "'\n\n"
	} else if trailingTip {
		msg += "  tip: to pass '" + name + "' as a value, use '-- " + name + "'\n\n"
	}
	return errors.New(msg + c.usage(used) + "\n\n" + tryHelp)
}

// parse parses params; params[0] is the binary name, used in usage messages.
func (c *command) parse(params []string) (*matches, error) {
	cmd := *c
	if len(params) > 0 {
		if bin := binName(params[0]); bin != "" {
			cmd.name = bin
			cmd.help = strings.Replace(c.help, "Usage: "+c.name+" ", "Usage: "+bin+" ", 1)
		}
		params = params[1:]
	}
	return cmd.parseArgs(params)
}

// binName returns the file name of path, as Rust's Path::file_name, or "".
func binName(path string) string {
	var last string
	for c := range strings.SplitSeq(path, "/") {
		if c != "" && (c != "." || last == "") {
			last = c
		}
	}
	if last == "." || last == ".." {
		return ""
	}
	return last
}

func (c *command) parseArgs(params []string) (*matches, error) {
	m := &matches{values: map[string][]string{}}
	var pending *arg
	trailing := false

	missingValue := func(a *arg) error {
		return errorNoUsage("a value is required for '" + a.String() + "' but none was supplied")
	}
	setValue := func(a *arg, value string) error {
		if a.parse != nil {
			v, err := a.parse(value)
			if err != nil {
				return errorNoUsage(fmt.Sprintf("invalid value '%s' for '%s': %s", value, a, err))
			}
			value = v
		}
		m.values[a.id] = append(m.values[a.id], value)
		if !m.has(a.id) {
			m.ids = append(m.ids, a.id)
		}
		return nil
	}
	occur := func(a *arg) error {
		if m.has(a.id) && !a.appendVals {
			return c.errorWithUsage("the argument '"+a.String()+"' cannot be used multiple times", nil)
		}
		if a.overrides != "" {
			m.remove(a.overrides)
		}
		if !a.takesValue() && !m.has(a.id) {
			m.ids = append(m.ids, a.id)
		}
		return nil
	}

	for _, p := range params {
		if pending != nil {
			isNewArg := strings.HasPrefix(p, "-") && p != "-" && !pending.allowHyphen
			if !isNewArg {
				if err := setValue(pending, p); err != nil {
					return nil, err
				}
				pending = nil
				continue
			}
		}
		switch {
		case trailing || p == "-" || !strings.HasPrefix(p, "-"):
			if pending != nil {
				return nil, missingValue(pending)
			}
			if m.has(c.positional) {
				return nil, c.unknownArgument(p, "", false, nil)
			}
			m.ids = append(m.ids, c.positional)
			m.values[c.positional] = []string{p}
		case p == "--":
			if pending != nil {
				return nil, missingValue(pending)
			}
			trailing = true
		case strings.HasPrefix(p, "--"):
			name, value, hasValue := strings.Cut(p[2:], "=")
			if name == "help" {
				if pending != nil {
					return nil, missingValue(pending)
				}
				return nil, &helpError{c.help}
			}
			a := c.findLong(name)
			if a == nil {
				used := slices.Clone(m.ids)
				suggestion := c.suggest(name)
				if suggestion != nil {
					used = append(used, suggestion.id)
					return nil, c.unknownArgument("--"+name, "--"+suggestion.long, false, used)
				}
				return nil, c.unknownArgument("--"+name, "", true, used)
			}
			if pending != nil {
				return nil, missingValue(pending)
			}
			if err := occur(a); err != nil {
				return nil, err
			}
			switch {
			case !a.takesValue() && hasValue:
				msg := fmt.Sprintf("unexpected value '%s' for '%s' found; no more were expected", value, a)
				return nil, c.errorWithUsage(msg, m.ids)
			case !a.takesValue():
			case hasValue:
				if err := setValue(a, value); err != nil {
					return nil, err
				}
			default:
				pending = a
			}
		default:
			cluster := []rune(p[1:])
			for i, r := range cluster {
				if r == 'h' {
					if pending != nil {
						return nil, missingValue(pending)
					}
					return nil, &helpError{c.help}
				}
				a := c.findShort(r)
				if a == nil {
					return nil, c.unknownArgument("-"+string(r), "", true, nil)
				}
				if pending != nil {
					return nil, missingValue(pending)
				}
				if err := occur(a); err != nil {
					return nil, err
				}
				if !a.takesValue() {
					continue
				}
				rest := string(cluster[i+1:])
				rest = strings.TrimPrefix(rest, "=")
				if rest != "" || i+1 < len(cluster) {
					if err := setValue(a, rest); err != nil {
						return nil, err
					}
				} else {
					pending = a
				}
				break
			}
		}
	}
	if pending != nil {
		return nil, missingValue(pending)
	}
	return m, nil
}

var helpArg = &arg{id: "help", long: "help"}

// suggest returns the long arg most similar to name, as clap's did_you_mean.
// On a tie, the last candidate wins.
func (c *command) suggest(name string) *arg {
	var best *arg
	bestScore := 0.0
	for _, a := range append(slices.Clone(c.args), helpArg) {
		if score := jaro(name, a.long); score > 0.7 && score >= bestScore {
			best, bestScore = a, score
		}
	}
	return best
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
		lo, hi := max(i-window, 0), min(i+window+1, len(br))
		for j := lo; j < hi; j++ {
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
