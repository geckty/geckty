// Package rc provides a minimal remote-control socket for geckty
// (Kitty-style @ commands over a Unix domain socket, or TCP on Windows).
package rc

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
)

// Host is the UI/session surface the listener invokes.
type Host interface {
	NewTab() error
	CloseTab() error
	SendText(text string) error
	GetText() (string, error)
	ListTabs() ([]string, error)
}

// SocketPath returns the listen address from GECKTY_SOCKET or GECKTY_LISTEN.
// Empty means remote control is disabled.
func SocketPath() string {
	if p := strings.TrimSpace(os.Getenv("GECKTY_SOCKET")); p != "" {
		return p
	}
	return strings.TrimSpace(os.Getenv("GECKTY_LISTEN"))
}

// ListenAndServe accepts connections on path (Unix socket, or tcp on
// Windows when path looks like host:port / is a bare port). stop closes
// the listener.
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
					return
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
		addr := path
		if !strings.Contains(path, ":") {
			addr = "127.0.0.1:" + strings.TrimPrefix(path, ":")
		}
		return net.Listen("tcp", addr)
	}
	_ = os.Remove(path)
	return net.Listen("unix", path)
}

func serveConn(c net.Conn, host Host) error {
	sc := bufio.NewScanner(c)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
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
	case "new_tab", "close_tab", "list_tabs", "get_text":
		if rest != "" {
			return Command{}, fmt.Errorf("%s takes no arguments", name)
		}
		return Command{Name: name}, nil
	case "send_text", "send-text":
		if rest == "" {
			return Command{}, fmt.Errorf("send_text requires text")
		}
		return Command{Name: "send_text", Arg: rest}, nil
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
	case "new_tab":
		if err := host.NewTab(); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case "close_tab":
		if err := host.CloseTab(); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case "send_text":
		if err := host.SendText(cmd.Arg); err != nil {
			return "ERR " + err.Error()
		}
		return "OK"
	case "get_text":
		text, err := host.GetText()
		if err != nil {
			return "ERR " + err.Error()
		}
		return "OK " + strings.ReplaceAll(text, "\n", "\\n")
	case "list_tabs":
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
// response line (without trailing newline).
func DialAndSend(path, line string) (string, error) {
	conn, err := dial(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()
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
		addr := path
		if !strings.Contains(path, ":") {
			addr = "127.0.0.1:" + strings.TrimPrefix(path, ":")
		}
		return net.Dial("tcp", addr)
	}
	return net.Dial("unix", path)
}
