package session

import "sync"

// Tab pairs a tab id with its focused Session (for UI that needs one
// session per tab pill — title, OSC 133 indicator). The tab may contain
// additional panes; Active()/focus tracks which leaf receives input.
type Tab struct {
	ID      int
	Session *Session
}

type tabEntry struct {
	id    int
	root  *paneNode
	focus *Session
}

// Manager owns the set of open tabs and tracks which one is active.
type Manager struct {
	mu     sync.Mutex
	tabs   []*tabEntry
	active int
	nextID int
	// sessTab maps every live pane Session to its owning tab id so shell
	// exit can close just that pane.
	sessTab  map[*Session]int
	onChange func()
	// spawn is set by the UI to create sibling sessions for Split.
	// When nil, Split returns false (tests without a spawn hook).
	spawn func(cols, rows int) (*Session, error)
}

// NewManager creates an empty Manager. onChange, if non-nil, is called
// (from whatever goroutine triggered the change) after any tab is added,
// removed, or made active — the UI layer uses it to schedule a repaint of
// the tab bar.
func NewManager(onChange func()) *Manager {
	return &Manager{
		sessTab:  make(map[*Session]int),
		onChange: onChange,
	}
}

// SetSpawn registers the factory used by Split to create the new pane's
// session. cols/rows are the initial PTY size for that pane.
func (m *Manager) SetSpawn(spawn func(cols, rows int) (*Session, error)) {
	m.mu.Lock()
	m.spawn = spawn
	m.mu.Unlock()
}

// New starts a new session and adds it as a tab, making it active. It owns
// the session's whole lifecycle from here: cfg.OnExit is overridden (any
// value the caller set is ignored) to auto-remove the pane when the shell
// exits, and Run is started in its own goroutine — callers don't need to
// call Run themselves.
func (m *Manager) New(cfg Config) (*Session, error) {
	cfg.OnExit = nil
	s, err := New(cfg)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	id := m.nextID
	m.nextID++
	tab := &tabEntry{
		id:    id,
		root:  &paneNode{Session: s},
		focus: s,
	}
	m.tabs = append(m.tabs, tab)
	m.sessTab[s] = id
	m.active = len(m.tabs) - 1
	m.mu.Unlock()

	s.SetOnExit(func(error) { _ = m.CloseSession(s) })
	go s.Run()

	m.notify()
	return s, nil
}

// Split splits the focused pane of the active tab in dir, spawning a new
// session in the new half. Returns false if there is no active tab or
// spawn is unset / fails.
func (m *Manager) Split(dir SplitDir, cols, rows int) bool {
	m.mu.Lock()
	spawn := m.spawn
	tab := m.activeTabLocked()
	var focus *Session
	if tab != nil {
		focus = tab.focus
	}
	m.mu.Unlock()
	if tab == nil || focus == nil || spawn == nil {
		return false
	}
	neu, err := spawn(cols, rows)
	if err != nil || neu == nil {
		return false
	}

	m.mu.Lock()
	tab = m.activeTabLocked()
	if tab == nil || tab.focus != focus {
		m.mu.Unlock()
		_ = neu.Close()
		return false
	}
	if !replaceLeafWithSplit(tab.root, focus, neu, dir) {
		m.mu.Unlock()
		_ = neu.Close()
		return false
	}
	m.sessTab[neu] = tab.id
	tab.focus = neu
	m.mu.Unlock()

	neu.SetOnExit(func(error) { _ = m.CloseSession(neu) })
	go neu.Run()
	m.notify()
	return true
}

// CloseSession removes the pane for s. If that was the tab's last pane,
// the tab is removed (same as Close).
func (m *Manager) CloseSession(s *Session) error {
	m.mu.Lock()
	tabID, ok := m.sessTab[s]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	idx := -1
	for i, tab := range m.tabs {
		if tab.id == tabID {
			idx = i
			break
		}
	}
	if idx < 0 {
		m.mu.Unlock()
		return nil
	}
	tab := m.tabs[idx]
	tab.root = removeSession(tab.root, s)
	delete(m.sessTab, s)
	if tab.root == nil {
		m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
		if m.active >= len(m.tabs) {
			m.active = len(m.tabs) - 1
		}
		m.mu.Unlock()
		m.notify()
		return s.Close()
	}
	if tab.focus == s || tab.focus == nil || !containsSession(tab.root, tab.focus) {
		tab.focus = leafSession(tab.root)
	}
	m.mu.Unlock()
	m.notify()
	return s.Close()
}

// Close closes the tab with the given id and every pane it contains.
func (m *Manager) Close(id int) error {
	m.mu.Lock()
	idx := -1
	for i, tab := range m.tabs {
		if tab.id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.mu.Unlock()
		return nil
	}
	tab := m.tabs[idx]
	sessions := collectLeaves(tab.root, nil)
	for _, s := range sessions {
		delete(m.sessTab, s)
	}
	m.tabs = append(m.tabs[:idx], m.tabs[idx+1:]...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
	m.mu.Unlock()

	m.notify()
	var firstErr error
	for _, s := range sessions {
		if err := s.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// CloseActive closes the focused pane of the active tab (or the whole tab
// when it has only one pane).
func (m *Manager) CloseActive() error {
	m.mu.Lock()
	tab := m.activeTabLocked()
	if tab == nil {
		m.mu.Unlock()
		return nil
	}
	focus := tab.focus
	only := tab.root != nil && tab.root.isLeaf()
	id := tab.id
	m.mu.Unlock()
	if only {
		return m.Close(id)
	}
	return m.CloseSession(focus)
}

// Active returns the focused session of the active tab, or nil.
func (m *Manager) Active() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	tab := m.activeTabLocked()
	if tab == nil {
		return nil
	}
	return tab.focus
}

// ActiveID returns the id of the currently active tab, or -1.
func (m *Manager) ActiveID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	tab := m.activeTabLocked()
	if tab == nil {
		return -1
	}
	return tab.id
}

// ActiveLayout returns pixel layout leaves for the active tab within the
// given content rectangle. ok is false when there is no active tab.
func (m *Manager) ActiveLayout(x, y, w, h int) (leaves []PaneRect, focus *Session, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tab := m.activeTabLocked()
	if tab == nil {
		return nil, nil, false
	}
	return LayoutLeaves(tab.root, x, y, w, h), tab.focus, true
}

// AllSessions returns every pane Session across all tabs (for resize/bell).
func (m *Manager) AllSessions() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*Session
	for _, tab := range m.tabs {
		out = collectLeaves(tab.root, out)
	}
	return out
}

// SetFocus focuses s if it belongs to the active tab.
func (m *Manager) SetFocus(s *Session) {
	m.mu.Lock()
	tab := m.activeTabLocked()
	if tab != nil && containsSession(tab.root, s) {
		tab.focus = s
	}
	m.mu.Unlock()
	m.notify()
}

// NextPane cycles focus among panes of the active tab.
func (m *Manager) NextPane() {
	m.shiftPane(1)
}

// PrevPane cycles focus backward among panes of the active tab.
func (m *Manager) PrevPane() {
	m.shiftPane(-1)
}

func (m *Manager) shiftPane(delta int) {
	m.mu.Lock()
	tab := m.activeTabLocked()
	if tab == nil {
		m.mu.Unlock()
		return
	}
	leaves := collectLeaves(tab.root, nil)
	n := len(leaves)
	if n == 0 {
		m.mu.Unlock()
		return
	}
	idx := 0
	for i, s := range leaves {
		if s == tab.focus {
			idx = i
			break
		}
	}
	tab.focus = leaves[((idx+delta)%n+n)%n]
	m.mu.Unlock()
	m.notify()
}

// SetActive switches the active tab to the given id.
func (m *Manager) SetActive(id int) {
	m.mu.Lock()
	for i, tab := range m.tabs {
		if tab.id == id {
			m.active = i
			break
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Next switches to the tab after the active one, wrapping around.
func (m *Manager) Next() {
	m.shift(1)
}

// Prev switches to the tab before the active one, wrapping around.
func (m *Manager) Prev() {
	m.shift(-1)
}

func (m *Manager) shift(delta int) {
	m.mu.Lock()
	n := len(m.tabs)
	if n == 0 {
		m.mu.Unlock()
		return
	}
	m.active = ((m.active+delta)%n + n) % n
	m.mu.Unlock()
	m.notify()
}

// MoveTo repositions the tab with the given id to index newIndex.
func (m *Manager) MoveTo(id, newIndex int) {
	m.mu.Lock()
	idx := -1
	for i, tab := range m.tabs {
		if tab.id == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.mu.Unlock()
		return
	}
	if newIndex < 0 {
		newIndex = 0
	}
	if newIndex > len(m.tabs)-1 {
		newIndex = len(m.tabs) - 1
	}
	if newIndex == idx {
		m.mu.Unlock()
		return
	}

	var activeTab *tabEntry
	if m.active >= 0 && m.active < len(m.tabs) {
		activeTab = m.tabs[m.active]
	}

	moved := m.tabs[idx]
	newOrder := make([]*tabEntry, 0, len(m.tabs))
	for i, tab := range m.tabs {
		if i == idx {
			continue
		}
		if len(newOrder) == newIndex {
			newOrder = append(newOrder, moved)
		}
		newOrder = append(newOrder, tab)
	}
	if len(newOrder) == newIndex {
		newOrder = append(newOrder, moved)
	}
	m.tabs = newOrder

	if activeTab != nil {
		for i, tab := range m.tabs {
			if tab == activeTab {
				m.active = i
				break
			}
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Tabs returns a snapshot of the current tabs (focused session per tab).
func (m *Manager) Tabs() []Tab {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Tab, len(m.tabs))
	for i, tab := range m.tabs {
		out[i] = Tab{ID: tab.id, Session: tab.focus}
	}
	return out
}

// EachTabLayout invokes fn with the leaf pixel rects for every tab inside
// the shared content rectangle (all tabs share the window's content area).
func (m *Manager) EachTabLayout(x, y, w, h int, fn func([]PaneRect)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, tab := range m.tabs {
		fn(LayoutLeaves(tab.root, x, y, w, h))
	}
}

func (m *Manager) activeTabLocked() *tabEntry {
	if m.active < 0 || m.active >= len(m.tabs) {
		return nil
	}
	return m.tabs[m.active]
}

func (m *Manager) notify() {
	if m.onChange != nil {
		m.onChange()
	}
}
