package app

import "testing"

func TestTruncateTitleKeepsEnd(t *testing.T) {
	short := "Users/foo"
	if got := truncateTitle(short); got != short {
		t.Fatalf("short = %q, want unchanged", got)
	}
	long := `C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe`
	got := truncateTitle(long)
	if r := []rune(got); len(r) == 0 || r[0] != '…' {
		t.Fatalf("got %q, want leading ellipsis", got)
	}
	if !hasSuffixRunes(got, "powershell.exe") {
		t.Fatalf("got %q, want end kept (powershell.exe)", got)
	}
	if hasPrefixASCII(got, `C:\WINDOWS`) {
		t.Fatalf("got %q, should not keep path start", got)
	}
}

func hasSuffixRunes(s, suffix string) bool {
	sr, zr := []rune(s), []rune(suffix)
	if len(sr) < len(zr) {
		return false
	}
	for i := range zr {
		if sr[len(sr)-len(zr)+i] != zr[i] {
			return false
		}
	}
	return true
}

func hasPrefixASCII(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestLooksLikeShellExeTitle(t *testing.T) {
	cases := map[string]bool{
		"powershell.exe": true,
		"pwsh.exe":       true,
		"cmd.exe":        true,
		`C:\WINDOWS\System32\WindowsPowerShell\v1.0\powershell.exe`: true,
		`/Users/foo/project`: false,
		"":                   false,
	}
	for title, want := range cases {
		if got := looksLikeShellExeTitle(title); got != want {
			t.Errorf("looksLikeShellExeTitle(%q) = %v, want %v", title, got, want)
		}
	}
}
