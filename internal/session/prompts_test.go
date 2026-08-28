package session

import (
	"strings"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestScrollToPromptAndSelectLastOutput(t *testing.T) {
	s := newTestSession(newFakePTY(), 20, 6, nil)

	s.Term.Parse([]byte(emu.OSC133PromptStart))
	s.Term.Parse([]byte("$ cmd1\r\n"))
	s.Term.Parse([]byte(emu.OSC133CommandExec))
	s.Term.Parse([]byte("out1a\r\nout1b\r\n"))
	s.Term.Parse([]byte(emu.OSC133CommandDone))
	s.Term.Parse([]byte(emu.OSC133PromptStart))
	s.Term.Parse([]byte("$ cmd2\r\n"))
	s.Term.Parse([]byte(emu.OSC133CommandExec))
	s.Term.Parse([]byte("out2\r\n"))
	s.Term.Parse([]byte(emu.OSC133CommandDone))
	s.Term.Parse([]byte(emu.OSC133PromptStart))

	if !s.ScrollToPrompt(-1) {
		t.Fatal("ScrollToPrompt(-1) should find a previous prompt")
	}
	firstJump := s.lastPromptJump
	if firstJump < 0 {
		t.Fatal("expected lastPromptJump set")
	}
	if !s.ScrollToPrompt(-1) {
		t.Fatal("ScrollToPrompt(-1) again should find an older prompt")
	}
	if s.lastPromptJump >= firstJump {
		t.Fatalf("second jump AbsLine %d should be older than %d", s.lastPromptJump, firstJump)
	}
	if !s.ScrollToPrompt(1) {
		t.Fatal("ScrollToPrompt(1) should move forward")
	}

	if !s.SelectLastCommandOutput() {
		t.Fatal("SelectLastCommandOutput should succeed")
	}
	text, ok := s.SelectedText()
	if !ok || text == "" {
		t.Fatalf("SelectedText = %q, ok=%v", text, ok)
	}
	if !strings.Contains(text, "out2") {
		t.Fatalf("selected text %q should include last command output", text)
	}
}

func TestScrollToPromptNoMarks(t *testing.T) {
	s := newTestSession(newFakePTY(), 20, 6, nil)
	if s.ScrollToPrompt(-1) {
		t.Fatal("ScrollToPrompt with no OSC 133 marks should return false")
	}
	if s.SelectLastCommandOutput() {
		t.Fatal("SelectLastCommandOutput with no marks should return false")
	}
}
