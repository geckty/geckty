//go:build windows

package pty

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// PeekNamedPipe isn't exposed by golang.org/x/sys/windows, so it's bound
// directly here — needed by conPTY.Read's poll loop (see doc comment there).
var (
	kernel32          = windows.NewLazySystemDLL("kernel32.dll")
	procPeekNamedPipe = kernel32.NewProc("PeekNamedPipe")
)

func peekNamedPipe(pipe windows.Handle) (bytesAvail uint32, err error) {
	r1, _, e1 := procPeekNamedPipe.Call(
		uintptr(pipe),
		0, 0, 0,
		uintptr(unsafe.Pointer(&bytesAvail)),
		0,
	)
	if r1 == 0 {
		return 0, e1
	}
	return bytesAvail, nil
}
