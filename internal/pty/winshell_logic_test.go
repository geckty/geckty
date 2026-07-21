package pty

import (
	"reflect"
	"testing"
)

func TestIsSpawnableWindowsExecutable(t *testing.T) {
	tests := []struct {
		name string
		path string
		size int64
		dir  bool
		want bool
	}{
		{"normal executable", `C:\Program Files\PowerShell\7\pwsh.exe`, 5000, false, true},
		{"empty path", "", 5000, false, false},
		{"directory", `C:\Program Files\PowerShell\7`, 5000, true, false},
		{"too small", `C:\Program Files\PowerShell\7\pwsh.exe`, 100, false, false},
		{"at the threshold", `C:\real\shell.exe`, 1024, false, false},
		{"just over the threshold", `C:\real\shell.exe`, 1025, false, true},
		{"WindowsApps stub, backslashes", `C:\Users\x\AppData\Local\Microsoft\WindowsApps\pwsh.exe`, 5000, false, false},
		{"WindowsApps stub, mixed case", `C:\Users\x\AppData\Local\Microsoft\WINDOWSAPPS\pwsh.exe`, 5000, false, false},
		{"WindowsApps stub, forward slashes", `C:/Users/x/AppData/Local/Microsoft/WindowsApps/pwsh.exe`, 5000, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSpawnableWindowsExecutable(tt.path, tt.size, tt.dir); got != tt.want {
				t.Fatalf("isSpawnableWindowsExecutable(%q, %d, %v) = %v, want %v", tt.path, tt.size, tt.dir, got, tt.want)
			}
		})
	}
}

func TestFilterAndCompleteInteractiveArgsAddsModeFlags(t *testing.T) {
	tests := []struct {
		name    string
		command []string
		want    []string
	}{
		{"pwsh gets NoLogo/NoExit/UTF-8", []string{`C:\pwsh.exe`}, []string{`C:\pwsh.exe`, "-NoLogo", "-NoExit", "-Command", windowsUTF8Init}},
		{"powershell.exe recognized case-insensitively", []string{`C:\PowerShell.EXE`}, []string{`C:\PowerShell.EXE`, "-NoLogo", "-NoExit", "-Command", windowsUTF8Init}},
		{"cmd gets /K + UTF-8 chcp", []string{`C:\Windows\System32\cmd.exe`}, []string{`C:\Windows\System32\cmd.exe`, "/K", "chcp", "65001"}},
		{"unrecognized shell gets nothing added", []string{`C:\bash.exe`}, []string{`C:\bash.exe`}},
		{"empty command is untouched", nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterAndCompleteInteractiveArgs(tt.command)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFilterAndCompleteInteractiveArgsStripsPosixFlags(t *testing.T) {
	got := filterAndCompleteInteractiveArgs([]string{"pwsh.exe", "-l", "--login", "-i", "-f", "--rcfile", "keep-me"})
	want := []string{"pwsh.exe", "keep-me"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterAndCompleteInteractiveArgsPreservesExplicitArgs(t *testing.T) {
	// A command that already carries its own arguments (after POSIX-only
	// flags are stripped) is assumed to already express whatever mode it
	// wants — no mode flags should be appended on top.
	got := filterAndCompleteInteractiveArgs([]string{"pwsh.exe", "-Command", "echo hi"})
	want := []string{"pwsh.exe", "-Command", "echo hi"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterAndCompleteInteractiveArgsDoesNotMutateInput(t *testing.T) {
	original := []string{"pwsh.exe"}
	_ = filterAndCompleteInteractiveArgs(original)
	if len(original) != 1 || original[0] != "pwsh.exe" {
		t.Fatalf("input slice was mutated: %v", original)
	}
}

func TestStripShellEnvVar(t *testing.T) {
	env := []string{"PATH=C:\\Windows", "SHELL=C:\\bash.exe", "shell=lowercase-too", "USER=me"}
	got := stripShellEnvVar(env, []string{"EXTRA=1"})
	want := []string{"PATH=C:\\Windows", "USER=me", "EXTRA=1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestStripShellEnvVarDoesNotMutateInput(t *testing.T) {
	env := []string{"SHELL=x", "PATH=y"}
	_ = stripShellEnvVar(env, nil)
	if !reflect.DeepEqual(env, []string{"SHELL=x", "PATH=y"}) {
		t.Fatalf("input slice was mutated: %v", env)
	}
}

func TestSortedUTF16EnvBlockEmpty(t *testing.T) {
	if got := sortedUTF16EnvBlock(nil); got != nil {
		t.Fatalf("got %v, want nil", got)
	}
}

func TestSortedUTF16EnvBlockSortsCaseInsensitively(t *testing.T) {
	// The original implementation used a case-sensitive sort.Strings,
	// which doesn't match CreateProcess's documented requirement (sorted
	// case-insensitively) — this test pins the corrected behavior:
	// "apple" must sort before "Banana" even though 'a' > 'B' in ASCII.
	block := sortedUTF16EnvBlock([]string{"Banana=2", "apple=1"})
	decoded := decodeUTF16EnvBlock(t, block)
	if !reflect.DeepEqual(decoded, []string{"apple=1", "Banana=2"}) {
		t.Fatalf("got %v, want [apple=1 Banana=2]", decoded)
	}
}

func TestSortedUTF16EnvBlockRoundTrips(t *testing.T) {
	env := []string{"ZED=1", "ALPHA=2"}
	block := sortedUTF16EnvBlock(env)

	// Must end with a double NUL (an extra zero uint16 after the last
	// entry's own terminating zero).
	if len(block) < 2 || block[len(block)-1] != 0 || block[len(block)-2] != 0 {
		t.Fatalf("block does not end in a double NUL: %v", block)
	}

	decoded := decodeUTF16EnvBlock(t, block)
	if !reflect.DeepEqual(decoded, []string{"ALPHA=2", "ZED=1"}) {
		t.Fatalf("got %v, want [ALPHA=2 ZED=1]", decoded)
	}
}

// decodeUTF16EnvBlock reverses sortedUTF16EnvBlock's encoding, splitting
// on single NULs and stopping at the final double NUL, so tests can
// assert against plain strings instead of raw uint16 slices.
func decodeUTF16EnvBlock(t *testing.T, block []uint16) []string {
	t.Helper()
	var entries []string
	var cur []uint16
	for _, u := range block {
		if u == 0 {
			if len(cur) == 0 {
				break // the final, second NUL terminates the whole block
			}
			entries = append(entries, string(utf16Decode(cur)))
			cur = nil
			continue
		}
		cur = append(cur, u)
	}
	return entries
}

func utf16Decode(u []uint16) []rune {
	// Minimal decode sufficient for ASCII test fixtures — avoids pulling
	// in unicode/utf16 just for the test when this repo's env values
	// never exercise surrogate pairs.
	out := make([]rune, len(u))
	for i, v := range u {
		out[i] = rune(v)
	}
	return out
}
