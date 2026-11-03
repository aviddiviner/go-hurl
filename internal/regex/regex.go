// Package regex compiles Hurl regexes with Go's regexp, rejecting the
// counted repetitions that Rust's regex crate (used by Hurl) rejects.
package regex

import (
	"errors"
	"regexp"
	"strconv"
)

// Compile compiles pattern. Errors from the repetition check have Rust's
// regex messages; the others are regexp's.
func Compile(pattern string) (*regexp.Regexp, error) {
	if message := countedRepetitionError(pattern); message != "" {
		return nil, errors.New(message)
	}
	return regexp.Compile(pattern)
}

// countedRepetitionError checks the counted repetitions ({n}, {n,}, {n,m})
// of pattern like Rust's regex crate: there, '{' always starts a
// repetition, where Go's regexp reads an invalid one as a literal.
func countedRepetitionError(pattern string) string {
	rs := []rune(pattern)
	digits := func(i int) (int, int, bool) {
		j := i
		for j < len(rs) && rs[j] >= '0' && rs[j] <= '9' {
			j++
		}
		n, err := strconv.Atoi(string(rs[i:j]))
		return j, n, err == nil
	}
	afterExpr := false
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\\':
			i++
		case '[':
			i = skipClass(rs, i)
		case '|':
			afterExpr = false
			continue
		case '(':
			afterExpr = false
			// Skip a group header: (?:, (?flags:, (?flags), (?P<name>, (?<name>.
			if i+1 < len(rs) && rs[i+1] == '?' {
				for i += 2; i < len(rs) && rs[i] != ':' && rs[i] != ')' && rs[i] != '>'; i++ {
				}
			}
			continue
		case '{':
			const unclosed, decimal = "unclosed counted repetition", "repetition quantifier expects a valid decimal"
			if !afterExpr {
				return "repetition operator missing expression"
			}
			j, lo, loOk := digits(i + 1)
			if j >= len(rs) {
				return unclosed
			}
			if rs[j] == ',' {
				j++
				if j >= len(rs) {
					return unclosed
				}
				if !loOk {
					return decimal
				}
				if rs[j] != '}' {
					var hi int
					var hiOk bool
					if j, hi, hiOk = digits(j); !hiOk {
						return decimal
					}
					if j < len(rs) && rs[j] == '}' && lo > hi {
						return "invalid repetition count range, the start must be <= the end"
					}
				}
			} else if !loOk {
				return decimal
			}
			if j >= len(rs) || rs[j] != '}' {
				return unclosed
			}
			i = j
		}
		afterExpr = true
	}
	return ""
}

// skipClass returns the index of the ']' closing the character class
// opened at rs[i], allowing nested and ASCII ([:alpha:]) classes.
func skipClass(rs []rune, i int) int {
	depth := 0
	for ; i < len(rs); i++ {
		switch rs[i] {
		case '\\':
			i++
		case '[':
			if end := asciiClassEnd(rs, i); end > 0 {
				i = end
				continue
			}
			depth++
			if i+1 < len(rs) && rs[i+1] == '^' {
				i++
			}
			if i+1 < len(rs) && rs[i+1] == ']' {
				i++
			}
		case ']':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return i
}

// asciiClassEnd returns the index of the ']' of an ASCII class "[:name:]"
// at rs[i], or -1.
func asciiClassEnd(rs []rune, i int) int {
	if i+1 >= len(rs) || rs[i+1] != ':' {
		return -1
	}
	for k := i + 2; k+1 < len(rs); k++ {
		if rs[k] == ':' && rs[k+1] == ']' {
			return k + 1
		}
	}
	return -1
}
