package runner

import (
	"bytes"
	"net/netip"
	"regexp"
	"strings"

	"github.com/aviddiviner/go-hurl/internal/regex"
	"github.com/aviddiviner/go-hurl/internal/timefmt"
)

// valueCompare compares two strings, numbers or dates; it returns -1, 0 or
// +1, or EvalErrorType for other types.
func valueCompare(v, other Value) (int, error) {
	switch v := v.(type) {
	case ValueString:
		if o, ok := other.(ValueString); ok {
			return strings.Compare(v.Value, o.Value), nil
		}
	case ValueNumber:
		if o, ok := other.(ValueNumber); ok {
			return cmpNumber(v.Value, o.Value), nil
		}
	case ValueDate:
		if o, ok := other.(ValueDate); ok {
			return v.Value.Compare(o.Value), nil
		}
	}
	return 0, EvalErrorType
}

func valueStartsWith(v, other Value) (bool, error) {
	switch v := v.(type) {
	case ValueString:
		if o, ok := other.(ValueString); ok {
			return strings.HasPrefix(v.Value, o.Value), nil
		}
	case ValueBytes:
		if o, ok := other.(ValueBytes); ok {
			return bytes.HasPrefix(v.Value, o.Value), nil
		}
	}
	return false, EvalErrorType
}

func valueEndsWith(v, other Value) (bool, error) {
	switch v := v.(type) {
	case ValueString:
		if o, ok := other.(ValueString); ok {
			return strings.HasSuffix(v.Value, o.Value), nil
		}
	case ValueBytes:
		if o, ok := other.(ValueBytes); ok {
			return bytes.HasSuffix(v.Value, o.Value), nil
		}
	}
	return false, EvalErrorType
}

func valueContains(v, other Value) (bool, error) {
	switch v := v.(type) {
	case ValueString:
		if o, ok := other.(ValueString); ok {
			return strings.Contains(v.Value, o.Value), nil
		}
	case ValueBytes:
		if o, ok := other.(ValueBytes); ok {
			return bytes.Contains(v.Value, o.Value), nil
		}
	case ValueList:
		return listIncludes(v.Value, other), nil
	}
	return false, EvalErrorType
}

// valueIncludes is Value::includes (to be deprecated in favor of contains).
func valueIncludes(v, other Value) (bool, error) {
	if l, ok := v.(ValueList); ok {
		return listIncludes(l.Value, other), nil
	}
	return false, EvalErrorType
}

func listIncludes(values []Value, other Value) bool {
	for _, e := range values {
		if Equal(e, other) {
			return true
		}
	}
	return false
}

func valueIsBoolean(v Value) bool { return v.Kind() == ValueKindBool }

func valueIsList(v Value) bool { k := v.Kind(); return k == ValueKindBytes || k == ValueKindList }

func valueIsObject(v Value) bool { k := v.Kind(); return k == ValueKindNodeset || k == ValueKindObject }

func valueIsCollection(v Value) bool {
	switch v.Kind() {
	case ValueKindBytes, ValueKindList, ValueKindNodeset, ValueKindObject:
		return true
	}
	return false
}

func valueIsDate(v Value) bool    { return v.Kind() == ValueKindDate }
func valueIsFloat(v Value) bool   { return v.Kind() == ValueKindFloat }
func valueIsInteger(v Value) bool { return v.Kind() == ValueKindInteger }

func valueIsNumber(v Value) bool {
	k := v.Kind()
	return k == ValueKindInteger || k == ValueKindFloat
}

func valueIsString(v Value) bool {
	k := v.Kind()
	return k == ValueKindString || k == ValueKindSecret
}

// parseIPAddr is Rust's IpAddr::from_str (no IPv6 zones).
func parseIPAddr(s string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(s)
	if err != nil || addr.Zone() != "" {
		return netip.Addr{}, false
	}
	return addr, true
}

func valueIsIPv4(v Value) (bool, error) {
	s, ok := v.(ValueString)
	if !ok {
		return false, EvalErrorType
	}
	addr, ok := parseIPAddr(s.Value)
	return ok && addr.Is4(), nil
}

func valueIsIPv6(v Value) (bool, error) {
	s, ok := v.(ValueString)
	if !ok {
		return false, EvalErrorType
	}
	addr, ok := parseIPAddr(s.Value)
	return ok && addr.Is6(), nil
}

func valueIsUUID(v Value) (bool, error) {
	s, ok := v.(ValueString)
	if !ok {
		return false, EvalErrorType
	}
	return isUUID(s.Value), nil
}

// isUUID is uuid::Uuid::try_parse: simple, hyphenated, braced or URN forms.
func isUUID(s string) bool {
	if rest, ok := strings.CutPrefix(s, "urn:uuid:"); ok {
		s = rest
	} else if len(s) >= 2 && s[0] == '{' && s[len(s)-1] == '}' {
		s = s[1 : len(s)-1]
	}
	isHex := func(c byte) bool {
		return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
	}
	switch len(s) {
	case 32:
		for i := range len(s) {
			if !isHex(s[i]) {
				return false
			}
		}
		return true
	case 36:
		for i := range len(s) {
			if i == 8 || i == 13 || i == 18 || i == 23 {
				if s[i] != '-' {
					return false
				}
			} else if !isHex(s[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// valueIsISODate reports whether a string value is an RFC 3339 date.
func valueIsISODate(v Value) (bool, error) {
	s, ok := v.(ValueString)
	if !ok {
		return false, EvalErrorType
	}
	_, err := timefmt.ParseRFC3339(s.Value)
	return err == nil, nil
}

// valueCount returns the count of a list, string (in bytes), node-set,
// object or bytes.
func valueCount(v Value) (int, error) {
	switch v := v.(type) {
	case ValueList:
		return len(v.Value), nil
	case ValueString:
		return len(v.Value), nil
	case ValueNodeset:
		return v.Value, nil
	case ValueObject:
		return len(v.Value), nil
	case ValueBytes:
		return len(v.Value), nil
	}
	return 0, EvalErrorType
}

// valueIsMatch reports whether the regex other (a string or a regex)
// matches anywhere in the string v.
func valueIsMatch(v, other Value) (bool, error) {
	var re *regexp.Regexp
	switch o := other.(type) {
	case ValueString:
		var err error
		if re, err = regex.Compile(o.Value); err != nil {
			return false, EvalErrorInvalidRegex
		}
	case ValueRegex:
		re = o.Value
	default:
		return false, EvalErrorType
	}
	s, ok := v.(ValueString)
	if !ok {
		return false, EvalErrorType
	}
	return re.MatchString(s.Value), nil
}
