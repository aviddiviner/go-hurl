package cli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// This is a port of the Rust glob crate 0.3 (glob::glob with the default
// match options), used by --glob: same pattern syntax (?, *, **, [...],
// [!...]) and same results order.

var errGlobPattern = errors.New("invalid glob pattern")

type globTokenKind int

const (
	globChar globTokenKind = iota
	globAnyChar
	globAnySequence
	globAnyRecursiveSequence
	globAnyWithin
	globAnyExcept
)

// globCharSpec is a single char (lo == hi) or a range.
type globCharSpec struct{ lo, hi rune }

type globToken struct {
	kind  globTokenKind
	char  rune
	specs []globCharSpec
}

type globPattern struct {
	tokens        []globToken
	isRecursive   bool
	hasMetachars  bool
	original      string
	startsWithDot bool
}

func isSeparator(c rune) bool { return c == '/' || c == filepath.Separator }

func compileGlob(pattern string) (*globPattern, error) {
	chars := []rune(pattern)
	p := &globPattern{original: pattern}
	for i := 0; i < len(chars); {
		switch chars[i] {
		case '?':
			p.tokens = append(p.tokens, globToken{kind: globAnyChar})
			i++
		case '*':
			old := i
			for i < len(chars) && chars[i] == '*' {
				i++
			}
			count := i - old
			switch {
			case count > 2:
				return nil, errGlobPattern
			case count == 2:
				// ** must form a whole path component.
				if old > 0 && !isSeparator(chars[old-1]) {
					return nil, errGlobPattern
				}
				if i < len(chars) && !isSeparator(chars[i]) {
					return nil, errGlobPattern
				}
				if i < len(chars) {
					i++
				}
				p.tokens = append(p.tokens, globToken{kind: globAnyRecursiveSequence})
				p.isRecursive = true
			default:
				p.tokens = append(p.tokens, globToken{kind: globAnySequence})
			}
		case '[':
			if i+4 <= len(chars) && chars[i+1] == '!' {
				j := slices.Index(chars[i+3:], ']')
				if j < 0 {
					return nil, errGlobPattern
				}
				specs := parseCharSpecs(chars[i+2 : i+3+j])
				p.tokens = append(p.tokens, globToken{kind: globAnyExcept, specs: specs})
				i += j + 4
				continue
			}
			if i+3 <= len(chars) && chars[i+1] != '!' {
				j := slices.Index(chars[i+2:], ']')
				if j < 0 {
					return nil, errGlobPattern
				}
				specs := parseCharSpecs(chars[i+1 : i+2+j])
				p.tokens = append(p.tokens, globToken{kind: globAnyWithin, specs: specs})
				i += j + 3
				continue
			}
			return nil, errGlobPattern
		default:
			p.tokens = append(p.tokens, globToken{kind: globChar, char: chars[i]})
			i++
		}
	}
	for _, t := range p.tokens {
		if t.kind != globChar {
			p.hasMetachars = true
		}
	}
	p.startsWithDot = len(p.tokens) > 0 && p.tokens[0].kind == globChar && p.tokens[0].char == '.'
	return p, nil
}

func parseCharSpecs(s []rune) []globCharSpec {
	var specs []globCharSpec
	for i := 0; i < len(s); {
		if i+3 <= len(s) && s[i+1] == '-' {
			specs = append(specs, globCharSpec{s[i], s[i+2]})
			i += 3
		} else {
			specs = append(specs, globCharSpec{s[i], s[i]})
			i++
		}
	}
	return specs
}

func (p *globPattern) matches(s string) bool {
	return matchGlobTokens(p.tokens, []rune(s))
}

func matchGlobTokens(tokens []globToken, s []rune) bool {
	for ti, t := range tokens {
		switch t.kind {
		case globAnySequence, globAnyRecursiveSequence:
			for i := 0; i <= len(s); i++ {
				if matchGlobTokens(tokens[ti+1:], s[i:]) {
					return true
				}
			}
			return false
		}
		if len(s) == 0 {
			return false
		}
		c := s[0]
		s = s[1:]
		switch t.kind {
		case globChar:
			if c != t.char {
				return false
			}
		case globAnyWithin:
			if !inCharSpecs(t.specs, c) {
				return false
			}
		case globAnyExcept:
			if inCharSpecs(t.specs, c) {
				return false
			}
		}
	}
	return len(s) == 0
}

func inCharSpecs(specs []globCharSpec, c rune) bool {
	for _, s := range specs {
		if s.lo <= c && c <= s.hi {
			return true
		}
	}
	return false
}

type globPath struct {
	path  string
	isDir bool
}

func newGlobPath(path string) globPath {
	info, err := os.Stat(path)
	return globPath{path: path, isDir: err == nil && info.IsDir()}
}

func (p globPath) fileName() string { return filepath.Base(p.path) }

type globTodo struct {
	path globPath
	// idx is the index of the pattern to match, -1 when already matched.
	idx int
	err error
}

// glob returns the paths matching pattern, in the glob crate order.
func glob(pattern string) ([]string, error) {
	if _, err := compileGlob(pattern); err != nil {
		return nil, err
	}
	root := ""
	rest := pattern
	if filepath.IsAbs(pattern) {
		vol := filepath.VolumeName(pattern)
		trimmed := strings.TrimLeftFunc(pattern[len(vol):], isSeparator)
		root = pattern[:len(pattern)-len(trimmed)]
		rest = trimmed
	}
	scope := "."
	if root != "" {
		scope = root
	}
	var patterns []*globPattern
	for _, c := range splitTerminator(rest) {
		p, err := compileGlob(c)
		if err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	if rest == "" {
		patterns = append(patterns, &globPattern{})
	}
	requireDir := strings.HasSuffix(pattern, "/") || strings.HasSuffix(pattern, string(filepath.Separator))

	var todo []globTodo
	if len(patterns) > 0 {
		fillGlobTodo(&todo, patterns, 0, newGlobPath(scope))
	}
	var results []string
	for len(patterns) > 0 && len(todo) > 0 {
		t := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if t.err != nil {
			return nil, t.err
		}
		path, idx := t.path, t.idx
		if idx == -1 {
			if requireDir && !path.isDir {
				continue
			}
			results = append(results, path.path)
			continue
		}
		if patterns[idx].isRecursive {
			next := idx
			for next+1 < len(patterns) && patterns[next+1].isRecursive {
				next++
			}
			if path.isDir {
				fillGlobTodo(&todo, patterns, next, path)
				if next == len(patterns)-1 {
					results = append(results, path.path)
					continue
				}
				idx = next + 1
			} else if next == len(patterns)-1 {
				continue
			} else {
				idx = next + 1
			}
		}
		if patterns[idx].matches(path.fileName()) {
			if idx == len(patterns)-1 {
				if !requireDir || path.isDir {
					results = append(results, path.path)
				}
			} else {
				fillGlobTodo(&todo, patterns, idx+1, path)
			}
		}
	}
	return results, nil
}

// splitTerminator is Rust's str::split_terminator on path separators: a
// trailing separator doesn't give an empty last component.
func splitTerminator(s string) []string {
	if s == "" {
		return nil
	}
	out := strings.Split(strings.ReplaceAll(s, string(filepath.Separator), "/"), "/")
	if out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

func joinGlob(dir, name string) string {
	if dir == "." {
		return name
	}
	if strings.HasSuffix(dir, "/") || strings.HasSuffix(dir, string(filepath.Separator)) {
		return dir + name
	}
	return dir + string(filepath.Separator) + name
}

func fillGlobTodo(todo *[]globTodo, patterns []*globPattern, idx int, path globPath) {
	add := func(next globPath) {
		if idx+1 == len(patterns) {
			*todo = append(*todo, globTodo{path: next, idx: -1})
		} else {
			fillGlobTodo(todo, patterns, idx+1, next)
		}
	}
	pattern := patterns[idx]
	if !pattern.hasMetachars {
		s := pattern.original
		special := s == "." || s == ".."
		next := newGlobPath(joinGlob(path.path, s))
		if special && path.isDir {
			add(next)
		} else if !special {
			if _, err := os.Lstat(next.path); err == nil {
				add(next)
			}
		}
		return
	}
	if !path.isDir {
		return
	}
	entries, err := os.ReadDir(path.path)
	if err != nil {
		*todo = append(*todo, globTodo{err: err})
		return
	}
	children := make([]globPath, 0, len(entries))
	for _, e := range entries {
		children = append(children, newGlobPath(joinGlob(path.path, e.Name())))
	}
	slices.SortFunc(children, func(a, b globPath) int { return strings.Compare(b.fileName(), a.fileName()) })
	for _, c := range children {
		*todo = append(*todo, globTodo{path: c, idx: idx})
	}
	if pattern.startsWithDot {
		for _, special := range []string{".", ".."} {
			if pattern.matches(special) {
				add(newGlobPath(joinGlob(path.path, special)))
			}
		}
	}
}
