// Package util holds small helpers shared by the HTTP client and the runner:
// access-controlled paths, secret redaction and outputs. Ported from
// hurl/src/util/path.rs, util/redacted.rs and runner/output.rs.
package util

import (
	"os"
	"path/filepath"
	"strings"
)

// ContextDir represents the contextual directories used to run a Hurl file.
type ContextDir struct {
	// currentDir is the current working directory. If it's relative,
	// IsAccessAllowed is not guaranteed to be correct.
	currentDir string
	// fileRoot is the file root, inferred or set by the user. It can be
	// relative (to the current directory) or absolute.
	fileRoot string
}

// NewContextDir returns a context directory with the given current
// directory and file root.
func NewContextDir(currentDir, fileRoot string) ContextDir {
	return ContextDir{currentDir: currentDir, fileRoot: fileRoot}
}

// ResolvedPath returns a path (absolute or relative), given a filename.
func (c ContextDir) ResolvedPath(filename string) string {
	return joinPath(c.fileRoot, filename)
}

// IsAccessAllowed checks if access to filename is authorized: it must be a
// descendant of the file root. This is used to check if a local file can be
// included in a request, or if a response can be written to a file.
func (c ContextDir) IsAccessAllowed(filename string) bool {
	file := c.ResolvedPath(filename)
	absoluteFile := joinPath(c.currentDir, file)
	absoluteFileRoot := joinPath(c.currentDir, c.fileRoot)
	return isDescendant(absoluteFile, absoluteFileRoot)
}

// joinPath joins like Rust's Path::join: an absolute p replaces base, and no
// lexical cleaning is done.
func joinPath(base, p string) string {
	if filepath.IsAbs(p) || base == "" {
		return p
	}
	if p == "" {
		if strings.HasSuffix(base, string(filepath.Separator)) {
			return base
		}
		return base + string(filepath.Separator)
	}
	if strings.HasSuffix(base, string(filepath.Separator)) {
		return base + p
	}
	return base + string(filepath.Separator) + p
}

// isDescendant reports whether path is a descendant of ancestor. Both paths
// are resolved first, so symlinks can't be used to escape ancestor.
func isDescendant(path, ancestor string) bool {
	p := resolvePath(path)
	a := resolvePath(ancestor)
	for {
		if p.String() == a.String() {
			return true
		}
		if !p.pop() {
			return false
		}
	}
}

// pathBuf is a path as a list of components, like Rust's PathBuf. The first
// component may be "/" (root) or "." (a leading current dir).
type pathBuf struct{ comps []string }

func components(path string) []string {
	var comps []string
	if strings.HasPrefix(path, "/") {
		comps = append(comps, "/")
	}
	for i, c := range strings.Split(path, "/") {
		switch {
		case c == "":
		case c == "." && !(i == 0 && len(comps) == 0):
		default:
			comps = append(comps, c)
		}
	}
	return comps
}

func (p *pathBuf) String() string {
	if len(p.comps) == 0 {
		return ""
	}
	if p.comps[0] == "/" {
		return "/" + strings.Join(p.comps[1:], "/")
	}
	return strings.Join(p.comps, "/")
}

func (p *pathBuf) push(c string) {
	if c == "/" {
		p.comps = []string{"/"}
		return
	}
	p.comps = append(p.comps, c)
}

// pop truncates p to its parent, returning false if p has no parent.
func (p *pathBuf) pop() bool {
	n := len(p.comps)
	if n == 0 || (n == 1 && p.comps[0] == "/") {
		return false
	}
	p.comps = p.comps[:n-1]
	return true
}

// resolvePath returns the absolute form of path with symlinks resolved.
// Unlike filepath.EvalSymlinks, path doesn't need to exist: the longest
// existing prefix is canonicalized, and the remaining components are appended
// and lexically normalized. This is needed for files that don't exist yet,
// like output files.
func resolvePath(path string) pathBuf {
	comps := components(path)
	for i := len(comps); i >= 0; i-- {
		prefix := pathBuf{comps: comps[:i]}
		resolved, err := canonicalize(prefix.String())
		if err != nil {
			continue
		}
		r := pathBuf{comps: components(resolved)}
		for _, c := range comps[i:] {
			switch c {
			case ".":
			case "..":
				r.pop()
			default:
				r.push(c)
			}
		}
		return r
	}
	return normalizePath(comps)
}

func canonicalize(path string) (string, error) {
	if path == "" {
		return "", os.ErrNotExist
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(abs)
}

// normalizePath normalizes all intermediate components lexically.
func normalizePath(comps []string) pathBuf {
	var r pathBuf
	for _, c := range comps {
		switch c {
		case ".":
		case "..":
			r.pop()
		default:
			r.push(c)
		}
	}
	return r
}

// CreateDirAll creates the missing parent directories of filename.
func CreateDirAll(filename string) error {
	parent := filepath.Dir(filename)
	if parent == "." || parent == "" {
		return nil
	}
	return os.MkdirAll(parent, 0o755)
}
