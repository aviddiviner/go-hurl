package format

import (
	"fmt"
	"strings"
	"unicode"
)

// jvalue is a JSON value whose objects keep their attribute order.
type jvalue interface{ format(b *strings.Builder) }

type (
	jnumber  string
	jstring  string
	jboolean bool
	jlist    []jvalue
	jobject  []jattr
	jnull    struct{}
)

type jattr struct {
	name  string
	value jvalue
}

func jsonFormat(v jvalue) string {
	var b strings.Builder
	v.format(&b)
	return b.String()
}

func (n jnumber) format(b *strings.Builder) { b.WriteString(string(n)) }

func (s jstring) format(b *strings.Builder) { writeJSONString(b, string(s)) }

func (v jboolean) format(b *strings.Builder) {
	if v {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
}

func (l jlist) format(b *strings.Builder) {
	b.WriteByte('[')
	for i, e := range l {
		if i > 0 {
			b.WriteByte(',')
		}
		e.format(b)
	}
	b.WriteByte(']')
}

func (o jobject) format(b *strings.Builder) {
	b.WriteByte('{')
	for i, a := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		writeJSONString(b, a.name)
		b.WriteByte(':')
		a.value.format(b)
	}
	b.WriteByte('}')
}

func (jnull) format(b *strings.Builder) { b.WriteString("null") }

func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, c := range s {
		b.WriteString(formatJSONChar(c))
	}
	b.WriteByte('"')
}

func formatJSONChar(c rune) string {
	switch {
	case c == '"':
		return `\"`
	case c == '\\':
		return `\\`
	case c == '\b':
		return `\b`
	case c == '\f':
		return `\f`
	case c == '\n':
		return `\n`
	case c == '\r':
		return `\r`
	case c == '\t':
		return `\t`
	case unicode.IsControl(c):
		return fmt.Sprintf(`\u%04x`, c)
	}
	return string(c)
}
