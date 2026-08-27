package app

import (
	"os"
	"os/exec"
	"runtime"
)

// showScrollbackInPager writes the active session's History()+Screen() to a
// temp file and launches $PAGER (or less / notepad) non-blocking.
func (s *uiState) showScrollbackInPager() {
	active := s.mgr.Active()
	if active == nil {
		return
	}
	text := active.ScrollbackText()
	f, err := os.CreateTemp("", "geckty-scrollback-*.txt")
	if err != nil {
		return
	}
	path := f.Name()
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return
	}
	_ = f.Close()

	pager := os.Getenv("PAGER")
	var cmd *exec.Cmd
	switch {
	case pager != "":
		cmd = exec.Command(pager, path) //nolint:gosec // G204: user $PAGER + temp scrollback file
	case runtime.GOOS == "windows":
		cmd = exec.Command("notepad", path) //nolint:gosec // G204: temp scrollback path
	default:
		cmd = exec.Command("less", path) //nolint:gosec // G204: temp scrollback path
	}
	_ = cmd.Start()
}
