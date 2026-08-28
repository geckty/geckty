package session

import "github.com/geckty/geckty/internal/vt/emu"

// ScrollToPrompt jumps to the previous (dir < 0) or next (dir > 0) OSC 133
// prompt-start mark relative to the last jump (or the current viewport top).
// Returns false when no matching mark exists.
func (s *Session) ScrollToPrompt(dir int) bool {
	if dir == 0 {
		dir = -1
	}
	s.Term.RLock()
	marks := s.Term.PromptMarks()
	s.Term.RUnlock()

	var prompts []int
	for _, m := range marks {
		if m.Type == emu.PromptStart {
			prompts = append(prompts, m.AbsLine)
		}
	}
	if len(prompts) == 0 {
		return false
	}

	ref := s.lastPromptJump
	if ref < 0 {
		// No prior jump: treat "here" as past the end of the buffer so the
		// most recent prompt counts as previous (Kitty-style from live view).
		s.Term.RLock()
		ref = len(s.Term.History()) + len(s.Term.Screen())
		s.Term.RUnlock()
	}

	target := -1
	if dir < 0 {
		for i := len(prompts) - 1; i >= 0; i-- {
			if prompts[i] < ref {
				target = prompts[i]
				break
			}
		}
	} else {
		for _, abs := range prompts {
			if abs > ref {
				target = abs
				break
			}
		}
	}
	if target < 0 {
		return false
	}
	s.lastPromptJump = target
	s.ScrollToAbsLine(target)
	return true
}

// SelectLastCommandOutput selects the region from the most recent OSC 133
// command-executed mark through the line before the following prompt (or
// the end of the buffer). Returns false when no command-output range exists.
func (s *Session) SelectLastCommandOutput() bool {
	s.Term.RLock()
	marks := s.Term.PromptMarks()
	histLen := len(s.Term.History())
	screenLen := len(s.Term.Screen())
	cols := s.Term.Size().C
	s.Term.RUnlock()
	if cols <= 0 || len(marks) == 0 {
		return false
	}

	startAbs := -1
	for i := len(marks) - 1; i >= 0; i-- {
		if marks[i].Type == emu.CommandExecuted {
			startAbs = marks[i].AbsLine
			break
		}
	}
	if startAbs < 0 {
		return false
	}

	endAbs := histLen + screenLen - 1
	if endAbs < startAbs {
		endAbs = startAbs
	}
	for _, m := range marks {
		if m.Type == emu.PromptStart && m.AbsLine > startAbs {
			endAbs = m.AbsLine - 1
			break
		}
	}
	if endAbs < startAbs {
		endAbs = startAbs
	}

	s.selMu.Lock()
	s.sel = selectionState{
		has: true, complete: true,
		anchor: cellPos{0, startAbs},
		head:   cellPos{cols - 1, endAbs},
	}
	s.selMu.Unlock()

	s.lastPromptJump = startAbs
	s.ScrollToAbsLine(startAbs)
	return true
}
