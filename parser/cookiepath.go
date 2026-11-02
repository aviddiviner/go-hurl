package parser

import (
	"strings"
	"unicode"

	"github.com/aviddiviner/go-hurl/ast"
	"github.com/aviddiviner/go-hurl/reader"
)

func cookiepath(r *reader.Reader) (ast.CookiePath, *ParseError) {
	start := r.Cursor().Pos

	// The name is parsed with a dedicated reader, whose positions are
	// relative to the main reader.
	s := r.ReadWhile(func(c rune) bool { return c != '[' })
	name, err := unquotedTemplate(reader.WithPos(s, start))
	if err != nil {
		return ast.CookiePath{}, err
	}
	attribute, err := optional(cookiepathAttribute, r)
	if err != nil {
		return ast.CookiePath{}, err
	}
	return ast.CookiePath{Name: name, Attribute: attribute}, nil
}

func cookiepathAttribute(r *reader.Reader) (ast.CookieAttribute, *ParseError) {
	if err := tryLiteral("[", r); err != nil {
		return ast.CookieAttribute{}, err
	}
	space0, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.CookieAttribute{}, err
	}
	name, err := cookiepathAttributeName(r)
	if err != nil {
		return ast.CookieAttribute{}, err
	}
	space1, err := zeroOrMoreSpaces(r)
	if err != nil {
		return ast.CookieAttribute{}, err
	}
	if err := literal("]", r); err != nil {
		return ast.CookieAttribute{}, err
	}
	return ast.CookieAttribute{Space0: space0, Name: name, Space1: space1}, nil
}

func cookiepathAttributeName(r *reader.Reader) (ast.CookieAttributeName, *ParseError) {
	start := r.Cursor().Pos
	s := r.ReadWhile(func(c rune) bool { return isAlphabetic(c) || c == '-' })
	var kind ast.CookieAttributeKind
	switch strings.ToLower(s) {
	case "value":
		kind = ast.CookieAttributeValue
	case "expires":
		kind = ast.CookieAttributeExpires
	case "max-age":
		kind = ast.CookieAttributeMaxAge
	case "domain":
		kind = ast.CookieAttributeDomain
	case "path":
		kind = ast.CookieAttributePath
	case "secure":
		kind = ast.CookieAttributeSecure
	case "httponly":
		kind = ast.CookieAttributeHTTPOnly
	case "samesite":
		kind = ast.CookieAttributeSameSite
	default:
		return ast.CookieAttributeName{}, newError(start, false, ParseErrorInvalidCookieAttribute{})
	}
	return ast.CookieAttributeName{Kind: kind, Value: s}, nil
}

// isAlphabetic matches Rust's char::is_alphabetic.
func isAlphabetic(c rune) bool {
	return unicode.IsLetter(c) || unicode.Is(unicode.Nl, c) || unicode.Is(unicode.Other_Alphabetic, c)
}
