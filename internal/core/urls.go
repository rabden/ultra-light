package core

import (
	"fmt"
	"strings"
)

// URL returns the address a web row opens for query, or "" for rows that
// open none.
func URL(kind Kind, query string) string {
	var base string
	switch kind {
	case KindAskGrok:
		base = "https://grok.com/?q="
	case KindSearchX:
		base = "https://x.com/search?f=live&q="
	case KindSearchWeb, KindSuggest:
		base = GoogleSearch
	default:
		return ""
	}
	return base + Escape(query)
}

// GoogleSearch is the start of a Google search's address.
const GoogleSearch = "https://www.google.com/search?q="

// Escape writes query, trimmed of spaces, as a URL's query value: plain
// bytes as they are, spaces as +, the rest as %XX.
func Escape(query string) string {
	var b strings.Builder
	q := strings.Trim(query, " ")
	for i := 0; i < len(q); i++ {
		ch := q[i]
		switch {
		case isAlpha(ch) || isDigit(ch) || ch == '-' || ch == '_' || ch == '.' || ch == '~':
			b.WriteByte(ch)
		case ch == ' ':
			b.WriteByte('+')
		default:
			fmt.Fprintf(&b, "%%%02X", ch)
		}
	}
	return b.String()
}
