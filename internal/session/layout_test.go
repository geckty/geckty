package session

import "testing"

func TestLayoutLeavesSingle(t *testing.T) {
	s := &Session{}
	root := &paneNode{Session: s}
	got := LayoutLeaves(root, 10, 20, 100, 50)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	if got[0].X != 10 || got[0].Y != 20 || got[0].W != 100 || got[0].H != 50 {
		t.Fatalf("rect = %+v", got[0])
	}
}

func TestLayoutLeavesVerticalSplit(t *testing.T) {
	a, b := &Session{}, &Session{}
	root := &paneNode{
		Dir: SplitVertical, Ratio: 0.5,
		A: &paneNode{Session: a}, B: &paneNode{Session: b},
	}
	got := LayoutLeaves(root, 0, 0, 102, 40) // 102 - 2 divider = 100 → 50/50
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Session != a || got[0].W != 50 || got[0].X != 0 {
		t.Fatalf("left = %+v", got[0])
	}
	if got[1].Session != b || got[1].X != 52 || got[1].W != 50 {
		t.Fatalf("right = %+v", got[1])
	}
}

func TestRemoveSessionCollapses(t *testing.T) {
	a, b := &Session{}, &Session{}
	root := &paneNode{
		Dir: SplitVertical, Ratio: 0.5,
		A: &paneNode{Session: a}, B: &paneNode{Session: b},
	}
	root = removeSession(root, a)
	if !root.isLeaf() || root.Session != b {
		t.Fatalf("after remove a, root = %+v, want leaf b", root)
	}
	root = removeSession(root, b)
	if root != nil {
		t.Fatal("removing last leaf should yield nil")
	}
}

func TestReplaceLeafWithSplit(t *testing.T) {
	a, b := &Session{}, &Session{}
	root := &paneNode{Session: a}
	if !replaceLeafWithSplit(root, a, b, SplitHorizontal) {
		t.Fatal("replace should succeed")
	}
	if root.isLeaf() || root.A.Session != a || root.B.Session != b {
		t.Fatalf("root after split = %+v", root)
	}
}
