package runner

import (
	"strings"

	"github.com/aviddiviner/go-hurl/internal/text"
)

// diff returns the hunks of a line diff between old and new, as rendered by
// Hurl (a unified diff without context lines). It reproduces the output of
// the Rust similar crate: Myers' divide-and-conquer algorithm, where
// adjacent deletions and insertions are merged into replacements.
func diff(old, new string) []DiffHunk {
	oldLines := tokenizeLines(old)
	newLines := tokenizeLines(new)
	var raw diffOps
	myersConquer(&raw, oldLines, 0, len(oldLines), newLines, 0, len(newLines))
	ops := cleanupDiffOps(oldLines, newLines, raw)
	var c diffCapture
	for _, op := range ops {
		switch op.tag {
		case diffEqual:
			c.equal(op.oldIndex, op.newIndex, op.oldLen)
		case diffDelete:
			c.delete(op.oldIndex, op.oldLen, op.newIndex)
		case diffInsert:
			c.insert(op.oldIndex, op.newIndex, op.newLen)
		}
	}
	c.finish()

	var hunks []DiffHunk
	for _, group := range groupDiffOps(c.ops) {
		start, sourceLine := hunkLines(group)
		var content text.StyledString
		for _, op := range group {
			for _, i := range op.oldRange() {
				if op.tag != diffInsert {
					sign, style := "-", text.NewStyle().Red()
					if op.tag == diffEqual {
						sign, style = " ", text.NewStyle()
					}
					content.PushWith(sign+changeString(oldLines[i]), style)
				}
			}
			if op.tag == diffInsert || op.tag == diffReplace {
				for _, i := range op.newRange() {
					content.PushWith("+"+changeString(newLines[i]), text.NewStyle().Green())
				}
			}
		}
		hunks = append(hunks, DiffHunk{Content: content, Start: start, SourceLine: sourceLine})
	}
	return hunks
}

// changeString is the display of a line change: a line missing its
// newline gets one.
func changeString(line string) string {
	if strings.HasSuffix(line, "\n") || strings.HasSuffix(line, "\r") {
		return line
	}
	return line + "\n"
}

// tokenizeLines splits s into lines, keeping their terminator ("\n", "\r\n"
// or a lone "\r").
func tokenizeLines(s string) []string {
	var lines []string
	last := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
			lines = append(lines, s[last:i+1])
			last = i + 1
		case '\n':
			lines = append(lines, s[last:i+1])
			last = i + 1
		}
	}
	if last < len(s) {
		lines = append(lines, s[last:])
	}
	return lines
}

// hunkLines returns the 0-based start line of a hunk and the 0-based line
// of its first change in the old string.
func hunkLines(ops []diffOp) (int, int) {
	start := 0
	for _, op := range ops {
		switch op.tag {
		case diffEqual:
			start = op.oldIndex
		case diffDelete, diffReplace:
			return start, op.oldIndex
		case diffInsert:
			return start, op.oldIndex - 1
		}
	}
	return start, 0
}

type diffTag int

const (
	diffEqual diffTag = iota
	diffDelete
	diffInsert
	diffReplace
)

type diffOp struct {
	tag      diffTag
	oldIndex int
	oldLen   int
	newIndex int
	newLen   int
}

func (op diffOp) oldRange() []int { return indexes(op.oldIndex, op.oldLen) }
func (op diffOp) newRange() []int { return indexes(op.newIndex, op.newLen) }

func indexes(start, n int) []int {
	r := make([]int, n)
	for i := range r {
		r[i] = start + i
	}
	return r
}

// diffHook receives the operations of a diff.
type diffHook interface {
	equal(oldIndex, newIndex, n int)
	delete(oldIndex, oldLen, newIndex int)
	insert(oldIndex, newIndex, newLen int)
}

// diffOps collects the raw operations of a diff.
type diffOps []diffOp

func (d *diffOps) equal(oldIndex, newIndex, n int) {
	*d = append(*d, diffOp{tag: diffEqual, oldIndex: oldIndex, oldLen: n, newIndex: newIndex, newLen: n})
}

func (d *diffOps) delete(oldIndex, oldLen, newIndex int) {
	*d = append(*d, diffOp{tag: diffDelete, oldIndex: oldIndex, oldLen: oldLen, newIndex: newIndex})
}

func (d *diffOps) insert(oldIndex, newIndex, newLen int) {
	*d = append(*d, diffOp{tag: diffInsert, oldIndex: oldIndex, newIndex: newIndex, newLen: newLen})
}

// diffCapture collects diff operations, merging consecutive equalities and
// turning adjacent deletions and insertions into replacements (similar's
// Replace hook over Capture).
type diffCapture struct {
	ops []diffOp
	eq  *diffOp
	del *diffOp
	ins *diffOp
}

func (c *diffCapture) equal(oldIndex, newIndex, n int) {
	c.flushDelIns()
	if c.eq != nil {
		c.eq.oldLen += n
		c.eq.newLen += n
	} else {
		c.eq = &diffOp{tag: diffEqual, oldIndex: oldIndex, oldLen: n, newIndex: newIndex, newLen: n}
	}
}

func (c *diffCapture) delete(oldIndex, oldLen, newIndex int) {
	c.flushEq()
	if c.del != nil {
		c.del.oldLen += oldLen
	} else {
		c.del = &diffOp{tag: diffDelete, oldIndex: oldIndex, oldLen: oldLen, newIndex: newIndex}
	}
}

func (c *diffCapture) insert(oldIndex, newIndex, newLen int) {
	c.flushEq()
	if c.ins != nil {
		c.ins.newLen += newLen
	} else {
		c.ins = &diffOp{tag: diffInsert, oldIndex: oldIndex, newIndex: newIndex, newLen: newLen}
	}
}

func (c *diffCapture) flushEq() {
	if c.eq != nil {
		c.ops = append(c.ops, *c.eq)
		c.eq = nil
	}
}

func (c *diffCapture) flushDelIns() {
	switch {
	case c.del != nil && c.ins != nil:
		c.ops = append(c.ops, diffOp{tag: diffReplace, oldIndex: c.del.oldIndex, oldLen: c.del.oldLen, newIndex: c.ins.newIndex, newLen: c.ins.newLen})
	case c.del != nil:
		c.ops = append(c.ops, *c.del)
	case c.ins != nil:
		c.ops = append(c.ops, *c.ins)
	}
	c.del, c.ins = nil, nil
}

func (c *diffCapture) finish() {
	c.flushEq()
	c.flushDelIns()
}

func commonPrefixLen(old []string, oldStart, oldEnd int, new []string, newStart, newEnd int) int {
	n := 0
	for oldStart+n < oldEnd && newStart+n < newEnd && old[oldStart+n] == new[newStart+n] {
		n++
	}
	return n
}

func commonSuffixLen(old []string, oldStart, oldEnd int, new []string, newStart, newEnd int) int {
	n := 0
	for oldEnd-n > oldStart && newEnd-n > newStart && old[oldEnd-n-1] == new[newEnd-n-1] {
		n++
	}
	return n
}

func myersMaxD(len1, len2 int) int { return (len1+len2+1)/2 + 1 }

// myersV is a V array indexed from -maxD to maxD.
type myersV struct {
	offset int
	v      []int
}

func newMyersV(maxD int) myersV { return myersV{offset: maxD, v: make([]int, 2*maxD)} }

func (v myersV) get(k int) int    { return v.v[k+v.offset] }
func (v myersV) set(k int, x int) { v.v[k+v.offset] = x }

func absInt(x int) int { return max(x, -x) }

// findMiddleSnake returns the start of the middle snake, in absolute
// indexes, or false if there is none.
func findMiddleSnake(old []string, oldStart, oldEnd int, new []string, newStart, newEnd int) (int, int, bool) {
	n := oldEnd - oldStart
	m := newEnd - newStart
	delta := n - m
	odd := delta&1 == 1

	dMax := myersMaxD(n, m)
	vf := newMyersV(dMax)
	vb := newMyersV(dMax)
	vf.set(1, 0)
	vb.set(1, 0)

	for d := range dMax {
		for k := d; k >= -d; k -= 2 {
			var x int
			if k == -d || (k != d && vf.get(k-1) < vf.get(k+1)) {
				x = vf.get(k + 1)
			} else {
				x = vf.get(k-1) + 1
			}
			y := x - k
			x0, y0 := x, y
			if x < n && y < m {
				x += commonPrefixLen(old, oldStart+x, oldEnd, new, newStart+y, newEnd)
			}
			vf.set(k, x)
			if odd && absInt(k-delta) <= d-1 && vf.get(k)+vb.get(-(k-delta)) >= n {
				return x0 + oldStart, y0 + newStart, true
			}
		}
		for k := d; k >= -d; k -= 2 {
			var x int
			if k == -d || (k != d && vb.get(k-1) < vb.get(k+1)) {
				x = vb.get(k + 1)
			} else {
				x = vb.get(k-1) + 1
			}
			y := x - k
			if x < n && y < m {
				advance := commonSuffixLen(old, oldStart, oldStart+n-x, new, newStart, newStart+m-y)
				x += advance
				y += advance
			}
			vb.set(k, x)
			if !odd && absInt(k-delta) <= d && vb.get(k)+vf.get(-(k-delta)) >= n {
				return n - x + oldStart, m - y + newStart, true
			}
		}
	}
	return 0, 0, false
}

func myersConquer(c diffHook, old []string, oldStart, oldEnd int, new []string, newStart, newEnd int) {
	prefix := commonPrefixLen(old, oldStart, oldEnd, new, newStart, newEnd)
	if prefix > 0 {
		c.equal(oldStart, newStart, prefix)
	}
	oldStart += prefix
	newStart += prefix

	suffix := commonSuffixLen(old, oldStart, oldEnd, new, newStart, newEnd)
	suffixOld, suffixNew := oldEnd-suffix, newEnd-suffix
	oldEnd -= suffix
	newEnd -= suffix

	switch {
	case oldStart >= oldEnd && newStart >= newEnd:
	case newStart >= newEnd:
		c.delete(oldStart, oldEnd-oldStart, newStart)
	case oldStart >= oldEnd:
		c.insert(oldStart, newStart, newEnd-newStart)
	default:
		if x, y, ok := findMiddleSnake(old, oldStart, oldEnd, new, newStart, newEnd); ok {
			myersConquer(c, old, oldStart, x, new, newStart, y)
			myersConquer(c, old, x, oldEnd, new, y, newEnd)
		} else {
			c.delete(oldStart, oldEnd-oldStart, newStart)
			c.insert(oldStart, newStart, newEnd-newStart)
		}
	}

	if suffix > 0 {
		c.equal(suffixOld, suffixNew, suffix)
	}
}

// groupDiffOps groups operations into hunks with no context lines
// (similar's group_diff_ops with n = 0).
func groupDiffOps(ops []diffOp) [][]diffOp {
	if len(ops) == 0 {
		return nil
	}
	ops = append([]diffOp(nil), ops...)
	if first := &ops[0]; first.tag == diffEqual {
		first.oldIndex += first.oldLen
		first.newIndex += first.oldLen
		first.oldLen, first.newLen = 0, 0
	}
	if last := &ops[len(ops)-1]; last.tag == diffEqual {
		last.oldLen, last.newLen = 0, 0
	}

	var groups [][]diffOp
	var pending []diffOp
	for _, op := range ops {
		if op.tag == diffEqual && op.oldLen > 0 {
			pending = append(pending, diffOp{tag: diffEqual, oldIndex: op.oldIndex, newIndex: op.newIndex})
			groups = append(groups, pending)
			pending = []diffOp{{tag: diffEqual, oldIndex: op.oldIndex + op.oldLen, newIndex: op.newIndex + op.oldLen}}
			continue
		}
		pending = append(pending, op)
	}
	if !(len(pending) == 0 || (len(pending) == 1 && pending[0].tag == diffEqual)) {
		groups = append(groups, pending)
	}
	return groups
}
