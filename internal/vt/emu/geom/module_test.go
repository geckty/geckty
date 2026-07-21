package geom

import "testing"

func TestVec2Scalar(t *testing.T) {
	v := Vec2{R: 2, C: 3}
	got := v.Scalar(4)
	want := Vec2{R: 8, C: 12}
	if got != want {
		t.Fatalf("Scalar(4) = %v, want %v", got, want)
	}
}

func TestVec2Add(t *testing.T) {
	a := Vec2{R: 1, C: 2}
	b := Vec2{R: 3, C: 4}
	got := a.Add(b)
	want := Vec2{R: 4, C: 6}
	if got != want {
		t.Fatalf("Add = %v, want %v", got, want)
	}
}

func TestVec2Sub(t *testing.T) {
	a := Vec2{R: 5, C: 7}
	b := Vec2{R: 2, C: 3}
	got := a.Sub(b)
	want := Vec2{R: 3, C: 4}
	if got != want {
		t.Fatalf("Sub = %v, want %v", got, want)
	}
}

func TestVec2Dist(t *testing.T) {
	a := Vec2{R: 0, C: 0}
	b := Vec2{R: 3, C: 4}
	if got := a.Dist(b); got != 5 {
		t.Fatalf("Dist = %d, want 5 (3-4-5 triangle)", got)
	}
	if got := a.Dist(a); got != 0 {
		t.Fatalf("Dist(self) = %d, want 0", got)
	}
}

func TestVec2IsZero(t *testing.T) {
	if !(Vec2{}.IsZero()) {
		t.Fatal("zero-value Vec2 should be IsZero")
	}
	if (Vec2{R: 1}).IsZero() {
		t.Fatal("Vec2{R:1} should not be IsZero")
	}
	if (Vec2{C: 1}).IsZero() {
		t.Fatal("Vec2{C:1} should not be IsZero")
	}
}

func TestVec2Center(t *testing.T) {
	outer := Vec2{R: 10, C: 20}
	inner := Vec2{R: 2, C: 4}
	got := outer.Center(inner)
	want := Vec2{R: 4, C: 8}
	if got != want {
		t.Fatalf("Center = %v, want %v", got, want)
	}
}

func TestVec2Clamp(t *testing.T) {
	v := Vec2{R: -5, C: 100}
	got := v.Clamp(Vec2{R: 0, C: 0}, Vec2{R: 10, C: 10})
	want := Vec2{R: 0, C: 10}
	if got != want {
		t.Fatalf("Clamp = %v, want %v", got, want)
	}
}

func TestVec2Comparisons(t *testing.T) {
	a := Vec2{R: 1, C: 5}
	b := Vec2{R: 2, C: 0}
	eq := Vec2{R: 1, C: 5}
	tieBreakGreater := Vec2{R: 1, C: 10}

	if !a.LT(b) {
		t.Error("a.LT(b) should be true: a.R < b.R")
	}
	if !b.GT(a) {
		t.Error("b.GT(a) should be true")
	}
	if a.GT(b) {
		t.Error("a.GT(b) should be false")
	}
	if !a.GTE(eq) {
		t.Error("a.GTE(equal) should be true")
	}
	if !a.LTE(eq) {
		t.Error("a.LTE(equal) should be true")
	}
	if a.GT(eq) {
		t.Error("a.GT(equal) should be false")
	}
	// Same row: tie-break on column.
	if !tieBreakGreater.GT(a) {
		t.Error("same row, larger column should be GT")
	}
	if !a.LT(tieBreakGreater) {
		t.Error("same row, smaller column should be LT")
	}
}

func TestNormalizeRange(t *testing.T) {
	start := Vec2{R: 5, C: 0}
	end := Vec2{R: 1, C: 0}
	newStart, newEnd := NormalizeRange(start, end)
	if newStart != end || newEnd != start {
		t.Fatalf("NormalizeRange should swap out-of-order points: got start=%v end=%v", newStart, newEnd)
	}

	// Already-ordered range should pass through unchanged.
	s2, e2 := NormalizeRange(end, start)
	if s2 != end || e2 != start {
		t.Fatalf("NormalizeRange should leave an already-ordered range unchanged: got start=%v end=%v", s2, e2)
	}
}

func TestRectContains(t *testing.T) {
	r := Rect{Position: Vec2{R: 2, C: 2}, Size: Vec2{R: 3, C: 3}}
	if !r.Contains(Vec2{R: 2, C: 2}) {
		t.Error("top-left corner should be contained (inclusive)")
	}
	if r.Contains(Vec2{R: 5, C: 5}) {
		t.Error("bottom-right corner (position+size) should NOT be contained (exclusive)")
	}
	if r.Contains(Vec2{R: 1, C: 2}) {
		t.Error("point above the rect should not be contained")
	}
	if r.Contains(Vec2{R: 2, C: 1}) {
		t.Error("point left of the rect should not be contained")
	}
}

func TestRectBottomRight(t *testing.T) {
	r := Rect{Position: Vec2{R: 2, C: 3}, Size: Vec2{R: 4, C: 5}}
	got := r.BottomRight()
	want := Vec2{R: 6, C: 8}
	if got != want {
		t.Fatalf("BottomRight = %v, want %v", got, want)
	}
}

func TestGetMaximum(t *testing.T) {
	a := Vec2{R: 10, C: 5}
	b := Vec2{R: 3, C: 20}
	got := GetMaximum(a, b)
	want := Vec2{R: 3, C: 5}
	if got != want {
		t.Fatalf("GetMaximum = %v, want %v (component-wise min)", got, want)
	}
}
