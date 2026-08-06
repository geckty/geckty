package session

// SplitDir is the orientation of an internal split node.
type SplitDir int

const (
	SplitVertical SplitDir = iota // left | right
	SplitHorizontal               // top / bottom
)

// paneNode is one node in a tab's layout tree: either a leaf (Session set)
// or an internal split (A/B set, Dir/Ratio valid).
type paneNode struct {
	Session *Session
	Dir     SplitDir
	Ratio   float64 // fraction of space for A (First); default 0.5
	A, B    *paneNode
}

func (n *paneNode) isLeaf() bool {
	return n != nil && n.Session != nil && n.A == nil && n.B == nil
}

func leafSession(n *paneNode) *Session {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		return n.Session
	}
	if s := leafSession(n.A); s != nil {
		return s
	}
	return leafSession(n.B)
}

func collectLeaves(n *paneNode, out []*Session) []*Session {
	if n == nil {
		return out
	}
	if n.isLeaf() {
		return append(out, n.Session)
	}
	out = collectLeaves(n.A, out)
	return collectLeaves(n.B, out)
}

func containsSession(n *paneNode, s *Session) bool {
	if n == nil || s == nil {
		return false
	}
	if n.Session == s {
		return true
	}
	return containsSession(n.A, s) || containsSession(n.B, s)
}

// removeSession drops s from the tree, collapsing a split whose other
// child remains. Returns the new root (nil if the tree is empty).
func removeSession(n *paneNode, s *Session) *paneNode {
	if n == nil {
		return nil
	}
	if n.isLeaf() {
		if n.Session == s {
			return nil
		}
		return n
	}
	n.A = removeSession(n.A, s)
	n.B = removeSession(n.B, s)
	switch {
	case n.A == nil && n.B == nil:
		return nil
	case n.A == nil:
		return n.B
	case n.B == nil:
		return n.A
	default:
		return n
	}
}

// replaceLeafWithSplit finds the leaf holding old and replaces it with a
// split whose A is old and B is neu.
func replaceLeafWithSplit(n *paneNode, old, neu *Session, dir SplitDir) bool {
	if n == nil {
		return false
	}
	if n.isLeaf() && n.Session == old {
		n.Session = nil
		n.Dir = dir
		n.Ratio = 0.5
		n.A = &paneNode{Session: old}
		n.B = &paneNode{Session: neu}
		return true
	}
	return replaceLeafWithSplit(n.A, old, neu, dir) || replaceLeafWithSplit(n.B, old, neu, dir)
}

// PaneRect is a leaf's pixel rectangle inside the content area.
type PaneRect struct {
	Session    *Session
	X, Y, W, H int
}

const splitDividerPx = 2

// LayoutLeaves walks the tree and assigns pixel rects within (x,y,w,h).
func LayoutLeaves(n *paneNode, x, y, w, h int) []PaneRect {
	if n == nil || w <= 0 || h <= 0 {
		return nil
	}
	if n.isLeaf() {
		return []PaneRect{{Session: n.Session, X: x, Y: y, W: w, H: h}}
	}
	ratio := n.Ratio
	if ratio <= 0 || ratio >= 1 {
		ratio = 0.5
	}
	div := splitDividerPx
	switch n.Dir {
	case SplitHorizontal:
		avail := h - div
		if avail < 2 {
			avail = h
			div = 0
		}
		h1 := int(float64(avail) * ratio)
		if h1 < 1 {
			h1 = 1
		}
		if h1 > avail-1 {
			h1 = avail - 1
		}
		h2 := avail - h1
		var out []PaneRect
		out = append(out, LayoutLeaves(n.A, x, y, w, h1)...)
		out = append(out, LayoutLeaves(n.B, x, y+h1+div, w, h2)...)
		return out
	default: // SplitVertical
		avail := w - div
		if avail < 2 {
			avail = w
			div = 0
		}
		w1 := int(float64(avail) * ratio)
		if w1 < 1 {
			w1 = 1
		}
		if w1 > avail-1 {
			w1 = avail - 1
		}
		w2 := avail - w1
		var out []PaneRect
		out = append(out, LayoutLeaves(n.A, x, y, w1, h)...)
		out = append(out, LayoutLeaves(n.B, x+w1+div, y, w2, h)...)
		return out
	}
}
