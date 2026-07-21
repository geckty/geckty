//go:build windows

package pty

import "os"

// preparePlatformEnv builds the child environment from the current process
// environment plus cfg.Env, with one deliberate override: SHELL is never
// inherited (see stripShellEnvVar in winshell_logic.go for why).
func preparePlatformEnv(extra []string) []string {
	return stripShellEnvVar(os.Environ(), extra)
}

// makeEnvBlock encodes env as the UTF-16 double-NUL-terminated block
// CreateProcess expects when passed with CREATE_UNICODE_ENVIRONMENT (see
// sortedUTF16EnvBlock in winshell_logic.go for the encoding itself).
func makeEnvBlock(env []string) (*uint16, error) {
	block := sortedUTF16EnvBlock(env)
	if block == nil {
		return nil, nil
	}
	return &block[0], nil
}
