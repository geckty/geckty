package gogpu

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
