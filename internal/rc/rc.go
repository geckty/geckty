// Package rc provides a minimal remote-control socket for geckty
// (Kitty-style @ commands over a Unix domain socket, or TCP on Windows).
package rc

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Environment variable names that enable the remote-control listener /
// client. SocketPath prefers EnvSocket over EnvListen.
const (
	EnvSocket = "GECKTY_SOCKET"
	EnvListen = "GECKTY_LISTEN"
)

// Command names accepted by ParseLine / HandleLine and by `geckty @`.
const (
	CmdNewTab    = "new_tab"
	CmdCloseTab  = "close_tab"
	CmdListTabs  = "list_tabs"
	CmdGetText   = "get_text"
	CmdSendText  = "send_text"
	CmdSendTextA = "send-text" // CLI alias for CmdSendText
)

// DialTimeout bounds client connect + request/response for DialAndSend.
const DialTimeout = 3 * time.Second

// Host is the UI/session surface the listener invokes.
type Host interface {
	NewTab() error
	CloseTab() error
	SendText(text string) error
	GetText() (string, error)
	ListTabs() ([]string, error)
}

// SocketPath returns the listen address from EnvSocket or EnvListen.
// Empty means remote control is disabled.
func SocketPath() string {
	if p := strings.TrimSpace(os.Getenv(EnvSocket)); p != "" {
		return p
	}
	return strings.TrimSpace(os.Getenv(EnvListen))
}

// ListenAndServe accepts connections on path (Unix socket, or tcp on
// Windows when path looks like host:port / is a bare port). stop closes
// the listener. Transient Accept errors are logged and retried; only a
// closed listener (via stop) ends the accept loop.
func ListenAndServe(path string, host Host) (stop func(), err error) {
	if path == "" || host == nil {
		return func() {}, nil
	}
	ln, err := listen(path)
	if err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	done := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-done:
					return
				default:
					slog.Warn("remote control accept", slog.String("path", path), slog.Any("error", err))
					time.Sleep(50 * time.Millisecond)
					continue
				}
			}
			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				defer func() { _ = c.Close() }()
				_ = serveConn(c, host)
			}(conn)
		}
	}()
	return func() {
		close(done)
		_ = ln.Close()
		wg.Wait()
		if runtime.GOOS != "windows" {
			_ = os.Remove(path)
		}
	}, nil
}

func listen(path string) (net.Listener, error) {
	if runtime.GOOS == "windows" {
		return net.Listen("tcp", windowsListenAddr(path))
	}
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

func windowsListenAddr(path string) string {
	if strings.Contains(path, ":") {
		return path
	}
	return "127.0.0.1:" + strings.TrimPrefix(path, ":")
}

func serveConn(c net.Conn, host Host) error {
	_ = c.SetDeadline(time.Now().Add(DialTimeout))
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		_ = c.SetDeadline(time.Now().Add(DialTimeout))
		resp := HandleLine(host, line)
		if _, err := io.WriteString(c, resp+"\n"); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Command is a parsed remote-control request.
type Command struct {
	Name string
	Arg  string
}

// ParseLine parses one remote-control request line.
func ParseLine(line string) (Command, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Command{}, fmt.Errorf("empty command")
	}
	name, rest, _ := strings.Cut(line, " ")
	name = strings.ToLower(strings.TrimSpace(name))
	rest = strings.TrimSpace(rest)
	switch name {
	case CmdNewTab, CmdCloseTab, CmdListTabs, CmdGetText:
		if rest != "" {
			return Command{}, fmt.Errorf("%s takes no arguments", name)
		}
		return Command{Name: name}, nil
	case CmdSendText, CmdSendTextA:
		if rest == "" {
			return Command{}, fmt.Errorf("%s requires text", CmdSendText)
		}
		return Command{Name: CmdSendText, Arg: rest}, nil
	default:
		return Command{}, fmt.Errorf("unknown command %q", name)
	}
}

// HandleLine parses and executes one line, returning a single-line response
// (OK / OK <payload> / ERR <msg>).
func HandleLine(host Host, line string) string {
	cmd, err := ParseLine(line)
	if err != nil {
		return "ERR " + err.Error()
	}
	switch cmd.Name {
	case CmdNewTab:
		if err := host.NewTab(); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case CmdCloseTab:
		if err := host.CloseTab(); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case CmdSendText:
		if err := host.SendText(cmd.Arg); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case CmdGetText:
		text, err := host.GetText()
		if err != nil {
			return "ERR " + err.Error()
		}
		return "OK " + strings.ReplaceAll(text, "\n", "\\n")
	case CmdListTabs:
		tabs, err := host.ListTabs()
		if err != nil {
			return "ERR " + err.Error()
		}
		return "OK " + strings.Join(tabs, ",")
	default:
		return "ERR unknown command"
	}
}

// DialAndSend connects to path and sends one command line, returning the
// response line (without trailing newline). Connect and I/O are bounded
// by DialTimeout.
func DialAndSend(path, line string) (string, error) {
	conn, err := dial(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(DialTimeout))
	if _, err := io.WriteString(conn, line+"\n"); err != nil {
		return "", err
	}
	sc := bufio.NewScanner(conn)
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("no response")
	}
	return sc.Text(), nil
}

func dial(path string) (net.Conn, error) {
	if runtime.GOOS == "windows" {
		return net.DialTimeout("tcp", windowsListenAddr(path), DialTimeout)
	}
	return net.DialTimeout("unix", path, DialTimeout)
}
