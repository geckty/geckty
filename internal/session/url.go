package session

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/geckty/geckty/internal/vt/emu"
)

// urlPattern matches common http(s)/file URLs and bare www. hosts.
// Kept deliberately simple — not a full RFC parser.
var urlPattern = regexp.MustCompile(`(?i)\b(?:https?://|file://|www\.)[^\s<>"')\]]+`)

// URLAt returns the URL touching col on absLine, if any. Prefers an OSC 8
// hyperlink on that cell; otherwise falls back to plain-text URL detection.
// Trailing punctuation commonly stuck to prose URLs is stripped.
func (s *Session) URLAt(absLine, col int) (url string, ok bool) {
	s.Term.RLock()
	defer s.Term.RUnlock()

	cols := s.Term.Size().C
	if col < 0 || col >= cols {
		return "", false
	}
	if link, found := hyperlinkAt(s.Term.History(), s.Term.Screen(), absLine, col); found {
		return link, true
	}
	runes, lineOK := lineRunesAt(s.Term.History(), s.Term.Screen(), absLine, cols)
	if !lineOK {
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

func hyperlinkAt(history, screen []emu.Line, absLine, col int) (string, bool) {
	var line emu.Line
	switch {
	case absLine < 0:
		return "", false
	case absLine < len(history):
		line = history[absLine]
	case absLine < len(history)+len(screen):
		line = screen[absLine-len(history)]
	default:
		return "", false
	}
	if col < 0 || col >= len(line) {
		return "", false
	}
	h := line[col].Hyperlink
	if h == "" {
		return "", false
	}
	return h, true
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
