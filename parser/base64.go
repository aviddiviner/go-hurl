package parser

import "github.com/aviddiviner/go-hurl/reader"

// base64Parse decodes base64 text (with or without padding, ignoring
// whitespace) until the first non-base64 character.
func base64Parse(r *reader.Reader) []byte {
	var bytes []byte
	var buf []int
	for {
		if padding(r) != "" {
			break
		}
		save := r.Cursor()
		c, ok := r.Read()
		if !ok {
			break
		}
		if c == ' ' || c == '\n' || c == '\t' {
			continue
		}
		v, ok := base64Value(c)
		if !ok {
			r.Seek(save)
			break
		}
		buf = append(buf, v)
		if len(buf) == 4 {
			bytes = append(bytes, decodeFourChars(buf[0], buf[1], buf[2], buf[3])...)
			buf = buf[:0]
		}
	}
	switch len(buf) {
	case 2:
		bytes = append(bytes, decodeTwoChars(buf[0], buf[1])...)
	case 3:
		bytes = append(bytes, decodeThreeChars(buf[0], buf[1], buf[2])...)
	}
	return bytes
}

func base64Value(c rune) (int, bool) {
	switch {
	case c >= 'A' && c <= 'Z':
		return int(c - 'A'), true
	case c >= 'a' && c <= 'z':
		return int(c-'a') + 26, true
	case c >= '0' && c <= '9':
		return int(c-'0') + 52, true
	case c == '+':
		return 62, true
	case c == '/':
		return 63, true
	}
	return 0, false
}

func padding(r *reader.Reader) string {
	return r.ReadWhile(func(c rune) bool { return c == '=' })
}

func decodeTwoChars(c1, c2 int) []byte {
	return []byte{byte(((c1 << 2) & 255) + (c2 >> 4))}
}

func decodeThreeChars(c1, c2, c3 int) []byte {
	return []byte{
		byte(((c1 << 2) & 255) + (c2 >> 4)),
		byte(((c2 << 4) & 255) + (c3 >> 2)),
	}
}

func decodeFourChars(c1, c2, c3, c4 int) []byte {
	return []byte{
		byte(((c1 << 2) & 255) + (c2 >> 4)),
		byte(((c2 << 4) & 255) + (c3 >> 2)),
		byte(((c3 << 6) & 255) + c4),
	}
}
