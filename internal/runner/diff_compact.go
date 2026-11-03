package runner

// cleanupDiffOps is similar's Compact hook (cleanup_diff_ops): it shifts
// deletions and insertions up and down to merge them with adjacent ones,
// then normalizes the cursors of the operations.
func cleanupDiffOps(old, new []string, raw diffOps) []diffOp {
	c := opsCompactor{ops: []diffOp(raw), old: old, new: new}
	for _, tag := range []diffTag{diffDelete, diffInsert} {
		for p := 0; p < len(c.ops); p++ {
			if c.ops[p].tag == tag {
				p = c.shiftUp(p)
				p = c.shiftDown(p)
			}
		}
	}
	normalizeDiffOpCursors(c.ops)
	return c.ops
}

func (op diffOp) isEmpty() bool { return op.oldLen == 0 && op.newLen == 0 }
func (op diffOp) oldEnd() int   { return op.oldIndex + op.oldLen }
func (op diffOp) newEnd() int   { return op.newIndex + op.newLen }

func (op *diffOp) adjust(offset, length int) {
	op.oldIndex += offset
	op.newIndex += offset
	switch op.tag {
	case diffEqual, diffReplace:
		op.oldLen += length
		op.newLen += length
	case diffDelete:
		op.oldLen += length
	case diffInsert:
		op.newLen += length
	}
}

func (op *diffOp) growLeft(n int)    { op.adjust(-n, n) }
func (op *diffOp) growRight(n int)   { op.adjust(0, n) }
func (op *diffOp) shiftLeft(n int)   { op.adjust(-n, 0) }
func (op *diffOp) shiftRight(n int)  { op.adjust(n, 0) }
func (op *diffOp) shrinkLeft(n int)  { op.adjust(0, -n) }
func (op *diffOp) shrinkRight(n int) { op.adjust(n, -n) }

type opsCompactor struct {
	ops      []diffOp
	old, new []string
}

func (c *opsCompactor) insertAt(i int, op diffOp) {
	c.ops = append(c.ops, diffOp{})
	copy(c.ops[i+1:], c.ops[i:])
	c.ops[i] = op
}

func (c *opsCompactor) removeAt(i int) { c.ops = append(c.ops[:i], c.ops[i+1:]...) }

func (c *opsCompactor) tagAt(i int) (diffTag, bool) {
	if i < 0 || i >= len(c.ops) {
		return 0, false
	}
	return c.ops[i].tag, true
}

func (c *opsCompactor) swapInsertDelete(left int) {
	right := left + 1
	oldStart, oldEnd := c.ops[left].oldIndex, c.ops[right].oldEnd()
	newStart, newEnd := c.ops[left].newIndex, c.ops[right].newEnd()
	c.ops[left], c.ops[right] = c.ops[right], c.ops[left]
	if c.ops[left].tag == diffInsert {
		c.ops[left].oldIndex = oldStart
	} else {
		c.ops[left].newIndex = newStart
	}
	if c.ops[right].tag == diffInsert {
		c.ops[right].oldIndex = oldEnd
	} else {
		c.ops[right].newIndex = newEnd
	}
}

func (c *opsCompactor) shiftUp(p int) int {
	for p > 0 {
		this, prev := c.ops[p], c.ops[p-1]
		switch {
		case (this.tag == diffInsert || this.tag == diffDelete) && prev.tag == diffEqual:
			var suffix int
			var eqOld, eqNew int
			if this.tag == diffInsert {
				suffix = commonSuffixLen(c.old, prev.oldIndex, prev.oldEnd(), c.new, this.newIndex, this.newEnd())
				eqOld, eqNew = prev.oldEnd()-suffix, this.newEnd()-suffix
			} else {
				if t, ok := c.tagAt(p - 2); !ok || t != diffDelete {
					return p
				}
				suffix = commonSuffixLen(c.old, this.oldIndex, this.oldEnd(), c.new, prev.newIndex, prev.newEnd())
				eqOld, eqNew = this.oldEnd()-suffix, prev.newEnd()-suffix
			}
			switch {
			case suffix > 0:
				if t, ok := c.tagAt(p + 1); ok && t == diffEqual {
					c.ops[p+1].growLeft(suffix)
				} else {
					c.insertAt(p+1, diffOp{tag: diffEqual, oldIndex: eqOld, oldLen: suffix, newIndex: eqNew, newLen: suffix})
				}
				c.ops[p].shiftLeft(suffix)
				c.ops[p-1].shrinkLeft(suffix)
				if c.ops[p-1].isEmpty() {
					c.removeAt(p - 1)
					p--
				}
			case c.ops[p-1].isEmpty():
				c.removeAt(p - 1)
				p--
			default:
				return p
			}
		case this.tag != prev.tag && (this.tag == diffInsert || this.tag == diffDelete) && (prev.tag == diffInsert || prev.tag == diffDelete):
			c.swapInsertDelete(p - 1)
			p--
		case this.tag == diffInsert && prev.tag == diffInsert:
			c.ops[p-1].growRight(this.newLen)
			c.removeAt(p)
			p--
		case this.tag == diffDelete && prev.tag == diffDelete:
			c.ops[p-1].growRight(this.oldLen)
			c.removeAt(p)
			p--
		default:
			panic("unexpected tag")
		}
	}
	return p
}

func (c *opsCompactor) shiftDown(p int) int {
	for p+1 < len(c.ops) {
		this, next := c.ops[p], c.ops[p+1]
		switch {
		case (this.tag == diffInsert || this.tag == diffDelete) && next.tag == diffEqual:
			var prefix int
			var eqOld, eqNew int
			if this.tag == diffInsert {
				prefix = commonPrefixLen(c.old, next.oldIndex, next.oldEnd(), c.new, this.newIndex, this.newEnd())
				eqOld, eqNew = next.oldIndex, this.newIndex
			} else {
				if t, ok := c.tagAt(p + 2); !ok || t != diffDelete {
					return p
				}
				prefix = commonPrefixLen(c.old, this.oldIndex, this.oldEnd(), c.new, next.newIndex, next.newEnd())
				eqOld, eqNew = this.oldIndex, this.newIndex
			}
			switch {
			case prefix > 0:
				if t, ok := c.tagAt(p - 1); ok && t == diffEqual {
					c.ops[p-1].growRight(prefix)
				} else {
					c.insertAt(p, diffOp{tag: diffEqual, oldIndex: eqOld, oldLen: prefix, newIndex: eqNew, newLen: prefix})
					p++
				}
				c.ops[p].shiftRight(prefix)
				c.ops[p+1].shrinkRight(prefix)
				if c.ops[p+1].isEmpty() {
					c.removeAt(p + 1)
				}
			case c.ops[p+1].isEmpty():
				c.removeAt(p + 1)
			default:
				return p
			}
		case this.tag != next.tag && (this.tag == diffInsert || this.tag == diffDelete) && (next.tag == diffInsert || next.tag == diffDelete):
			c.swapInsertDelete(p)
			p++
		case this.tag == diffInsert && next.tag == diffInsert:
			c.ops[p].growRight(next.newLen)
			c.removeAt(p + 1)
		case this.tag == diffDelete && next.tag == diffDelete:
			c.ops[p].growRight(next.oldLen)
			c.removeAt(p + 1)
		default:
			panic("unexpected tag")
		}
	}
	return p
}

func normalizeDiffOpCursors(ops []diffOp) {
	if len(ops) == 0 {
		return
	}
	oldCursor, newCursor := ops[0].oldIndex, ops[0].newIndex
	for i := range ops {
		op := &ops[i]
		switch op.tag {
		case diffEqual, diffReplace:
			oldCursor, newCursor = op.oldEnd(), op.newEnd()
		case diffDelete:
			op.newIndex = newCursor
			oldCursor = op.oldEnd()
		case diffInsert:
			op.oldIndex = oldCursor
			newCursor = op.newEnd()
		}
	}
}
