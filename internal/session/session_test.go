package session

import (
	"encoding/base64"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/geckty/geckty/internal/vt/emu"
)

// fakePTY is an io.Pipe-backed stand-in for pty.PTY, letting session tests
// run without spawning a real shell.
type fakePTY struct {
	toSession    *io.PipeReader // Session reads shell "output" from here
	toSessionW   *io.PipeWriter // test writes simulated shell output here
	fromSession  *io.PipeReader // test reads what Session wrote (keystrokes)
	fromSessionW *io.PipeWriter

	mu     sync.Mutex
	cols   uint16
	rows   uint16
	closed bool
}

func newFakePTY() *fakePTY {
	toR, toW := io.Pipe()
	fromR, fromW := io.Pipe()
	return &fakePTY{toSession: toR, toSessionW: toW, fromSession: fromR, fromSessionW: fromW}
}

func (f *fakePTY) Read(b []byte) (int, error)  { return f.toSession.Read(b) }
func (f *fakePTY) Write(b []byte) (int, error) { return f.fromSessionW.Write(b) }
func (f *fakePTY) Resize(cols, rows uint16) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cols, f.rows = cols, rows
	return nil
}
func (f *fakePTY) Pid() int    { return 1 }
func (f *fakePTY) Wait() error { return nil }
func (f *fakePTY) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return nil
	}
	f.closed = true
	_ = f.toSessionW.Close()
	_ = f.fromSessionW.Close()
	return nil
}

func TestSessionParsesShellOutput(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })

	go s.Run()

	if _, err := p.toSessionW.Write([]byte("hi")); err != nil {
		t.Fatalf("write shell output: %v", err)
	}

	select {
	case <-dirty:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDirty")
	}

	if ch := s.Term.Cell(0, 0).Char; ch != 'h' {
		t.Fatalf("cell(0,0) = %q, want 'h'", ch)
	}
	if ch := s.Term.Cell(1, 0).Char; ch != 'i' {
		t.Fatalf("cell(1,0) = %q, want 'i'", ch)
	}

	_ = s.Close()
}

func TestSessionWriteIsGuarded(t *testing.T) {
	p := newFakePTY()
	s := newTestSession(p, 10, 2, nil)

	const n = 50
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Write([]byte("x"))
		}()
	}

	got := make([]byte, n)
	go func() {
		_, _ = io.ReadFull(p.fromSession, got)
	}()
	wg.Wait()

	time.Sleep(50 * time.Millisecond)
	for _, b := range got {
		if b != 0 && b != 'x' {
			t.Fatalf("interleaved write corruption: %q", got)
		}
	}

	_ = s.Close()
}

func TestManagerLifecycle(t *testing.T) {
	changes := 0
	m := NewManager(func() { changes++ })

	// Manager.New spawns via the real pty.Open (not the fake), so this
	// test only exercises tab bookkeeping via direct session injection.
	s1 := newTestSession(newFakePTY(), 10, 2, nil)
	s2 := newTestSession(newFakePTY(), 10, 2, nil)

	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	injectTabLocked(m, s2, 1)
	m.nextID = 2
	m.active = 1
	m.mu.Unlock()

	if m.Active() != s2 {
		t.Fatal("expected s2 active")
	}

	m.SetActive(0)
	if m.Active() != s1 {
		t.Fatal("expected s1 active after SetActive(0)")
	}

	if err := m.Close(0); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if len(m.Tabs()) != 1 {
		t.Fatalf("expected 1 session left, got %d", len(m.Tabs()))
	}
	if changes == 0 {
		t.Fatal("expected onChange to have fired")
	}
}

func TestManagerTabsCarryIDs(t *testing.T) {
	m := NewManager(nil)
	s1 := newTestSession(newFakePTY(), 10, 2, nil)
	s2 := newTestSession(newFakePTY(), 10, 2, nil)

	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	injectTabLocked(m, s2, 1)
	m.nextID = 2
	m.active = 0
	m.mu.Unlock()

	tabs := m.Tabs()
	if len(tabs) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(tabs))
	}
	if tabs[0].ID != 0 || tabs[0].Session != s1 {
		t.Fatalf("tabs[0] = %+v, want {ID:0 Session:s1}", tabs[0])
	}
	if tabs[1].ID != 1 || tabs[1].Session != s2 {
		t.Fatalf("tabs[1] = %+v, want {ID:1 Session:s2}", tabs[1])
	}
	if got := m.ActiveID(); got != 0 {
		t.Fatalf("ActiveID() = %d, want 0", got)
	}
}

func TestManagerNextPrevWraps(t *testing.T) {
	m := NewManager(nil)
	s1 := newTestSession(newFakePTY(), 10, 2, nil)
	s2 := newTestSession(newFakePTY(), 10, 2, nil)
	s3 := newTestSession(newFakePTY(), 10, 2, nil)

	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	injectTabLocked(m, s2, 1)
	injectTabLocked(m, s3, 2)
	m.nextID = 3
	m.active = 0
	m.mu.Unlock()

	m.Next()
	if m.Active() != s2 {
		t.Fatal("Next() from tab 0 expected s2")
	}
	m.Next()
	if m.Active() != s3 {
		t.Fatal("Next() from tab 1 expected s3")
	}
	m.Next() // wraps back to first
	if m.Active() != s1 {
		t.Fatal("Next() from last tab expected wrap to s1")
	}

	m.Prev() // wraps to last
	if m.Active() != s3 {
		t.Fatal("Prev() from first tab expected wrap to s3")
	}
}

// injectTabLocked appends a single-pane tab. Caller must hold m.mu.
func injectTabLocked(m *Manager, s *Session, id int) {
	m.tabs = append(m.tabs, &tabEntry{
		id:    id,
		root:  &paneNode{Session: s},
		focus: s,
	})
	m.sessTab[s] = id
}

func newThreeTabManager(t *testing.T) (m *Manager, s1, s2, s3 *Session) {
	t.Helper()
	m = NewManager(nil)
	s1 = newTestSession(newFakePTY(), 10, 2, nil)
	s2 = newTestSession(newFakePTY(), 10, 2, nil)
	s3 = newTestSession(newFakePTY(), 10, 2, nil)

	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	injectTabLocked(m, s2, 1)
	injectTabLocked(m, s3, 2)
	m.nextID = 3
	m.active = 0
	m.mu.Unlock()
	return m, s1, s2, s3
}

func tabOrder(m *Manager) []int {
	tabs := m.Tabs()
	order := make([]int, len(tabs))
	for i, t := range tabs {
		order[i] = t.ID
	}
	return order
}

func TestMoveToReordersTabs(t *testing.T) {
	m, _, _, _ := newThreeTabManager(t)

	m.MoveTo(0, 2) // move tab 0 (s1) to the end
	if got, want := tabOrder(m), []int{1, 2, 0}; !slicesEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestMoveToMiddle(t *testing.T) {
	m, _, _, _ := newThreeTabManager(t)

	m.MoveTo(2, 0) // move tab 2 (s3) to the front
	if got, want := tabOrder(m), []int{2, 0, 1}; !slicesEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestMoveToPreservesActiveTabByIdentity(t *testing.T) {
	m, _, s2, _ := newThreeTabManager(t)
	m.SetActive(1) // s2 active

	// Move a different tab (id 0, s1) around s2 — s2 must remain active
	// (tracked by identity, not by its old index) regardless of how the
	// move shifts indices underneath it.
	m.MoveTo(0, 2)
	if m.Active() != s2 {
		t.Fatalf("expected s2 to remain active after moving an unrelated tab, got %v", m.Active())
	}

	m.MoveTo(0, 0) // id 0 (now at index 2 after the previous move) back near the front
	if m.Active() != s2 {
		t.Fatalf("expected s2 to remain active, got %v", m.Active())
	}
}

func TestMoveToTheActiveTabItselfStaysActive(t *testing.T) {
	m, s1, _, _ := newThreeTabManager(t)
	m.SetActive(0) // s1 active

	m.MoveTo(0, 2) // move the active tab itself
	if m.Active() != s1 {
		t.Fatalf("expected the moved tab to remain active, got %v", m.Active())
	}
	if got := m.ActiveID(); got != 0 {
		t.Fatalf("ActiveID() = %d, want 0 (ids are stable across reorder)", got)
	}
}

func TestMoveToUnknownIDIsNoOp(t *testing.T) {
	m, _, _, _ := newThreeTabManager(t)
	before := tabOrder(m)

	m.MoveTo(999, 1)

	if got := tabOrder(m); !slicesEqual(got, before) {
		t.Fatalf("order changed for an unknown id: got %v, want %v", got, before)
	}
}

func TestMoveToSameIndexIsNoOp(t *testing.T) {
	m, _, _, _ := newThreeTabManager(t)
	before := tabOrder(m)

	m.MoveTo(1, 1)

	if got := tabOrder(m); !slicesEqual(got, before) {
		t.Fatalf("order changed for a same-index move: got %v, want %v", got, before)
	}
}

func TestMoveToClampsOutOfRangeIndex(t *testing.T) {
	m, _, _, _ := newThreeTabManager(t)

	m.MoveTo(0, 100) // way past the end
	if got, want := tabOrder(m), []int{1, 2, 0}; !slicesEqual(got, want) {
		t.Fatalf("order = %v, want %v (clamped to the last valid index)", got, want)
	}

	m2, _, _, _ := newThreeTabManager(t)
	m2.MoveTo(2, -50) // way before the start
	if got, want := tabOrder(m2), []int{2, 0, 1}; !slicesEqual(got, want) {
		t.Fatalf("order = %v, want %v (clamped to 0)", got, want)
	}
}

func slicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestManagerNextPrevNoTabsIsNoOp(t *testing.T) {
	m := NewManager(nil)
	m.Next() // must not panic on an empty manager
	m.Prev()
	if m.Active() != nil {
		t.Fatal("expected nil Active() with no tabs")
	}
	if got := m.ActiveID(); got != -1 {
		t.Fatalf("ActiveID() with no tabs = %d, want -1", got)
	}
}

func TestManagerCloseActive(t *testing.T) {
	m := NewManager(nil)
	s1 := newTestSession(newFakePTY(), 10, 2, nil)
	s2 := newTestSession(newFakePTY(), 10, 2, nil)

	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	injectTabLocked(m, s2, 1)
	m.nextID = 2
	m.active = 1
	m.mu.Unlock()

	if err := m.CloseActive(); err != nil {
		t.Fatalf("CloseActive: %v", err)
	}
	if len(m.Tabs()) != 1 {
		t.Fatalf("expected 1 tab left, got %d", len(m.Tabs()))
	}
	if m.Active() != s1 {
		t.Fatal("expected s1 to remain")
	}
}

func TestManagerSplitAndClosePane(t *testing.T) {
	m := NewManager(nil)
	s1 := newTestSession(newFakePTY(), 10, 2, nil)
	m.mu.Lock()
	injectTabLocked(m, s1, 0)
	m.nextID = 1
	m.active = 0
	m.mu.Unlock()

	s2 := newTestSession(newFakePTY(), 10, 2, nil)
	m.SetSpawn(func(cols, rows int) (*Session, error) {
		return s2, nil
	})

	if !m.Split(SplitVertical, 5, 2) {
		t.Fatal("Split should succeed")
	}
	if got := len(m.AllSessions()); got != 2 {
		t.Fatalf("AllSessions = %d, want 2", got)
	}
	if m.Active() != s2 {
		t.Fatal("Split should focus the new pane")
	}
	leaves, focus, ok := m.ActiveLayout(0, 0, 100, 40)
	if !ok || len(leaves) != 2 || focus != s2 {
		t.Fatalf("ActiveLayout leaves=%d focus=%v ok=%v", len(leaves), focus, ok)
	}

	m.NextPane()
	if m.Active() != s1 {
		t.Fatal("NextPane should cycle to s1")
	}
	m.PrevPane()
	if m.Active() != s2 {
		t.Fatal("PrevPane should cycle back to s2")
	}

	if err := m.CloseSession(s2); err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if got := len(m.AllSessions()); got != 1 {
		t.Fatalf("after CloseSession AllSessions = %d, want 1", got)
	}
	if m.Active() != s1 {
		t.Fatal("expected focus to fall back to s1")
	}
	if len(m.Tabs()) != 1 {
		t.Fatal("tab should remain after closing one pane")
	}
}

func TestSessionTracksCursorStyle(t *testing.T) {
	// cy/emu tracks DECSCUSR natively via Cursor().Style — this
	// supersedes geckty's earlier hand-rolled cursorstyle sniffer
	// (used when the project was on ActiveState/vt10x, which had no
	// DECSCUSR support at all).
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })

	go s.Run()

	if _, err := p.toSessionW.Write([]byte("\x1b[6 q")); err != nil {
		t.Fatalf("write shell output: %v", err)
	}

	select {
	case <-dirty:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDirty")
	}

	if got := s.Term.Cursor().Style; got != emu.CursorStyleBar {
		t.Fatalf("Cursor().Style = %v, want CursorStyleBar", got)
	}

	_ = s.Close()
}

func writeAndWaitDirty(t *testing.T, p *fakePTY, dirty chan struct{}, data string) {
	t.Helper()
	if _, err := p.toSessionW.Write([]byte(data)); err != nil {
		t.Fatalf("write shell output: %v", err)
	}
	select {
	case <-dirty:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnDirty")
	}
}

func TestScrollByClampsToHistoryLength(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	// 2 rows visible; write 5 lines so some scroll into history. The
	// exact count depends on emu's scroll-on-newline timing, not
	// something this test should hardcode — it asserts ScrollBy clamps
	// to whatever History() actually reports, not a derived number.
	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line\r\n")
	}

	s.Term.RLock()
	wantMax := len(s.Term.History())
	s.Term.RUnlock()
	if wantMax == 0 {
		t.Fatal("expected some history to have accumulated")
	}

	if got := s.ScrollBy(100); got != wantMax {
		t.Fatalf("ScrollBy(100) = %d, want clamped to history length %d", got, wantMax)
	}
	if got := s.ScrollOffset(); got != wantMax {
		t.Fatalf("ScrollOffset() = %d, want %d", got, wantMax)
	}

	if got := s.ScrollBy(-100); got != 0 {
		t.Fatalf("ScrollBy(-100) = %d, want clamped to 0", got)
	}
}

func TestResetScroll(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line\r\n")
	}
	s.ScrollBy(2)
	if s.ScrollOffset() == 0 {
		t.Fatal("expected non-zero scroll offset before reset")
	}
	s.ResetScroll()
	if got := s.ScrollOffset(); got != 0 {
		t.Fatalf("ScrollOffset() after reset = %d, want 0", got)
	}
}

func TestAltScreenResetsScroll(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 32)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	for i := 0; i < 5; i++ {
		writeAndWaitDirty(t, p, dirty, "line\r\n")
	}
	s.ScrollBy(2)
	if s.ScrollOffset() == 0 {
		t.Fatal("expected non-zero scroll offset before entering alt screen")
	}

	writeAndWaitDirty(t, p, dirty, "\x1b[?1049h")

	if got := s.ScrollOffset(); got != 0 {
		t.Fatalf("ScrollOffset() after entering alt screen = %d, want 0 (full-screen apps shouldn't inherit a scrolled view)", got)
	}
}

func TestManagerNewAutoStartsRunAndAutoRemovesOnExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX shell command")
	}

	changes := make(chan struct{}, 10)
	m := NewManager(func() { changes <- struct{}{} })

	// Manager.New must start Run() itself (no `go s.Run()` here) and
	// wire OnExit to auto-remove the tab when the shell exits — both
	// exercised for real, not mocked.
	if _, err := m.New(Config{Command: []string{"/bin/sh", "-c", "exit 0"}, Cols: 80, Rows: 24}); err != nil {
		t.Fatalf("Manager.New: %v", err)
	}

	deadline := time.After(5 * time.Second)
	for {
		select {
		case <-changes:
			if len(m.Tabs()) == 0 {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for the tab to auto-remove after the shell exited")
		}
	}
}

func TestOSC52WriteIsAvailableToTake(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	payload := base64.StdEncoding.EncodeToString([]byte("copied text"))
	writeAndWaitDirty(t, p, dirty, "\x1b]52;c;"+payload+"\x07")

	data, _, ok := s.TakeClipboardWrite()
	if !ok {
		t.Fatal("expected a pending clipboard write")
	}
	if string(data) != "copied text" {
		t.Fatalf("TakeClipboardWrite = %q, want %q", data, "copied text")
	}
}

func TestOSC52TakeClipboardWriteClearsPending(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	payload := base64.StdEncoding.EncodeToString([]byte("once"))
	writeAndWaitDirty(t, p, dirty, "\x1b]52;c;"+payload+"\x07")

	if _, _, ok := s.TakeClipboardWrite(); !ok {
		t.Fatal("expected a pending clipboard write the first time")
	}
	if _, _, ok := s.TakeClipboardWrite(); ok {
		t.Fatal("expected no pending clipboard write after it was already taken")
	}
}

func TestOSC52NoWriteMeansNothingToTake(t *testing.T) {
	s := newTestSession(newFakePTY(), 10, 2, nil)
	if _, _, ok := s.TakeClipboardWrite(); ok {
		t.Fatal("expected no pending clipboard write by default")
	}
}

func TestOSC52EmptyPayloadClearsClipboard(t *testing.T) {
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "\x1b]52;c;\x07")
	data, shouldClear, ok := s.TakeClipboardWrite()
	if !ok || !shouldClear || len(data) != 0 {
		t.Fatalf("TakeClipboardWrite = (%q, shouldClear=%v, ok=%v), want clear", data, shouldClear, ok)
	}
}

func TestOSC52QueryGetsNoResponse(t *testing.T) {
	// Query is disabled by default (see osc52Bridge) — the shell should
	// see silence, and nothing should reach TakeClipboardWrite either.
	p := newFakePTY()
	dirty := make(chan struct{}, 8)
	s := newTestSession(p, 10, 2, func() { dirty <- struct{}{} })
	go s.Run()
	defer func() { _ = s.Close() }()

	writeAndWaitDirty(t, p, dirty, "\x1b]52;c;?\x07")

	select {
	case b := <-readAvailable(p.fromSession):
		t.Fatalf("expected no OSC52 query response, got %q", b)
	case <-time.After(100 * time.Millisecond):
	}

	if _, _, ok := s.TakeClipboardWrite(); ok {
		t.Fatal("a query must not populate a pending write")
	}
}

// readAvailable returns a channel that receives whatever bytes are
// available to read from r within a short window, without blocking the
// caller if nothing arrives.
func readAvailable(r io.Reader) <-chan []byte {
	ch := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 256)
		n, _ := r.Read(buf)
		if n > 0 {
			ch <- buf[:n]
		}
	}()
	return ch
}
