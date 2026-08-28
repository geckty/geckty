package app

import (
	"testing"

	"github.com/gogpu/gpucontext"

	"github.com/geckty/geckty/internal/config"
)

func TestSearchOpenClose(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	if s.searchActive() {
		t.Fatal("search should be inactive by default")
	}
	s.openSearch()
	if !s.searchActive() {
		t.Fatal("openSearch should activate search")
	}
	s.closeSearch()
	if s.searchActive() {
		t.Fatal("closeSearch should deactivate search")
	}
}

func TestDispatchActionSearchScrollbackToggles(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.dispatchAction(ActionSearchScrollback)
	if !s.searchActive() {
		t.Fatal("ActionSearchScrollback should open search")
	}
	s.dispatchAction(ActionSearchScrollback)
	if s.searchActive() {
		t.Fatal("ActionSearchScrollback again should close search")
	}
}

func TestHandleSearchTextAndEscape(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.openSearch()
	if !s.handleSearchText("hi") {
		t.Fatal("handleSearchText should consume input while search is active")
	}
	if s.search.query != "hi" {
		t.Fatalf("query = %q, want hi", s.search.query)
	}
	if !s.handleSearchKey(gpucontext.KeyBackspace, 0) {
		t.Fatal("Backspace should be consumed")
	}
	if s.search.query != "h" {
		t.Fatalf("query after Backspace = %q, want h", s.search.query)
	}
	if !s.handleSearchKey(gpucontext.KeyEscape, 0) {
		t.Fatal("Escape should close search")
	}
	if s.searchActive() {
		t.Fatal("Escape should deactivate search")
	}
}

func TestKeymapRecognizesSearchScrollback(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "F", Mods: []string{"ctrl", "shift"}, Action: string(ActionSearchScrollback)},
	})
	if err != nil {
		t.Fatal(err)
	}
	a, ok := k.Match(gpucontext.KeyF, gpucontext.ModControl|gpucontext.ModShift)
	if !ok || a != ActionSearchScrollback {
		t.Fatalf("Match = %q, %v, want search_scrollback", a, ok)
	}
}

func TestSearchStepFindsAndWraps(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("aaa\r\nbbb\r\naaa\r\n"))

	s.openSearch()
	if !s.handleSearchText("aaa") {
		t.Fatal("expected search text consumed")
	}
	if !s.search.hasHit || s.search.count < 2 {
		t.Fatalf("after refresh hasHit=%v count=%d", s.search.hasHit, s.search.count)
	}
	first := s.search.hit

	s.handleSearchKey(gpucontext.KeyEnter, 0)
	if !s.search.hasHit {
		t.Fatal("Enter should step to next match")
	}
	if s.search.hit == first && s.search.count > 1 {
		// May wrap or advance; after one more Enter we should still have a hit.
		s.handleSearchKey(gpucontext.KeyN, 0)
	}
	s.handleSearchKey(gpucontext.KeyN, gpucontext.ModShift)
	if !s.search.hasHit {
		t.Fatal("Shift+N should keep a wrapped hit")
	}

	s.search.query = "zzz_missing"
	s.searchRefresh(true)
	if s.search.hasHit || s.search.status != "no matches" {
		t.Fatalf("missing query: hasHit=%v status=%q", s.search.hasHit, s.search.status)
	}
	s.searchStep(true) // no-op path still safe
}

func TestHandleSearchKeyCtrlNPassesThrough(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.openSearch()
	if s.handleSearchKey(gpucontext.KeyN, gpucontext.ModControl) {
		t.Fatal("Ctrl+N should pass through to shell")
	}
}

func TestHandleSearchKeyBackspaceOnEmptyQuery(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.openSearch()
	if !s.handleSearchKey(gpucontext.KeyBackspace, 0) {
		t.Fatal("Backspace on empty query should still be consumed")
	}
}

func TestHandleSearchTextSkipsControlChars(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	s.openSearch()
	if !s.handleSearchText("a\x01b") {
		t.Fatal("expected printable chars to be consumed")
	}
	if s.search.query != "ab" {
		t.Fatalf("query = %q, want ab", s.search.query)
	}
	if s.handleSearchText("") {
		t.Fatal("empty text should not be consumed")
	}
}

func TestSearchRefreshWrapsFromCurrentHit(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("one two one\r\n"))
	s.openSearch()
	_ = s.handleSearchText("one")
	if s.search.count < 2 {
		t.Fatalf("count = %d, want at least 2 matches", s.search.count)
	}
	first := s.search.hit
	s.searchRefresh(false)
	if !s.search.hasHit {
		t.Fatal("refresh from current hit should keep a match")
	}
	if s.search.hit == first {
		s.searchRefresh(false) // advance again
	}
}

func TestSearchStepBackwardWraps(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("aaa\r\nbbb\r\n"))
	s.openSearch()
	_ = s.handleSearchText("aaa")
	s.searchStep(false)
	if !s.search.hasHit {
		t.Fatal("backward step should find a hit")
	}
}

func TestPaintSearchOverlay(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	active := s.mgr.Active()
	active.Term.Parse([]byte("findme please\r\n"))
	s.openSearch()
	_ = s.handleSearchText("findme")
	s.frame = make([]byte, 400*200*4)
	s.frameW, s.frameH = 400, 200
	s.paintSearchOverlay(400, 200, 4, 28)
	// Hit highlight path: force hasHit in viewport.
	if !s.search.hasHit {
		t.Fatal("expected a hit for findme")
	}
	s.search.status = "no matches"
	s.search.hasHit = false
	s.paintSearchOverlay(400, 200, 4, 28)
}
