package input

import (
	"testing"

	"gioui.org/io/key"

	"github.com/geckty/geckty/internal/config"
)

func TestNewKeymapRejectsUnknownAction(t *testing.T) {
	_, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"ctrl"}, Action: "not_a_real_action"},
	})
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestNewKeymapRejectsUnknownModifier(t *testing.T) {
	_, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"hyper"}, Action: string(ActionNewTab)},
	})
	if err == nil {
		t.Fatal("expected error for unknown modifier")
	}
}

func TestNewKeymapRejectsEmptyKey(t *testing.T) {
	_, err := NewKeymap([]config.Keybinding{
		{Key: "", Mods: nil, Action: string(ActionNewTab)},
	})
	if err == nil {
		t.Fatal("expected error for empty key")
	}
}

func TestKeymapMatchLiteralKey(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"ctrl", "shift"}, Action: string(ActionNewTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	a, ok := k.Match(key.Event{Name: "T", Modifiers: key.ModCtrl | key.ModShift, State: key.Press})
	if !ok || a != ActionNewTab {
		t.Fatalf("Match = %q, %v; want new_tab, true", a, ok)
	}
}

func TestKeymapMatchNamedKey(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "Tab", Mods: []string{"ctrl"}, Action: string(ActionNextTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	a, ok := k.Match(key.Event{Name: key.NameTab, Modifiers: key.ModCtrl, State: key.Press})
	if !ok || a != ActionNextTab {
		t.Fatalf("Match = %q, %v; want next_tab, true", a, ok)
	}
}

func TestKeymapMatchRequiresExactModifiers(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"ctrl", "shift"}, Action: string(ActionNewTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	// Ctrl alone (no shift) must not match a Ctrl+Shift binding.
	if _, ok := k.Match(key.Event{Name: "T", Modifiers: key.ModCtrl, State: key.Press}); ok {
		t.Fatal("expected no match with only Ctrl held")
	}
	// An extra modifier beyond what's configured must not match either.
	if _, ok := k.Match(key.Event{Name: "T", Modifiers: key.ModCtrl | key.ModShift | key.ModAlt, State: key.Press}); ok {
		t.Fatal("expected no match with an extra modifier held")
	}
}

func TestKeymapMatchIgnoresRelease(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"ctrl"}, Action: string(ActionNewTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	if _, ok := k.Match(key.Event{Name: "T", Modifiers: key.ModCtrl, State: key.Release}); ok {
		t.Fatal("expected Release events not to match (would double-fire the action)")
	}
}

func TestKeymapMatchRussianLayoutShortcutAliases(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "C", Mods: []string{"cmd"}, Action: string(ActionCopy)},
		{Key: "V", Mods: []string{"cmd"}, Action: string(ActionPaste)},
		{Key: "[", Mods: []string{"cmd", "shift"}, Action: string(ActionPrevTab)},
		{Key: "]", Mods: []string{"cmd", "shift"}, Action: string(ActionNextTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}

	// Russian layout physical equivalents: C->С, V->М, [->Х, ]->Ъ.
	if a, ok := k.Match(key.Event{Name: "С", Modifiers: key.ModCommand, State: key.Press}); !ok || a != ActionCopy {
		t.Fatalf("Match(ru С) = %q, %v; want copy, true", a, ok)
	}
	if a, ok := k.Match(key.Event{Name: "М", Modifiers: key.ModCommand, State: key.Press}); !ok || a != ActionPaste {
		t.Fatalf("Match(ru М) = %q, %v; want paste, true", a, ok)
	}
	if a, ok := k.Match(key.Event{Name: "Х", Modifiers: key.ModCommand | key.ModShift, State: key.Press}); !ok || a != ActionPrevTab {
		t.Fatalf("Match(ru Х) = %q, %v; want prev_tab, true", a, ok)
	}
	if a, ok := k.Match(key.Event{Name: "Ъ", Modifiers: key.ModCommand | key.ModShift, State: key.Press}); !ok || a != ActionNextTab {
		t.Fatalf("Match(ru Ъ) = %q, %v; want next_tab, true", a, ok)
	}
}

func TestKeymapFiltersOneFilterPerBinding(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "T", Mods: []string{"ctrl"}, Action: string(ActionNewTab)},
		{Key: "W", Mods: []string{"ctrl"}, Action: string(ActionCloseTab)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	if got := len(k.Filters(struct{}{})); got != 4 {
		t.Fatalf("len(Filters()) = %d, want 4 (latin + russian aliases)", got)
	}
}

func TestNewKeymapFromDefaults(t *testing.T) {
	// The real default keybindings (platform-aware) must compile into a
	// valid Keymap without error — catches typos in defaults.go.
	if _, err := NewKeymap(config.Default().Keybindings); err != nil {
		t.Fatalf("NewKeymap(defaults): %v", err)
	}
}
