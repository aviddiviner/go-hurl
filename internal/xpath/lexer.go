package xpath

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// xpath1Functions are the XPath 1.0 core functions, the only ones libxml2 registers without a
// namespace. antchfx/xpath also implements some XPath 2.0 functions, which we reject.
var xpath1Functions = map[string]bool{
	"last": true, "position": true, "count": true, "id": true, "local-name": true,
	"namespace-uri": true, "name": true, "string": true, "concat": true, "starts-with": true,
	"contains": true, "substring-before": true, "substring-after": true, "substring": true,
	"string-length": true, "normalize-space": true, "translate": true, "boolean": true,
	"not": true, "true": true, "false": true, "lang": true, "number": true, "sum": true,
	"floor": true, "ceiling": true, "round": true,
}

var nodeTypes = map[string]bool{
	"comment": true, "text": true, "processing-instruction": true, "node": true,
}

type tokenKind int

const (
	tokNone tokenKind = iota
	tokOperator
	tokOpen // one of ( [ , @ ::
	tokOther
)

// preprocess scans expr following the XPath 1.0 lexical rules (section 3.7). It reports false if
// expr calls a function that isn't an XPath 1.0 core function, or uses the namespace axis, which
// antchfx/xpath doesn't support (it loops forever). Prefix wildcards p:* are rewritten
// with a namespace-uri() predicate, as antchfx/xpath never matches them. If noNSPrefix isn't
// empty, every unprefixed name test is rewritten as noNSPrefix:name.
func preprocess(expr string, namespaces map[string]string, noNSPrefix string) (string, bool) {
	var out strings.Builder
	prev := tokNone
	i := 0
	for i < len(expr) {
		r, size := utf8.DecodeRuneInString(expr[i:])
		if strings.ContainsRune(" \t\r\n", r) {
			out.WriteRune(r)
			i += size
			continue
		}
		start := i
		operatorContext := prev == tokNone || prev == tokOperator || prev == tokOpen
		switch {
		case r == '"' || r == '\'':
			end := strings.IndexRune(expr[i+1:], r)
			if end < 0 {
				return expr, true
			}
			i += end + 2
			prev = tokOther
		case r == '$':
			i++
			i = scanQName(expr, i)
			prev = tokOther
		case isDigit(r) || (r == '.' && i+1 < len(expr) && isDigit(rune(expr[i+1]))):
			i++
			for i < len(expr) && (isDigit(rune(expr[i])) || expr[i] == '.') {
				i++
			}
			prev = tokOther
		case r == '*':
			i++
			if operatorContext {
				prev = tokOther
			} else {
				prev = tokOperator
			}
		case isNameStart(r):
			i = scanNCName(expr, i)
			name := expr[start:i]
			if !operatorContext && (name == "and" || name == "or" || name == "div" || name == "mod") {
				prev = tokOperator
				break
			}
			next := skipSpace(expr, i)
			if strings.HasPrefix(expr[next:], "::") {
				if name == "namespace" {
					return "", false
				}
				prev = tokOther
				break
			}
			prefixed := false
			if i+1 < len(expr) && expr[i] == ':' {
				if expr[i+1] == '*' {
					i += 2
					prev = tokOther
					if uri, ok := namespaces[name]; ok {
						q := "'"
						if strings.Contains(uri, q) {
							q = `"`
						}
						out.WriteString("*[namespace-uri()=" + q + uri + q + "]")
						continue
					}
					break
				}
				if r, _ := utf8.DecodeRuneInString(expr[i+1:]); isNameStart(r) {
					prefixed = true
					i = scanNCName(expr, i+1)
					next = skipSpace(expr, i)
				}
			}
			if next < len(expr) && expr[next] == '(' {
				if prefixed || (!nodeTypes[name] && !xpath1Functions[name]) {
					return "", false
				}
			} else if !prefixed && noNSPrefix != "" {
				out.WriteString(noNSPrefix + ":")
			}
			prev = tokOther
		default:
			i += size
			for _, op := range []string{"//", "::", "!=", "<=", ">=", ".."} {
				if strings.HasPrefix(expr[start:], op) {
					i = start + len(op)
					break
				}
			}
			switch expr[start:i] {
			case "(", "[", ",", "@", "::":
				prev = tokOpen
			case "/", "//", "|", "+", "-", "=", "!=", "<", "<=", ">", ">=":
				prev = tokOperator
			default:
				prev = tokOther
			}
		}
		out.WriteString(expr[start:i])
	}
	return out.String(), true
}

func skipSpace(s string, i int) int {
	for i < len(s) && strings.IndexByte(" \t\r\n", s[i]) >= 0 {
		i++
	}
	return i
}

func scanQName(s string, i int) int {
	i = scanNCName(s, i)
	if i+1 < len(s) && s[i] == ':' {
		if r, _ := utf8.DecodeRuneInString(s[i+1:]); isNameStart(r) {
			i = scanNCName(s, i+1)
		}
	}
	return i
}

func scanNCName(s string, i int) int {
	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		if !isNameChar(r) {
			break
		}
		i += size
	}
	return i
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func isNameStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isNameChar(r rune) bool {
	return isNameStart(r) || isDigit(r) || r == '-' || r == '.' || r == 0xB7 ||
		unicode.IsDigit(r) || unicode.In(r, unicode.Mn, unicode.Mc)
}
