package session

import "sync"

// Tab pairs a Session with the stable id Manager assigned it, for UI code
// (the tab bar) that needs to render and target individual tabs.
type Tab struct {
	ID      int
	Session *Session
}

// Manager owns the set of open tabs and tracks which one is active.
type Manager struct {
	mu       sync.Mutex
	sessions []*Session
	active   int
	nextID   int
	ids      map[*Session]int
	onChange func()
}

// NewManager creates an empty Manager. onChange, if non-nil, is called
// (from whatever goroutine triggered the change) after any tab is added,
// removed, or made active — the UI layer uses it to schedule a repaint of
// the tab bar.
func NewManager(onChange func()) *Manager {
	return &Manager{
		ids:      make(map[*Session]int),
		onChange: onChange,
	}
}

// New starts a new session and adds it as a tab, making it active. It owns
// the session's whole lifecycle from here: cfg.OnExit is overridden (any
// value the caller set is ignored) to auto-remove the tab when the shell
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
	m.sessions = append(m.sessions, s)
	m.ids[s] = id
	m.nextID++
	m.active = len(m.sessions) - 1
	m.mu.Unlock()

	s.SetOnExit(func(error) { _ = m.Close(id) })
	go s.Run()

	m.notify()
	return s, nil
}

// Close closes the session with the given id and removes its tab.
func (m *Manager) Close(id int) error {
	m.mu.Lock()
	idx := -1
	for i, s := range m.sessions {
		if m.ids[s] == id {
			idx = i
			break
		}
	}
	if idx == -1 {
		m.mu.Unlock()
		return nil
	}
	s := m.sessions[idx]
	m.sessions = append(m.sessions[:idx], m.sessions[idx+1:]...)
	delete(m.ids, s)
	if m.active >= len(m.sessions) {
		m.active = len(m.sessions) - 1
	}
	m.mu.Unlock()

	m.notify()
	return s.Close()
}

// CloseActive closes the currently active tab, if any.
func (m *Manager) CloseActive() error {
	id := m.ActiveID()
	if id < 0 {
		return nil
	}
	return m.Close(id)
}

// Active returns the currently active session, or nil if there are none.
func (m *Manager) Active() *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active < 0 || m.active >= len(m.sessions) {
		return nil
	}
	return m.sessions[m.active]
}

// ActiveID returns the id of the currently active tab, or -1 if there are
// none.
func (m *Manager) ActiveID() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active < 0 || m.active >= len(m.sessions) {
		return -1
	}
	return m.ids[m.sessions[m.active]]
}

// SetActive switches the active tab to the session with the given id.
func (m *Manager) SetActive(id int) {
	m.mu.Lock()
	for i, s := range m.sessions {
		if m.ids[s] == id {
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
	n := len(m.sessions)
	if n == 0 {
		m.mu.Unlock()
		return
	}
	m.active = ((m.active+delta)%n + n) % n
	m.mu.Unlock()
	m.notify()
}

// MoveTo repositions the tab with the given id to index newIndex (clamped
// to the valid range), preserving the relative order of every other tab.
// A no-op if id doesn't exist or is already at newIndex.
//
// The active tab is tracked by identity (which *Session was active),
// not by re-deriving its new index arithmetically from the move — a
// slice removal followed by a reinsertion has enough index-shifting edge
// cases (move left vs. right, across vs. away from the active tab) that
// re-finding "where did the active session end up" after the fact is
// less error-prone than trying to get the shift arithmetic right for
// every case.
func (m *Manager) MoveTo(id, newIndex int) {
	m.mu.Lock()
	idx := -1
	for i, s := range m.sessions {
		if m.ids[s] == id {
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
	if newIndex > len(m.sessions)-1 {
		newIndex = len(m.sessions) - 1
	}
	if newIndex == idx {
		m.mu.Unlock()
		return
	}

	var activeSession *Session
	if m.active >= 0 && m.active < len(m.sessions) {
		activeSession = m.sessions[m.active]
	}

	moved := m.sessions[idx]
	newOrder := make([]*Session, 0, len(m.sessions))
	for i, s := range m.sessions {
		if i == idx {
			continue
		}
		if len(newOrder) == newIndex {
			newOrder = append(newOrder, moved)
		}
		newOrder = append(newOrder, s)
	}
	if len(newOrder) == newIndex {
		newOrder = append(newOrder, moved)
	}
	m.sessions = newOrder

	if activeSession != nil {
		for i, s := range m.sessions {
			if s == activeSession {
				m.active = i
				break
			}
		}
	}
	m.mu.Unlock()
	m.notify()
}

// Tabs returns a snapshot of the current tabs, in order, paired with their
// stable ids.
func (m *Manager) Tabs() []Tab {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Tab, len(m.sessions))
	for i, s := range m.sessions {
		out[i] = Tab{ID: m.ids[s], Session: s}
	}
	return out
}

func (m *Manager) notify() {
	if m.onChange != nil {
		m.onChange()
	}
}
