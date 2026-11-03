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
// lexical cleaning is done. Like Rust on Windows, a rooted p without a volume
// (\dir) replaces base but keeps its volume.
func joinPath(base, p string) string {
	if filepath.IsAbs(p) || base == "" || filepath.VolumeName(p) != "" {
		return p
	}
	if p != "" && os.IsPathSeparator(p[0]) {
		return filepath.VolumeName(base) + p
	}
	if p == "" {
		if hasTrailingSeparator(base) {
			return base
		}
		return base + string(filepath.Separator)
	}
	if hasTrailingSeparator(base) {
		return base + p
	}
	return base + string(filepath.Separator) + p
}

func hasTrailingSeparator(s string) bool {
	return s != "" && os.IsPathSeparator(s[len(s)-1])
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

// pathBuf is a path as a root and a list of components, like Rust's
// PathBuf. root is empty for a relative path, else the volume and/or the
// separator ("/", `C:\`, `\`). The first component may be "." (a leading
// current dir).
type pathBuf struct {
	root  string
	comps []string
}

func parsePath(path string) pathBuf {
	var p pathBuf
	vol := filepath.VolumeName(path)
	rest := path[len(vol):]
	p.root = vol
	if rest != "" && os.IsPathSeparator(rest[0]) {
		p.root += string(filepath.Separator)
	}
	for i, c := range strings.Split(filepath.ToSlash(rest), "/") {
		switch {
		case c == "":
		case c == "." && !(i == 0 && p.root == ""):
		default:
			p.comps = append(p.comps, c)
		}
	}
	return p
}

func (p *pathBuf) String() string {
	return p.root + strings.Join(p.comps, string(filepath.Separator))
}

func (p *pathBuf) push(c string) { p.comps = append(p.comps, c) }

// pop truncates p to its parent, returning false if p has no parent.
func (p *pathBuf) pop() bool {
	n := len(p.comps)
	if n == 0 {
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
	p := parsePath(path)
	for i := len(p.comps); i >= 0; i-- {
		prefix := pathBuf{root: p.root, comps: p.comps[:i]}
		resolved, err := canonicalize(prefix.String())
		if err != nil {
			continue
		}
		r := parsePath(resolved)
		for _, c := range p.comps[i:] {
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
	return normalizePath(p)
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
func normalizePath(p pathBuf) pathBuf {
	r := pathBuf{root: p.root}
	for _, c := range p.comps {
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
