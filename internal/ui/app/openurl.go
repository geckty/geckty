package app

import (
	"os/exec"
	"runtime"
)

// openURL launches the platform URL handler for u (https://..., file://...).
// Best-effort: failures are ignored — there's nowhere useful to surface them
// from a mouse click without a toast UI.
func openURL(u string) {
	if u == "" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", u) //nolint:gosec // G204: URL from terminal hyperlink / plain detect
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u) //nolint:gosec // G204
	default:
		cmd = exec.Command("xdg-open", u) //nolint:gosec // G204
	}
	_ = cmd.Start()
}
