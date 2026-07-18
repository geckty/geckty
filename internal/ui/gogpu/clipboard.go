package gogpu

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// clipboardProvider is satisfied by *gogpu.App (kept as an interface so
// clipboard logic is testable without a live App).
type clipboardProvider interface {
	ClipboardWrite(text string) error
	ClipboardRead() (string, error)
}

// clipboardWrite puts text on the system clipboard. On macOS, pbcopy is
// tried first — per termizard's own field experience, gogpu's NSPasteboard
// writes can appear to succeed while leaving the system pasteboard
// unchanged. Falls back to the other path on error either way.
func clipboardWrite(app clipboardProvider, text string) error {
	if text == "" {
		return nil
	}
	if runtime.GOOS == "darwin" {
		if err := clipboardWriteNative(text); err == nil {
			return nil
		}
		return app.ClipboardWrite(text)
	}
	if err := app.ClipboardWrite(text); err == nil {
		return nil
	}
	return clipboardWriteNative(text)
}

// clipboardRead returns text from the system clipboard.
func clipboardRead(app clipboardProvider) (string, error) {
	if runtime.GOOS == "darwin" {
		if text, err := clipboardReadNative(); err == nil && text != "" {
			return text, nil
		}
	}
	text, err := app.ClipboardRead()
	if err == nil && text != "" {
		return text, nil
	}
	native, nerr := clipboardReadNative()
	if nerr == nil && native != "" {
		return native, nil
	}
	if err != nil {
		return "", err
	}
	return "", nerr
}

func clipboardWriteNative(text string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case osWindows:
		cmd = exec.Command("clip")
	default:
		if _, err := exec.LookPath("wl-copy"); err == nil {
			cmd = exec.Command("wl-copy")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else {
			return fmt.Errorf("no clipboard helper (wl-copy/xclip)")
		}
	}
	cmd.Stdin = strings.NewReader(text)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func clipboardReadNative() (string, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbpaste")
	case osWindows:
		cmd = exec.Command("powershell", "-NoProfile", "-Command", "Get-Clipboard -Raw")
	default:
		if _, err := exec.LookPath("wl-paste"); err == nil {
			cmd = exec.Command("wl-paste", "-n")
		} else if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
		} else {
			return "", fmt.Errorf("no clipboard helper (wl-paste/xclip)")
		}
	}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
