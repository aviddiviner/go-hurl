package curlimport

import (
	"fmt"
	"strings"

	"github.com/aviddiviner/go-hurl/reader"
)

// split splits a command line into its params, handling 'quoted' and
// $'escaped' params and backslash escapes.
func split(s string) ([]string, error) {
	var params []string
	p := newArgsParser(s)
	for {
		param, ok, err := p.param()
		if err != nil {
			return nil, err
		}
		if !ok {
			return params, nil
		}
		params = append(params, param)
	}
}

type argsParser struct {
	reader *reader.Reader
}

func newArgsParser(s string) *argsParser { return &argsParser{reader: reader.New(s)} }

// delimiter reads an opening delimiter, returning it and whether escapes are
// interpreted ($'...').
func (p *argsParser) delimiter() (rune, bool, bool) {
	c, _ := p.reader.Peek()
	switch c {
	case '\'':
		p.reader.Read()
		return '\'', false, true
	case '$':
		save := p.reader.Cursor()
		p.reader.Read()
		if c, _ := p.reader.Peek(); c == '\'' {
			p.reader.Read()
			return '\'', true, true
		}
		p.reader.Seek(save)
	}
	return 0, false, false
}

func (p *argsParser) col() int { return p.reader.Cursor().Pos.Column }

func (p *argsParser) param() (string, bool, error) {
	p.reader.ReadWhile(func(c rune) bool { return c == ' ' })
	if p.reader.IsEOF() {
		return "", false, nil
	}
	var value strings.Builder
	if delimiter, escaping, ok := p.delimiter(); ok {
		for {
			c1, ok := p.reader.Read()
			if !ok {
				break
			}
			switch {
			case c1 == '\\' && escaping:
				c2, ok := p.reader.Read()
				if !ok {
					return "", false, fmt.Errorf("Invalid escape at column %d", p.col())
				}
				switch c2 {
				case 'n':
					c2 = '\n'
				case 't':
					c2 = '\t'
				case 'r':
					c2 = '\r'
				}
				value.WriteRune(c2)
			case c1 == delimiter:
				return value.String(), true, nil
			default:
				value.WriteRune(c1)
			}
		}
		return "", false, fmt.Errorf("Missing delimiter %c at column %d", delimiter, p.col())
	}
	for {
		c, ok := p.reader.Read()
		switch {
		case !ok, c == ' ':
			return value.String(), true, nil
		case c == '\\':
			c, ok := p.reader.Read()
			if !ok {
				return "", false, fmt.Errorf("Invalid escape at column %d", p.col())
			}
			value.WriteRune(c)
		default:
			value.WriteRune(c)
		}
	}
}
