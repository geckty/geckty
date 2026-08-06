package session

import (
	"regexp"
	"strings"
	"unicode"
)

// urlPattern matches common http(s)/file URLs and bare www. hosts.
// Kept deliberately simple — not a full RFC parser.
var urlPattern = regexp.MustCompile(`(?i)\b(?:https?://|file://|www\.)[^\s<>"')\]]+`)

// URLAt returns the URL touching col on absLine, if any. Trailing
// punctuation commonly stuck to URLs in prose is stripped.
func (s *Session) URLAt(absLine, col int) (url string, ok bool) {
	s.Term.RLock()
	defer s.Term.RUnlock()

	cols := s.Term.Size().C
	runes, lineOK := lineRunesAt(s.Term.History(), s.Term.Screen(), absLine, cols)
	if !lineOK || col < 0 || col >= cols {
		return "", false
	}
	line := strings.TrimRightFunc(string(runes), unicode.IsSpace)
	matches := urlPattern.FindAllStringIndex(line, -1)
	for _, m := range matches {
		if col >= m[0] && col < m[1] {
			return trimURLTrailer(line[m[0]:m[1]]), true
		}
	}
	return "", false
}

func trimURLTrailer(u string) string {
	for len(u) > 0 {
		r := rune(u[len(u)-1])
		switch r {
		case '.', ',', ';', ':', '!', '?', ')', ']', '}':
			u = u[:len(u)-1]
			continue
		}
		break
	}
	if strings.HasPrefix(strings.ToLower(u), "www.") {
		return "https://" + u
	}
	return u
}
