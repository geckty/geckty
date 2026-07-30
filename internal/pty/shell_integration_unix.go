//go:build !windows

package pty

import (
	"os"
	"path/filepath"
	"sync"
)

// applyShellIntegration wraps command/env so the shell about to be spawned
// emits OSC 133 semantic-prompt markers (see internal/vt/emu's OSC133*
// handling and vt.Terminal.CommandState) around every command it runs —
// zsh and bash only, since those are the two shells geckty knows how to
// hook without a general shell-plugin mechanism; any other $SHELL (fish,
// nushell, ...) is returned unmodified, same as before this existed (no
// OSC 133, no "command running" indicator).
//
// Callers (Open) must only call this for the platform-default-resolved
// shell (Config.Command empty) with Config.Integration set — an explicit
// Command is the caller's exact choice and is never modified. If the
// integration files can't be written (e.g. no writable temp dir), command/
// env are returned unmodified rather than failing the spawn — the
// indicator is a nice-to-have, not a reason to refuse to open a shell.
func applyShellIntegration(command, env []string) ([]string, []string) {
	dir, err := ensureShellIntegrationDir()
	if err != nil {
		return command, env
	}
	switch filepath.Base(command[0]) {
	case "zsh":
		origZDOTDIR := os.Getenv("ZDOTDIR")
		env = append(env,
			"GECKTY_ORIG_ZDOTDIR="+origZDOTDIR,
			"ZDOTDIR="+dir,
		)
	case "bash":
		rest := append([]string{"--rcfile", filepath.Join(dir, "bashrc")}, command[1:]...)
		command = append([]string{command[0]}, rest...)
	}
	return command, env
}

var (
	shellIntegrationOnce sync.Once
	shellIntegrationDir  string
	shellIntegrationErr  error
)

// ensureShellIntegrationDir lazily writes this process's shell-integration
// scripts to a private temp directory, once, reused for every shell
// spawned afterward (every tab, every window).
func ensureShellIntegrationDir() (string, error) {
	shellIntegrationOnce.Do(func() {
		dir, err := os.MkdirTemp("", "geckty-shell-integration-*")
		if err != nil {
			shellIntegrationErr = err
			return
		}
		files := map[string]string{
			".zshenv": zshenvIntegration,
			".zshrc":  zshrcIntegration,
			"bashrc":  bashrcIntegration,
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
				shellIntegrationErr = err
				return
			}
		}
		shellIntegrationDir = dir
	})
	return shellIntegrationDir, shellIntegrationErr
}

// zsh reads .zshenv unconditionally and .zshrc for interactive shells
// (geckty spawns a plain interactive, non-login shell — no .zprofile/
// .zlogin, those are login-shell-only) from $ZDOTDIR, which
// applyShellIntegration points at ensureShellIntegrationDir's directory
// instead of the user's real one (defaulting to $HOME). Both stubs below
// restore the user's actual startup files by absolute path — via
// GECKTY_ORIG_ZDOTDIR, the real value applyShellIntegration captured
// before overriding it — so nothing about the user's own setup is skipped,
// only added to.
const zshenvIntegration = `# geckty shell integration (see internal/pty/shell_integration_unix.go)
if [ -n "${GECKTY_ORIG_ZDOTDIR:-}" ] && [ -f "$GECKTY_ORIG_ZDOTDIR/.zshenv" ]; then
  source "$GECKTY_ORIG_ZDOTDIR/.zshenv"
elif [ -z "${GECKTY_ORIG_ZDOTDIR:-}" ] && [ -f "$HOME/.zshenv" ]; then
  source "$HOME/.zshenv"
fi
`

const zshrcIntegration = `# geckty shell integration (see internal/pty/shell_integration_unix.go)
if [ -n "${GECKTY_ORIG_ZDOTDIR:-}" ]; then
  export ZDOTDIR="$GECKTY_ORIG_ZDOTDIR"
else
  unset ZDOTDIR
fi
unset GECKTY_ORIG_ZDOTDIR
[ -f "${ZDOTDIR:-$HOME}/.zshrc" ] && source "${ZDOTDIR:-$HOME}/.zshrc"

__geckty_prompt_started=0
__geckty_precmd() {
  local ec=$?
  if [ "$__geckty_prompt_started" = 1 ]; then
    printf '\033]133;D;%s\033\\' "$ec"
  fi
  __geckty_prompt_started=1
  printf '\033]133;A\033\\'
}
__geckty_preexec() {
  printf '\033]133;C\033\\'
}
autoload -Uz add-zsh-hook 2>/dev/null && {
  add-zsh-hook precmd __geckty_precmd
  add-zsh-hook preexec __geckty_preexec
}
`

// bash --rcfile replaces ~/.bashrc sourcing outright (for the plain
// interactive non-login shell geckty spawns), so this stub sources the
// real one itself before adding hooks — PROMPT_COMMAND (runs before each
// prompt, like zsh's precmd) for prompt-start/command-finished, PS0 (bash
// 4.4+; runs right after a command line is read, before it executes) for
// command-start. Bash-only builtins (PROMPT_COMMAND chaining, PS0) are
// fine unconditionally: this file is only ever sourced by bash.
const bashrcIntegration = `# geckty shell integration (see internal/pty/shell_integration_unix.go)
[ -f "$HOME/.bashrc" ] && source "$HOME/.bashrc"

__geckty_prompt_started=0
__geckty_precmd() {
  local ec=$?
  if [ "$__geckty_prompt_started" = 1 ]; then
    printf '\033]133;D;%s\033\\' "$ec"
  fi
  __geckty_prompt_started=1
  printf '\033]133;A\033\\'
}
PROMPT_COMMAND='__geckty_precmd'"${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
PS0='\033]133;C\033\\'
`
