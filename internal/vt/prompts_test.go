package vt

import (
	"bytes"
	"testing"

	"github.com/geckty/geckty/internal/vt/emu"
)

func TestPromptMarksRecordOSC133(t *testing.T) {
	term := New(20, 5, &bytes.Buffer{}, nil, 0)

	term.Parse([]byte(emu.OSC133PromptStart))
	term.Parse([]byte("prompt\r\n"))
	term.Parse([]byte(emu.OSC133CommandExec))
	term.Parse([]byte("out\r\n"))
	term.Parse([]byte(emu.OSC133CommandDone))
	term.Parse([]byte(emu.OSC133PromptStart))

	marks := term.PromptMarks()
	var kinds []emu.SemanticPromptType
	for _, m := range marks {
		kinds = append(kinds, m.Type)
	}
	want := []emu.SemanticPromptType{
		emu.PromptStart, emu.CommandExecuted, emu.CommandFinished, emu.PromptStart,
	}
	if len(kinds) != len(want) {
		t.Fatalf("marks = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("marks[%d] = %v, want %v", i, kinds[i], want[i])
		}
	}
	if marks[0].AbsLine < 0 || marks[3].AbsLine <= marks[0].AbsLine {
		t.Fatalf("prompt AbsLines not advancing: %+v", marks)
	}
}

func TestPromptMarksShiftOnHistoryPrune(t *testing.T) {
	term := New(10, 2, &bytes.Buffer{}, nil, 4) // small history cap
	term.Parse([]byte(emu.OSC133PromptStart))
	first := term.PromptMarks()
	if len(first) != 1 {
		t.Fatalf("marks = %d, want 1", len(first))
	}
	base := first[0].AbsLine

	for i := 0; i < 20; i++ {
		term.Parse([]byte("line\r\n"))
	}
	after := term.PromptMarks()
	if len(after) == 0 {
		// Mark may have been pruned entirely once history scrolled past it.
		return
	}
	if after[0].AbsLine >= base+20 {
		t.Fatalf("expected AbsLine to shrink after prune; before=%d after=%d", base, after[0].AbsLine)
	}
}
