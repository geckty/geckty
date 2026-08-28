package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/geckty/geckty/internal/rc"
)

// runRemote handles `geckty @ <cmd> [args…]` against rc.EnvSocket / rc.EnvListen.
func runRemote(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: geckty @ <%s|%s|%s|%s|%s> [args]",
			rc.CmdNewTab, rc.CmdCloseTab, rc.CmdSendTextA, rc.CmdGetText, rc.CmdListTabs)
	}
	path := rc.SocketPath()
	if path == "" {
		return fmt.Errorf("%s (or %s) is not set", rc.EnvSocket, rc.EnvListen)
	}
	cmd := strings.ToLower(args[0])
	var line string
	switch cmd {
	case rc.CmdNewTab, rc.CmdCloseTab, rc.CmdGetText, rc.CmdListTabs:
		line = cmd
	case rc.CmdSendTextA, rc.CmdSendText:
		if len(args) < 2 {
			return fmt.Errorf("usage: geckty @ %s <text>", rc.CmdSendTextA)
		}
		line = rc.CmdSendText + " " + strings.Join(args[1:], " ")
	default:
		return fmt.Errorf("unknown remote command %q", args[0])
	}
	resp, err := rc.DialAndSend(path, line)
	if err != nil {
		return err
	}
	fmt.Println(resp)
	if strings.HasPrefix(resp, "ERR ") {
		os.Exit(1)
	}
	return nil
}
