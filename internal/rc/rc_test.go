package rc

import (
	"strings"
	"testing"
)

type fakeHost struct {
	tabs   []string
	text   string
	sent   string
	newN   int
	closeN int
}

func (f *fakeHost) NewTab() error               { f.newN++; f.tabs = append(f.tabs, "tab"); return nil }
func (f *fakeHost) CloseTab() error             { f.closeN++; return nil }
func (f *fakeHost) SendText(text string) error  { f.sent = text; return nil }
func (f *fakeHost) GetText() (string, error)    { return f.text, nil }
func (f *fakeHost) ListTabs() ([]string, error) { return append([]string(nil), f.tabs...), nil }

func TestParseLine(t *testing.T) {
	cmd, err := ParseLine("new_tab")
	if err != nil || cmd.Name != "new_tab" {
		t.Fatalf("ParseLine new_tab: %+v %v", cmd, err)
	}
	cmd, err = ParseLine("send_text hello world")
	if err != nil || cmd.Name != "send_text" || cmd.Arg != "hello world" {
		t.Fatalf("ParseLine send_text: %+v %v", cmd, err)
	}
	cmd, err = ParseLine("send-text hi")
	if err != nil || cmd.Name != "send_text" || cmd.Arg != "hi" {
		t.Fatalf("ParseLine send-text: %+v %v", cmd, err)
	}
	if _, err := ParseLine("nope"); err == nil {
		t.Fatal("expected error for unknown command")
	}
}

func TestHandleLine(t *testing.T) {
	h := &fakeHost{tabs: []string{"a", "b"}, text: "line1\nline2"}
	if got := HandleLine(h, "new_tab"); got != "OK" || h.newN != 1 {
		t.Fatalf("new_tab: %q n=%d", got, h.newN)
	}
	if got := HandleLine(h, "send_text hi"); got != "OK" || h.sent != "hi" {
		t.Fatalf("send_text: %q sent=%q", got, h.sent)
	}
	if got := HandleLine(h, "get_text"); got != "OK line1\\nline2" {
		t.Fatalf("get_text: %q", got)
	}
	if got := HandleLine(h, "list_tabs"); !strings.HasPrefix(got, "OK ") || !strings.Contains(got, "a") {
		t.Fatalf("list_tabs: %q", got)
	}
	if got := HandleLine(h, "close_tab"); got != "OK" || h.closeN != 1 {
		t.Fatalf("close_tab: %q", got)
	}
	if got := HandleLine(h, "bogus"); !strings.HasPrefix(got, "ERR ") {
		t.Fatalf("bogus: %q", got)
	}
}
