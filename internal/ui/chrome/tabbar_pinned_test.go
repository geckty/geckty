package chrome

import "testing"

func TestTabAtScrolledPinnedOutsideClusterIsMiss(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := TabAtScrolledPinned(-1, 0, g, 4, 0); got != -1 {
		t.Fatalf("x<0 = %d, want -1", got)
	}
	if got := TabAtScrolledPinned(g.PlusX, 0, g, 4, 0); got != -1 {
		t.Fatalf("x==PlusX = %d, want -1", got)
	}
}

func TestTabAtScrolledPinnedFallsBackWhenNotPinned(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	// With few enough tabs to fit, ActiveTabPin never pins, so this should
	// behave exactly like TabAtScrolled.
	want := TabAtScrolled(10, 0, g, 4)
	got := TabAtScrolledPinned(10, 0, g, 4, 0)
	if got != want {
		t.Fatalf("TabAtScrolledPinned = %d, want %d (same as TabAtScrolled when unpinned)", got, want)
	}
}

func TestTabAtScrolledPinnedLeft(t *testing.T) {
	// Many tabs overflow the strip; scroll far enough right that tab 0
	// (the active tab) is cut off on the left -> pinL.
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	scroll := g.TabWidth * 5
	if got := TabAtScrolledPinned(10, scroll, g, 12, 0); got != 0 {
		t.Fatalf("TabAtScrolledPinned near left edge while pinned-left = %d, want 0 (active idx)", got)
	}
}

func TestTabAtScrolledPinnedRight(t *testing.T) {
	// Active tab is the last one, scrolled out of view to the right -> pinR.
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := TabAtScrolledPinned(g.PlusX-1, 0, g, 12, 11); got != 11 {
		t.Fatalf("TabAtScrolledPinned near right edge while pinned-right = %d, want 11 (active idx)", got)
	}
}

func TestDropIndexScrolled(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := DropIndexScrolled(-1, 0, g, 4); got != 0 {
		t.Fatalf("DropIndexScrolled(x<0) = %d, want 0", got)
	}
	if got := DropIndexScrolled(g.PlusX, 0, g, 4); got != 3 {
		t.Fatalf("DropIndexScrolled(x>=PlusX) = %d, want numTabs-1=3", got)
	}
	want := DropIndex(10+5, g, 4)
	got := DropIndexScrolled(10, 5, g, 4)
	if got != want {
		t.Fatalf("DropIndexScrolled(in-bounds) = %d, want %d (matches DropIndex(x+scroll,...))", got, want)
	}
}

func TestTabVisualLeftUnpinned(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	got := TabVisualLeft(2, 0, 5, g, 4)
	want := 2*g.TabWidth - 5
	if got != want {
		t.Fatalf("TabVisualLeft(unpinned) = %d, want %d", got, want)
	}
}

func TestTabVisualLeftPinnedLeft(t *testing.T) {
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	scroll := g.TabWidth * 5
	got := TabVisualLeft(0, 0, scroll, g, 12)
	if got != 0 {
		t.Fatalf("TabVisualLeft(pinned-left) = %d, want 0", got)
	}
}

func TestTabVisualLeftPinnedRight(t *testing.T) {
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	got := TabVisualLeft(11, 11, 0, g, 12)
	want := g.PlusX - g.TabWidth
	if want < 0 {
		want = 0
	}
	if got != want {
		t.Fatalf("TabVisualLeft(pinned-right) = %d, want %d", got, want)
	}
}
