//go:build windows

package pty

import (
	"os"
	"path/filepath"
)

// resolveShell picks the best interactive shell available: pwsh (PowerShell
// 7+) > Windows PowerShell > cmd.exe. Store/WindowsApps "App Execution
// Alias" stubs are skipped — see isSpawnableWindowsExecutable in
// winshell_logic.go.
func resolveShell() string {
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), `PowerShell\7\pwsh.exe`),
		filepath.Join(os.Getenv("ProgramFiles"), `PowerShell\7-preview\pwsh.exe`),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), `PowerShell\7\pwsh.exe`),
		filepath.Join(os.Getenv("SystemRoot"), `System32\WindowsPowerShell\v1.0\powershell.exe`),
	}
	for _, c := range candidates {
		if isSpawnable(c) {
			return c
		}
	}
	if comspec := os.Getenv("COMSPEC"); isSpawnable(comspec) {
		return comspec
	}
	return filepath.Join(os.Getenv("SystemRoot"), `System32\cmd.exe`)
}

// isSpawnable stats path and defers the actual spawnability decision to
// isSpawnableWindowsExecutable in winshell_logic.go.
func isSpawnable(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return isSpawnableWindowsExecutable(path, info.Size(), info.IsDir())
}

// ensureInteractive appends the flags needed to keep the shell open and
// interactive when the caller didn't already specify a mode flag, and
// strips POSIX-only flags a Windows shell wouldn't understand — see
// filterAndCompleteInteractiveArgs in winshell_logic.go.
func ensureInteractive(command []string) []string {
	return filterAndCompleteInteractiveArgs(command)
}
