package rc

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeHost struct {
	tabs   []string
	text   string
	sent   string
	newN   int
	closeN int

	newErr   error
	closeErr error
	sendErr  error
	getErr   error
	listErr  error
}

func (f *fakeHost) NewTab() error {
	if f.newErr != nil {
		return f.newErr
	}
	f.newN++
	f.tabs = append(f.tabs, "tab")
	return nil
}
func (f *fakeHost) CloseTab() error {
	if f.closeErr != nil {
		return f.closeErr
	}
	f.closeN++
	return nil
}
func (f *fakeHost) SendText(text string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = text
	return nil
}
func (f *fakeHost) GetText() (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.text, nil
}
func (f *fakeHost) ListTabs() ([]string, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]string(nil), f.tabs...), nil
}

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
	if _, err := ParseLine(""); err == nil {
		t.Fatal("expected error for empty command")
	}
	if _, err := ParseLine("new_tab extra"); err == nil {
		t.Fatal("expected error when new_tab has args")
	}
	if _, err := ParseLine("send_text"); err == nil {
		t.Fatal("expected error for send_text without text")
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

func TestHandleLineHostErrors(t *testing.T) {
	errBoom := errors.New("boom")
	cases := []struct {
		line string
		host *fakeHost
	}{
		{"new_tab", &fakeHost{newErr: errBoom}},
		{"close_tab", &fakeHost{closeErr: errBoom}},
		{"send_text x", &fakeHost{sendErr: errBoom}},
		{"get_text", &fakeHost{getErr: errBoom}},
		{"list_tabs", &fakeHost{listErr: errBoom}},
	}
	for _, tc := range cases {
		got := HandleLine(tc.host, tc.line)
		if !strings.HasPrefix(got, "ERR ") || !strings.Contains(got, "boom") {
			t.Fatalf("%s: %q", tc.line, got)
		}
	}
}

func TestSocketPath(t *testing.T) {
	t.Setenv(EnvSocket, "")
	t.Setenv(EnvListen, "")
	if got := SocketPath(); got != "" {
		t.Fatalf("SocketPath empty env = %q", got)
	}
	t.Setenv(EnvListen, " listen-path ")
	if got := SocketPath(); got != "listen-path" {
		t.Fatalf("SocketPath LISTEN = %q", got)
	}
	t.Setenv(EnvSocket, " sock-path ")
	if got := SocketPath(); got != "sock-path" {
		t.Fatalf("SocketPath SOCKET wins = %q", got)
	}
}

func TestListenAndServeEmptyIsNoop(t *testing.T) {
	stop, err := ListenAndServe("", &fakeHost{})
	if err != nil {
		t.Fatal(err)
	}
	stop()
	stop, err = ListenAndServe("/tmp/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	stop()
}

func TestListenAndServeDialAndSend(t *testing.T) {
	path := testListenPath(t)
	h := &fakeHost{tabs: []string{"one"}, text: "hi"}
	stop, err := ListenAndServe(path, h)
	if err != nil {
		t.Fatalf("ListenAndServe: %v", err)
	}
	defer stop()

	deadline := time.Now().Add(2 * time.Second)
	var resp string
	for {
		resp, err = DialAndSend(path, "list_tabs")
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("DialAndSend list_tabs: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if resp != "OK one" {
		t.Fatalf("list_tabs resp = %q", resp)
	}
	resp, err = DialAndSend(path, "send_text hello")
	if err != nil || resp != "OK" || h.sent != "hello" {
		t.Fatalf("send_text: resp=%q err=%v sent=%q", resp, err, h.sent)
	}
	resp, err = DialAndSend(path, "get_text")
	if err != nil || resp != "OK hi" {
		t.Fatalf("get_text: resp=%q err=%v", resp, err)
	}
	resp, err = DialAndSend(path, "new_tab")
	if err != nil || resp != "OK" || h.newN != 1 {
		t.Fatalf("new_tab: resp=%q err=%v n=%d", resp, err, h.newN)
	}
	resp, err = DialAndSend(path, "close_tab")
	if err != nil || resp != "OK" || h.closeN != 1 {
		t.Fatalf("close_tab: resp=%q err=%v n=%d", resp, err, h.closeN)
	}
	resp, err = DialAndSend(path, "nope")
	if err != nil || !strings.HasPrefix(resp, "ERR ") {
		t.Fatalf("nope: resp=%q err=%v", resp, err)
	}
}

func TestDialAndSendMissingSocket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.sock")
	if runtime.GOOS == "windows" {
		path = "127.0.0.1:1"
	}
	if _, err := DialAndSend(path, "list_tabs"); err == nil {
		t.Fatal("expected dial error")
	}
}

func TestWindowsListenAddr(t *testing.T) {
	if got := windowsListenAddr("127.0.0.1:9"); got != "127.0.0.1:9" {
		t.Fatalf("host:port = %q", got)
	}
	if got := windowsListenAddr(":1234"); got != ":1234" {
		t.Fatalf("port only = %q", got)
	}
}

func TestServeConnSkipsBlankLines(t *testing.T) {
	client, server := net.Pipe()
	defer func() { _ = client.Close() }()
	defer func() { _ = server.Close() }()

	done := make(chan error, 1)
	go func() {
		done <- serveConn(server, &fakeHost{tabs: []string{"a"}})
	}()

	_, _ = io.WriteString(client, "\n  list_tabs  \n")
	sc := bufio.NewScanner(client)
	if !sc.Scan() {
		t.Fatal("expected response")
	}
	if sc.Text() != "OK a" {
		t.Fatalf("resp = %q", sc.Text())
	}
	_ = client.Close()
	if err := <-done; err != nil {
		t.Fatalf("serveConn: %v", err)
	}
}

func testListenPath(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		// Ephemeral TCP port.
		ln, err := listen("127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		addr := ln.Addr().String()
		_ = ln.Close()
		return addr
	}
	// macOS sockaddr_un path limit is ~104 bytes; keep this short.
	path := filepath.Join(os.TempDir(), fmt.Sprintf("gckty-%d.sock", os.Getpid()))
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}
