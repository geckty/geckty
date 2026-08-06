package session

import (
	"regexp"
	"strconv"
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

// URLHit is one URL found in History()+Screen() (OSC 8 or plain-text).
type URLHit struct {
	URL          string
	AbsLine, Col int
}

// CollectURLs scans History()+Screen() for OSC 8 hyperlinks and plain
// urlPattern matches, deduped by URL+AbsLine. Caps at max (default 64 when
// max <= 0).
func (s *Session) CollectURLs(max int) []URLHit {
	if max <= 0 {
		max = 64
	}
	s.Term.RLock()
	defer s.Term.RUnlock()

	history := s.Term.History()
	screen := s.Term.Screen()
	cols := s.Term.Size().C
	total := len(history) + len(screen)
	if total == 0 || cols <= 0 {
		return nil
	}

	seen := make(map[string]bool)
	out := make([]URLHit, 0, 8)
	add := func(url string, abs, col int) {
		if url == "" || len(out) >= max {
			return
		}
		key := url + "\x00" + strconv.Itoa(abs)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, URLHit{URL: url, AbsLine: abs, Col: col})
	}

	for abs := 0; abs < total && len(out) < max; abs++ {
		var line emu.Line
		switch {
		case abs < len(history):
			line = history[abs]
		default:
			line = screen[abs-len(history)]
		}
		var prevLink string
		for col := 0; col < len(line) && col < cols; col++ {
			h := line[col].Hyperlink
			if h != "" && h != prevLink {
				add(h, abs, col)
			}
			prevLink = h
		}
		runes, lineOK := lineRunesAt(history, screen, abs, cols)
		if !lineOK {
			continue
		}
		text := strings.TrimRightFunc(string(runes), unicode.IsSpace)
		for _, m := range urlPattern.FindAllStringIndex(text, -1) {
			add(trimURLTrailer(text[m[0]:m[1]]), abs, m[0])
		}
	}
	return out
}
