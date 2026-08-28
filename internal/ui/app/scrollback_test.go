package app

import (
	"runtime"
	"strings"
	"testing"
)

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellQuote("it's/a/path")
	if !strings.Contains(got, `'"'"'`) {
		t.Fatalf("shellQuote = %q, want escaped single quotes", got)
	}
	if got != `'it'"'"'s/a/path'` {
		t.Fatalf("shellQuote = %q", got)
	}
}

func TestShowScrollbackInPagerNoActiveTab(t *testing.T) {
	s, _ := testUIState(t)
	s.showScrollbackInPager() // must not panic
}

func TestShowScrollbackInPagerDefaultLess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("default less path is unix-specific")
	}
	s, _ := testUIStateWithTab(t)
	t.Setenv("PAGER", "")
	active := s.mgr.Active()
	if active == nil {
		t.Fatal("expected active tab")
	}
	active.Term.Parse([]byte("scrollback line\r\n"))
	s.showScrollbackInPager()
}
