package gogpu

import (
	"testing"

	"github.com/geckty/geckty/internal/session"
)

func TestTabBarSectionVisible(t *testing.T) {
	cases := []struct {
		name      string
		hidden    bool
		threshold int
		numTabs   int
		want      bool
	}{
		{"hidden overrides count", true, 1, 5, false},
		{"below threshold", false, 2, 1, false},
		{"at threshold", false, 2, 2, true},
		{"above threshold", false, 2, 3, true},
		{"threshold below 1 behaves as 1", false, 0, 1, true},
		{"threshold below 1 still needs a tab", false, -3, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tabBarSectionVisible(tc.hidden, tc.threshold, tc.numTabs); got != tc.want {
				t.Fatalf("tabBarSectionVisible(%v, %d, %d) = %v, want %v", tc.hidden, tc.threshold, tc.numTabs, got, tc.want)
			}
		})
	}
}

// addTab spawns a real short-lived session and registers it on s.mgr, so
// tabBarShowTabs/tabBarShowPlus see a genuine tab count rather than a
// mocked one.
func addTab(t *testing.T, s *uiState) {
	t.Helper()
	if _, err := s.mgr.New(session.Config{Command: testSleepCommand(), Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("mgr.New: %v", err)
	}
}

func TestTabBarVisibilityDefaultsHideAtOneTab(t *testing.T) {
	s, _ := testUIState(t)
	addTab(t, s)

	if s.tabBarShowTabs() {
		t.Fatal("tab strip should stay hidden with a single tab open (default show_threshold=2)")
	}
	if s.tabBarShowPlus() {
		t.Fatal("plus button should stay hidden with a single tab open (default show_threshold=2)")
	}
	if got := s.tabBarHeightPx(); got != 0 {
		t.Fatalf("tabBarHeightPx() = %d, want 0 when both tabs and plus are hidden", got)
	}

	addTab(t, s)
	if !s.tabBarShowTabs() {
		t.Fatal("tab strip should appear once a second tab opens")
	}
	if !s.tabBarShowPlus() {
		t.Fatal("plus button should appear once a second tab opens")
	}
	if got := s.tabBarHeightPx(); got == 0 {
		t.Fatal("tabBarHeightPx() should be non-zero once the bar becomes visible")
	}
}

func TestTabBarPlusButtonVisibilityIsIndependentOfStrip(t *testing.T) {
	s, _ := testUIState(t)
	s.cfg.TabBar.ShowThreshold = 3            // keep the tab strip hidden longer...
	s.cfg.TabBar.PlusButton.ShowThreshold = 1 // ...while "+" shows from the very first tab.
	addTab(t, s)

	if s.tabBarShowTabs() {
		t.Fatal("tab strip should stay hidden below its own threshold")
	}
	if !s.tabBarShowPlus() {
		t.Fatal("plus button should show independently of the tab strip's threshold")
	}
	if got := s.tabBarHeightPx(); got == 0 {
		t.Fatal("bar row must still be reserved when only the plus button is visible")
	}
}

func TestTabBarHiddenOverridesPlusButton(t *testing.T) {
	s, _ := testUIState(t)
	s.cfg.TabBar.Hidden = true
	s.cfg.TabBar.PlusButton.ShowThreshold = 1
	addTab(t, s)
	addTab(t, s)

	if s.tabBarShowTabs() || s.tabBarShowPlus() {
		t.Fatal("TabBar.Hidden should suppress both the strip and the plus button")
	}
	if got := s.tabBarHeightPx(); got != 0 {
		t.Fatalf("tabBarHeightPx() = %d, want 0 when TabBar.Hidden", got)
	}
}
