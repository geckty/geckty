package gogpu

import (
	"testing"

	"github.com/gogpu/gpucontext"

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

func TestNewKeymapRejectsUnknownKey(t *testing.T) {
	_, err := NewKeymap([]config.Keybinding{
		{Key: "NotAKey", Mods: nil, Action: string(ActionNewTab)},
	})
	if err == nil {
		t.Fatal("expected error for unknown key")
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
	a, ok := k.Match(gpucontext.KeyT, gpucontext.ModControl|gpucontext.ModShift)
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
	a, ok := k.Match(gpucontext.KeyTab, gpucontext.ModControl)
	if !ok || a != ActionNextTab {
		t.Fatalf("Match = %q, %v; want next_tab, true", a, ok)
	}
}

func TestKeymapMatchRequiresExactModifiers(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "C", Mods: []string{"cmd"}, Action: string(ActionCopy)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	if _, ok := k.Match(gpucontext.KeyC, gpucontext.ModSuper|gpucontext.ModShift); ok {
		t.Fatal("Cmd+Shift+C must not match a Cmd+C binding")
	}
}

func TestKeymapMatchIgnoresCapsNumLock(t *testing.T) {
	k, err := NewKeymap([]config.Keybinding{
		{Key: "C", Mods: []string{"cmd"}, Action: string(ActionCopy)},
	})
	if err != nil {
		t.Fatalf("NewKeymap: %v", err)
	}
	a, ok := k.Match(gpucontext.KeyC, gpucontext.ModSuper|gpucontext.ModCapsLock)
	if !ok || a != ActionCopy {
		t.Fatalf("Match with CapsLock set = %q, %v; want copy, true", a, ok)
	}
}

func TestEncodeKeyCtrlLetter(t *testing.T) {
	b, ok := EncodeKey(gpucontext.KeyC, gpucontext.ModControl)
	if !ok || len(b) != 1 || b[0] != 3 {
		t.Fatalf("EncodeKey(Ctrl+C) = %v, %v; want [0x03], true", b, ok)
	}
}

func TestEncodeKeyNamed(t *testing.T) {
	b, ok := EncodeKey(gpucontext.KeyEnter, 0)
	if !ok || string(b) != "\r" {
		t.Fatalf("EncodeKey(Enter) = %v, %v; want \\r, true", b, ok)
	}
}

func TestEncodeKeyUnhandledLetterFallsThrough(t *testing.T) {
	// A plain letter with no modifiers has nothing to say here — its text
	// arrives via the SetOnTextInput callback instead.
	if _, ok := EncodeKey(gpucontext.KeyC, 0); ok {
		t.Fatal("EncodeKey(C, no mods) should return ok=false")
	}
}
