package app

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/geckty/geckty/internal/rc"
)

func TestRCHostListTabsMarksActive(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	h := rcHost{s: s}
	tabs, err := h.ListTabs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 1 || !strings.HasSuffix(tabs[0], "*") {
		t.Fatalf("ListTabs = %v, want one active marked with *", tabs)
	}
	s.dispatchAction(ActionNewTab)
	tabs, err = h.ListTabs()
	if err != nil {
		t.Fatal(err)
	}
	if len(tabs) != 2 {
		t.Fatalf("ListTabs after NewTab = %v", tabs)
	}
	star := 0
	for _, tab := range tabs {
		if strings.HasSuffix(tab, "*") {
			star++
		}
	}
	if star != 1 {
		t.Fatalf("expected exactly one active mark, got %v", tabs)
	}
}

func TestRCHostNewTabCloseTab(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	h := rcHost{s: s}
	before := len(s.mgr.Tabs())
	if err := h.NewTab(); err != nil {
		t.Fatal(err)
	}
	if len(s.mgr.Tabs()) != before+1 {
		t.Fatalf("tabs = %d, want %d", len(s.mgr.Tabs()), before+1)
	}
	if err := h.CloseTab(); err != nil {
		t.Fatal(err)
	}
	if len(s.mgr.Tabs()) != before {
		t.Fatalf("after CloseTab tabs = %d, want %d", len(s.mgr.Tabs()), before)
	}
}

func TestRCHostNewTabWithoutFactory(t *testing.T) {
	s, _ := testUIState(t)
	h := rcHost{s: s}
	if err := h.NewTab(); err == nil {
		t.Fatal("expected error without newTab factory")
	}
}

func TestRCHostSendTextAndGetText(t *testing.T) {
	s, _ := testUIStateWithTab(t)
	h := rcHost{s: s}
	active := s.mgr.Active()
	active.Term.Parse([]byte("hello from scrollback\r\n"))

	text, err := h.GetText()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "hello from scrollback") {
		t.Fatalf("GetText = %q", text)
	}

	active.StartSelection(0, 0)
	active.ExtendSelection(4, 0)
	active.EndSelection()
	text, err = h.GetText()
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Fatal("GetText should prefer non-empty selection")
	}

	if err := h.SendText("typed"); err != nil {
		t.Fatal(err)
	}
}

func TestRCHostGetTextNoSession(t *testing.T) {
	s, _ := testUIState(t)
	h := rcHost{s: s}
	if _, err := h.GetText(); err == nil {
		t.Fatal("expected error with no active session")
	}
	if err := h.SendText("x"); err == nil {
		t.Fatal("expected SendText error with no active session")
	}
}

func TestWireRemoteControlNoopWithoutEnv(t *testing.T) {
	s, _ := testUIState(t)
	t.Setenv("GECKTY_SOCKET", "")
	t.Setenv("GECKTY_LISTEN", "")
	stop := s.wireRemoteControl()
	stop()
}

func TestWireRemoteControlStartsListener(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix socket path used in this test")
	}
	s, _ := testUIStateWithTab(t)
	path := filepath.Join(os.TempDir(), fmt.Sprintf("gckty-ui-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(path) })
	t.Setenv("GECKTY_SOCKET", path)
	t.Setenv("GECKTY_LISTEN", "")
	stop := s.wireRemoteControl()
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	var resp string
	var err error
	for {
		resp, err = rc.DialAndSend(path, "list_tabs")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("DialAndSend: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !strings.HasPrefix(resp, "OK ") {
		t.Fatalf("list_tabs via wireRemoteControl = %q", resp)
	}
}
