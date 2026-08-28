package config

import "testing"

func TestActionConstantsNonEmpty(t *testing.T) {
	actions := []string{
		ActionNewTab, ActionCloseTab, ActionClosePane,
		ActionNextTab, ActionPrevTab, ActionNextPane, ActionPrevPane,
		ActionSplitVertical, ActionSplitHorizontal,
		ActionCopy, ActionPaste, ActionSearchScrollback, ActionOpenURLHints,
		ActionShowScrollback, ActionIncreaseFontSize, ActionDecreaseFontSize,
		ActionResetFontSize, ActionScrollToPrevPrompt, ActionScrollToNextPrompt,
		ActionSelectLastCmdOutput,
	}
	for _, a := range actions {
		if a == "" {
			t.Fatal("action constant must not be empty")
		}
	}
}

func TestModifierConstantsNonEmpty(t *testing.T) {
	for _, m := range []string{ModCtrl, ModShift, ModAlt, ModCmd} {
		if m == "" {
			t.Fatal("modifier constant must not be empty")
		}
	}
}
