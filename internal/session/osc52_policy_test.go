package session

import "testing"

func TestParseClipboardPolicy(t *testing.T) {
	p := ParseClipboardPolicy("allow", "deny", 0)
	if !p.WriteAllow || p.ReadAllow || p.MaxSize != defaultMaxOSC52 {
		t.Fatalf("defaults: %+v", p)
	}
	p = ParseClipboardPolicy("deny", "allow", 100)
	if p.WriteAllow || !p.ReadAllow || p.MaxSize != 100 {
		t.Fatalf("custom: %+v", p)
	}
}
