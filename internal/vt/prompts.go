package vt

import "github.com/geckty/geckty/internal/vt/emu"

// maxPromptMarks caps retained OSC 133 markers so long-lived sessions don't
// grow unbounded. Older marks fall off the front (same order as history).
const maxPromptMarks = 2048

// PromptMark is one OSC 133 semantic marker anchored to an absolute line
// index in History()+Screen() (same addressing as session selection).
type PromptMark struct {
	AbsLine  int
	Type     emu.SemanticPromptType
	ExitCode *int
}

// PromptMarks returns a snapshot of retained OSC 133 markers. Call within
// RLock/RUnlock when combining with History()/Screen().
func (t *Terminal) PromptMarks() []PromptMark {
	if len(t.promptMarks) == 0 {
		return nil
	}
	out := make([]PromptMark, len(t.promptMarks))
	copy(out, t.promptMarks)
	return out
}

// syncPromptMarkOffset shifts mark AbsLines when emu prunes scrollback,
// dropping marks that scrolled off the retained history. Called with t.mu held.
func (t *Terminal) syncPromptMarkOffset() {
	off := t.HistoryOffset()
	if off > t.promptHistOffset {
		drop := off - t.promptHistOffset
		kept := t.promptMarks[:0]
		for _, m := range t.promptMarks {
			m.AbsLine -= drop
			if m.AbsLine >= 0 {
				kept = append(kept, m)
			}
		}
		t.promptMarks = kept
	}
	t.promptHistOffset = off
}

func (t *Terminal) recordPromptMark(ev emu.SemanticPromptEvent) {
	switch ev.Type {
	case emu.PromptStart, emu.CommandExecuted, emu.CommandFinished:
	default:
		return
	}
	abs := len(t.History()) + ev.Position.R
	if abs < 0 {
		abs = 0
	}
	m := PromptMark{AbsLine: abs, Type: ev.Type, ExitCode: ev.ExitCode}
	t.promptMarks = append(t.promptMarks, m)
	if len(t.promptMarks) > maxPromptMarks {
		t.promptMarks = append([]PromptMark(nil), t.promptMarks[len(t.promptMarks)-maxPromptMarks:]...)
	}
}
