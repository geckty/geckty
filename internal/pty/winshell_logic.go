package pty

import (
	"sort"
	"strings"
	"unicode/utf16"
)

// This file holds the pure decision logic behind the Windows PTY backend's
// shell resolution and environment handling (conpty_windows.go,
// shell_windows.go, env_windows.go) — deliberately kept free of any
// windows-only import or //go:build tag. Those callers still do the real
// os.Stat/os.Environ/syscall work (only meaningful on Windows, and this
// repo's CI is the only place that ever runs a compiled Windows binary —
// see the project plan's M10 write-up), but the decisions themselves
// (is this path a real, spawnable executable; which flags does this
// command line need; does this environment block need SHELL stripped;
// what does the sorted UTF-16 block look like) are ordinary string/slice
// logic with no OS dependency, so they belong somewhere every platform's
// `go test` can actually exercise, not trapped behind a build tag no
// darwin/linux CI job can compile past.

// isSpawnableWindowsExecutable reports whether a candidate shell path is
// one CreateProcess+ConPTY can actually launch, given facts already
// gathered about it (size and whether it's a directory) — the caller
// (shell_windows.go's resolveShell) does the real os.Stat.
//
// Store/WindowsApps "App Execution Alias" stubs are 0-byte-ish reparse
// points that appear on PATH and pass a naive existence check but can't
// actually be spawned; rejecting anything under \WindowsApps\ (case-
// insensitively, forward or backward slashes) plus a small size floor
// catches them without needing to parse the reparse point itself.
func isSpawnableWindowsExecutable(path string, size int64, isDir bool) bool {
	if path == "" || isDir {
		return false
	}
	normalized := strings.ToLower(windowsPathToSlash(path))
	if strings.Contains(normalized, "/windowsapps/") {
		return false
	}
	return size > 1024
}

// windowsPathToSlash and windowsBaseName parse a Windows-style path
// (backslash or forward-slash separated, per CreateProcess/cmd.exe's own
// tolerance of both) explicitly, rather than via path/filepath — that
// package's Base/ToSlash behave per the *host* OS's separator convention,
// not per the separator convention of the path string's own origin. These
// functions describe Windows paths specifically and must behave
// identically regardless of which OS compiles or runs them (this package
// compiles on every OS; only conpty_windows.go et al. are windows-only —
// see this file's doc comment), which path/filepath does not guarantee:
// on a POSIX host, filepath.Base("C:\pwsh.exe") returns the whole string
// unchanged (backslash isn't a POSIX separator), not "pwsh.exe".
func windowsPathToSlash(path string) string {
	return strings.ReplaceAll(path, `\`, "/")
}

func windowsBaseName(path string) string {
	slashed := windowsPathToSlash(path)
	if _, after, ok := strings.CutLast(slashed, "/"); ok {
		return after
	}
	return slashed
}

// windowsUTF8Init forces UTF-8 console I/O so ConPTY bytes match the VT
// parser (OEM code pages otherwise garble Cyrillic and prompt glyphs).
const windowsUTF8Init = "[Console]::InputEncoding=[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false); chcp 65001 > $null; $Host.UI.RawUI.WindowTitle=(Get-Location).Path"

// windowsInteractiveModeFlags maps a shell executable's lowercase base
// name to the flags that keep it open and interactive when the caller
// didn't already specify one. Shells not listed here get no flags added —
// filterAndCompleteInteractiveArgs still strips POSIX-only flags for them.
var windowsInteractiveModeFlags = map[string][]string{
	"pwsh.exe":       {"-NoLogo", "-NoExit", "-Command", windowsUTF8Init},
	"powershell.exe": {"-NoLogo", "-NoExit", "-Command", windowsUTF8Init},
	"cmd.exe":        {"/K", "chcp", "65001"},
}

// posixOnlyShellFlags are stripped from a command line before launching on
// Windows — they'd have carried over from a cross-platform default config
// (e.g. a POSIX login-shell flag) that a Windows shell doesn't understand.
var posixOnlyShellFlags = map[string]bool{
	"-l": true, "--login": true, "-i": true, "-f": true, "--rcfile": true,
}

// filterAndCompleteInteractiveArgs strips posixOnlyShellFlags from
// command[1:], then — only if that left no arguments at all — appends the
// interactive-mode flags windowsInteractiveModeFlags names for
// command[0]'s executable name (recognized case-insensitively by base
// name, ignoring any directory prefix). A command that already carries
// its own arguments is assumed to already express whatever mode it wants
// and is left alone past the flag-stripping pass.
func filterAndCompleteInteractiveArgs(command []string) []string {
	if len(command) == 0 {
		return command
	}

	filtered := command[:1:1]
	for _, arg := range command[1:] {
		if posixOnlyShellFlags[strings.ToLower(arg)] {
			continue
		}
		filtered = append(filtered, arg)
	}
	command = filtered

	if len(command) > 1 {
		return command
	}

	exe := strings.ToLower(windowsBaseName(command[0]))
	if flags, ok := windowsInteractiveModeFlags[exe]; ok {
		return append(command, flags...)
	}
	return command
}

// stripShellEnvVar removes any SHELL=... entry from env (matched case-
// insensitively on the key, per Windows environment-variable convention)
// and appends extra. Git for Windows sets SHELL to a POSIX shell path
// (e.g. …\bash.exe); letting that leak into a ConPTY child launched from
// within it breaks downstream tools that check $SHELL and expect a native
// Windows shell.
func stripShellEnvVar(env []string, extra []string) []string {
	filtered := env[:0:0]
	for _, kv := range env {
		if strings.HasPrefix(strings.ToUpper(kv), "SHELL=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	return append(filtered, extra...)
}

// sortedUTF16EnvBlock encodes env as the UTF-16 "key=value\0" ...  "\0\0"
// double-NUL-terminated block CreateProcess expects when passed with
// CREATE_UNICODE_ENVIRONMENT, sorted case-insensitively as CreateProcess's
// documentation requires (an unsorted block is accepted but undocumented
// behavior differs across Windows versions for duplicate keys). Returns
// nil for an empty env — env_windows.go's makeEnvBlock treats that as "no
// custom environment block" (nil *uint16) rather than an empty one.
func sortedUTF16EnvBlock(env []string) []uint16 {
	if len(env) == 0 {
		return nil
	}

	sorted := append([]string(nil), env...)
	sort.Slice(sorted, func(i, j int) bool {
		return strings.ToLower(sorted[i]) < strings.ToLower(sorted[j])
	})

	var block []uint16
	for _, kv := range sorted {
		block = append(block, utf16.Encode([]rune(kv))...)
		block = append(block, 0)
	}
	return append(block, 0)
}
