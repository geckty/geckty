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

func TestOSC52BridgeDenyWrite(t *testing.T) {
	b := newOSC52Bridge(ClipboardPolicy{WriteAllow: false, MaxSize: 64}, nil)
	b.OSC52Write("c", []byte("secret"))
	if _, _, ok := b.take(); ok {
		t.Fatal("denied write must not queue a clipboard payload")
	}
}

func TestOSC52BridgeTooLarge(t *testing.T) {
	b := newOSC52Bridge(ClipboardPolicy{WriteAllow: true, MaxSize: 4}, nil)
	b.OSC52Write("c", []byte("too-big"))
	if _, _, ok := b.take(); ok {
		t.Fatal("oversized payload must be dropped")
	}
}

func TestOSC52BridgeReadDenied(t *testing.T) {
	b := newOSC52Bridge(ClipboardPolicy{ReadAllow: false}, nil)
	if _, ok := b.OSC52Read("c"); ok {
		t.Fatal("read must be denied by default")
	}
	b = newOSC52Bridge(ClipboardPolicy{ReadAllow: true}, nil)
	if _, ok := b.OSC52Read("c"); ok {
		t.Fatal("ReadAllow still returns no host clipboard data")
	}
}
