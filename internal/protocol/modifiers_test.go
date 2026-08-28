package protocol

import "testing"

func TestModifiersContain(t *testing.T) {
	m := ModShift | ModCtrl
	if !m.Contain(ModShift) {
		t.Fatal("expected ModShift contained")
	}
	if !m.Contain(ModShift | ModCtrl) {
		t.Fatal("expected combined mask contained")
	}
	if m.Contain(ModAlt) {
		t.Fatal("ModAlt should not be contained")
	}
	if (Modifiers(0)).Contain(ModShift) {
		t.Fatal("empty modifiers should contain nothing")
	}
}
