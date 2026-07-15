package chrome

import "testing"

const (
	testMinTabWidth    = 60
	testPlusWidth      = 36
	testCloseZoneWidth = 22
)

func TestComputeGeometryNoTabs(t *testing.T) {
	g := ComputeGeometry(400, 0, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if g.TabWidth != 0 {
		t.Errorf("TabWidth = %d, want 0", g.TabWidth)
	}
	if g.PlusWidth != testPlusWidth {
		t.Errorf("PlusWidth = %d, want %d", g.PlusWidth, testPlusWidth)
	}
	if g.PlusX != 400-testPlusWidth {
		t.Errorf("PlusX = %d, want %d", g.PlusX, 400-testPlusWidth)
	}
}

func TestComputeGeometryZeroWidth(t *testing.T) {
	g := ComputeGeometry(0, 3, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if g != (Geometry{}) {
		t.Errorf("ComputeGeometry(0, ...) = %+v, want zero value", g)
	}
}

func TestComputeGeometrySharesWidthEvenly(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	want := (400 - testPlusWidth) / 4
	if g.TabWidth != want {
		t.Errorf("TabWidth = %d, want stretched %d", g.TabWidth, want)
	}
	if g.PlusX != 400-testPlusWidth {
		t.Errorf("PlusX = %d, want %d", g.PlusX, 400-testPlusWidth)
	}
	if !g.ShowClose {
		t.Errorf("ShowClose = false, want true")
	}
}

func TestComputeGeometryStretchesFewTabs(t *testing.T) {
	g := ComputeGeometry(1000, 2, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	want := (1000 - testPlusWidth) / 2
	if g.TabWidth != want {
		t.Errorf("TabWidth = %d, want %d", g.TabWidth, want)
	}
	if TabsEnd(g, 2) != 2*want {
		t.Errorf("TabsEnd = %d, want %d", TabsEnd(g, 2), 2*want)
	}
}

func TestComputeGeometryScrollsWhenCrowded(t *testing.T) {
	g := ComputeGeometry(400, 10, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if g.TabWidth != testMinTabWidth {
		t.Errorf("TabWidth = %d, want min %d", g.TabWidth, testMinTabWidth)
	}
	if ScrollMax(g, 10) <= 0 {
		t.Errorf("ScrollMax = %d, want > 0 when crowded", ScrollMax(g, 10))
	}
	if !g.ShowClose {
		t.Errorf("ShowClose = false, want true")
	}
}

func TestComputeGeometryDropsPlusWhenNoRoom(t *testing.T) {
	g := ComputeGeometry(70, 1, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if g.PlusWidth != 0 {
		t.Errorf("PlusWidth = %d, want 0", g.PlusWidth)
	}
	if g.TabWidth != 70 {
		t.Errorf("TabWidth = %d, want 70", g.TabWidth)
	}
}

func TestTabAt(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	tests := []struct {
		x    int
		want int
	}{
		{0, 0},
		{g.TabWidth - 1, 0},
		{g.TabWidth, 1},
		{2 * g.TabWidth, 2},
		{3*g.TabWidth + (g.TabWidth - 1), 3},
		{-1, -1},
		{5 * g.TabWidth, -1},
	}
	for _, tt := range tests {
		if got := TabAt(tt.x, g, 4); got != tt.want {
			t.Errorf("TabAt(%d) = %d, want %d", tt.x, got, tt.want)
		}
	}
}

func TestTabAtFillsStrip(t *testing.T) {
	g := ComputeGeometry(1000, 2, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := TabAtScrolled(g.PlusX-1, 0, g, 2); got != 1 {
		t.Errorf("TabAtScrolled near plus = %d, want last tab 1", got)
	}
	if !IsPlusHit(g.PlusX, g) {
		t.Error("expected + hit at PlusX")
	}
}

func TestTabAtNoTabs(t *testing.T) {
	g := ComputeGeometry(400, 0, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := TabAt(10, g, 0); got != -1 {
		t.Errorf("TabAt with no tabs = %d, want -1", got)
	}
}

func TestIsPlusHit(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if !IsPlusHit(g.PlusX, g) {
		t.Error("IsPlusHit(left edge) = false, want true")
	}
	if !IsPlusHit(g.PlusX+g.PlusWidth-1, g) {
		t.Error("IsPlusHit(right edge) = false, want true")
	}
	if IsPlusHit(g.PlusX+g.PlusWidth, g) {
		t.Error("IsPlusHit(past right edge) = true, want false")
	}
	if IsPlusHit(g.PlusX-1, g) {
		t.Error("IsPlusHit(just before) = true, want false")
	}
}

func TestIsPlusHitWhenDropped(t *testing.T) {
	g := ComputeGeometry(70, 1, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if IsPlusHit(70, g) {
		t.Error("IsPlusHit with dropped plus button = true, want false")
	}
}

func TestIsCloseHit(t *testing.T) {
	const edgePad = 8
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if IsCloseHit(0, g, testCloseZoneWidth, edgePad) {
		t.Error("IsCloseHit(0) = true, want false (before edge pad)")
	}
	if !IsCloseHit(edgePad, g, testCloseZoneWidth, edgePad) {
		t.Error("IsCloseHit at padded close zone = false, want true")
	}
	if !IsCloseHit(edgePad+testCloseZoneWidth-1, g, testCloseZoneWidth, edgePad) {
		t.Error("IsCloseHit at close-zone right edge = false, want true")
	}
	if IsCloseHit(edgePad+testCloseZoneWidth, g, testCloseZoneWidth, edgePad) {
		t.Error("IsCloseHit just past close zone = true, want false")
	}
	if IsCloseHit(g.TabWidth-1, g, testCloseZoneWidth, edgePad) {
		t.Error("IsCloseHit at tab's right edge = true, want false (close is left)")
	}
}

func TestIsCloseHitWhenTooNarrow(t *testing.T) {
	g := ComputeGeometry(200, 10, 1, testPlusWidth, testCloseZoneWidth)
	if IsCloseHit(8, g, testCloseZoneWidth, 8) {
		t.Error("IsCloseHit = true when ShowClose is false, want false")
	}
}

func TestShowCloseRequiresMultipleTabs(t *testing.T) {
	g := ComputeGeometry(400, 1, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if g.ShowClose {
		t.Error("ShowClose = true with a single tab, want false")
	}
}

func TestDropIndexMidpoints(t *testing.T) {
	g := ComputeGeometry(400, 4, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if got := DropIndex(0, g, 4); got != 0 {
		t.Errorf("DropIndex(0) = %d, want 0", got)
	}
	if got := DropIndex(g.TabWidth/2-1, g, 4); got != 0 {
		t.Errorf("DropIndex before first mid = %d, want 0", got)
	}
	if got := DropIndex(g.TabWidth/2+1, g, 4); got != 1 {
		t.Errorf("DropIndex past first mid = %d, want 1", got)
	}
	if got := DropIndex(1000, g, 4); got != 3 {
		t.Errorf("DropIndex past end = %d, want last index 3", got)
	}
}

func TestScrollHelpers(t *testing.T) {
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	if max := ScrollMax(g, 12); max <= 0 {
		t.Fatalf("ScrollMax = %d, want > 0", max)
	}
	if got := TabAtScrolled(0, g.TabWidth, g, 12); got != 1 {
		t.Fatalf("TabAtScrolled = %d, want 1", got)
	}
}

func TestActiveTabPin(t *testing.T) {
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	pinL, pinR := ActiveTabPin(0, 12, g.TabWidth, g.PlusX, g.TabWidth)
	if !pinL || pinR {
		t.Fatalf("ActiveTabPin(left) = %v,%v, want true,false", pinL, pinR)
	}
	max := ScrollMax(g, 12)
	pinL, pinR = ActiveTabPin(11, 12, g.TabWidth, g.PlusX, max-g.TabWidth/2)
	if pinL || !pinR {
		t.Fatalf("ActiveTabPin(right) = %v,%v, want false,true", pinL, pinR)
	}
}

func TestTabAtScrolledPinned(t *testing.T) {
	g := ComputeGeometry(400, 12, testMinTabWidth, testPlusWidth, testCloseZoneWidth)
	scroll := g.TabWidth
	if got := TabAtScrolledPinned(10, scroll, g, 12, 0); got != 0 {
		t.Fatalf("TabAtScrolledPinned(left pinned) = %d, want active 0", got)
	}
}

func TestVisualTabSlot(t *testing.T) {
	if got := VisualTabSlot(1, 0, 2); got != 0 {
		t.Errorf("VisualTabSlot(1, from=0, over=2) = %d, want 0", got)
	}
	if got := VisualTabSlot(2, 0, 2); got != 1 {
		t.Errorf("VisualTabSlot(2, from=0, over=2) = %d, want 1", got)
	}
	if got := VisualTabSlot(0, 0, 2); got != 0 {
		t.Errorf("dragged tab keeps its index for VisualTabSlot = %d, want 0", got)
	}
	if got := VisualTabSlot(0, 3, 0); got != 1 {
		t.Errorf("VisualTabSlot(0, from=3, over=0) = %d, want 1", got)
	}
}

func TestTruncateTitleKeepsEnd(t *testing.T) {
	short := "Users/foo"
	if got := truncateTitle(short); got != short {
		t.Fatalf("short = %q, want unchanged", got)
	}
	long := `C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe`
	got := truncateTitle(long)
	if r := []rune(got); len(r) == 0 || r[0] != '…' {
		t.Fatalf("got %q, want leading ellipsis", got)
	}
	if !hasSuffixRunes(got, "powershell.exe") {
		t.Fatalf("got %q, want end kept (powershell.exe)", got)
	}
	if hasPrefixASCII(got, `C:\WINDOWS`) {
		t.Fatalf("got %q, should not keep path start", got)
	}
}

func hasSuffixRunes(s, suffix string) bool {
	sr, zr := []rune(s), []rune(suffix)
	if len(sr) < len(zr) {
		return false
	}
	for i := range zr {
		if sr[len(sr)-len(zr)+i] != zr[i] {
			return false
		}
	}
	return true
}

func hasPrefixASCII(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestDropIndexByOverlap(t *testing.T) {
	const tabW, num, from = 100, 4, 1
	// 40% onto tab 2: left=160? ov = left+tabW-2*tabW = left-tabW → left=140 → ov=40.
	if got := DropIndexByOverlap(140, 0, tabW, num, from, from); got != from {
		t.Fatalf("40%% overlap: over=%d, want stay %d", got, from)
	}
	// 45% onto tab 2 → left=145.
	if got := DropIndexByOverlap(145, 0, tabW, num, from, from); got != 2 {
		t.Fatalf("45%% overlap right: over=%d, want 2", got)
	}
	// 45% onto tab 0 while dragging left: ov = tabW-left → left=55.
	if got := DropIndexByOverlap(55, 0, tabW, num, from, from); got != 0 {
		t.Fatalf("45%% overlap left: over=%d, want 0", got)
	}
}
