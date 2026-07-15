// Command fixture is a minimal WASM guest used only by
// internal/plugin's own tests (see host_test.go's buildFixture) — not a
// real geckty plugin. It exercises every part of the M9 host ABI: guest
// hooks (on_activate, draw_statusbar), a host-to-guest data transfer
// (echo, exercising writeToGuest's malloc-based buffer passing), and the
// malloc/free exports that mechanism depends on.
//
// Build with: GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o
// entry.wasm . — buildmode=c-shared is what makes Go export "_initialize"
// (WASI reactor semantics) instead of "_start" (WASI command semantics,
// which runs main and exits, closing the module) — see
// internal/plugin/host.go's package doc comment for why that matters.
package main

import "unsafe"

//go:wasmimport geckty log
func hostLog(ptr, length uint32)

//go:wasmimport geckty statusbar_draw
func hostStatusbarDraw(ptr, length uint32)

// keepAlive retains every buffer handed out by malloc for the module's
// lifetime — plugins are short-lived and small, so a real free-list
// allocator isn't worth the complexity; free is a no-op.
var keepAlive [][]byte

func ptrOf(b []byte) uint32 {
	if len(b) == 0 {
		return 0
	}
	return uint32(uintptr(unsafe.Pointer(&b[0])))
}

func logString(s string) {
	b := []byte(s)
	hostLog(ptrOf(b), uint32(len(b)))
}

func statusbarDraw(s string) {
	b := []byte(s)
	hostStatusbarDraw(ptrOf(b), uint32(len(b)))
}

//go:wasmexport malloc
func malloc(size uint32) uint32 {
	buf := make([]byte, size)
	keepAlive = append(keepAlive, buf)
	return ptrOf(buf)
}

//go:wasmexport free
func free(_ uint32) {}

//go:wasmexport on_activate
func onActivate() {
	logString("activated")
}

//go:wasmexport draw_statusbar
func drawStatusbar() {
	statusbarDraw("fixture-status")
}

// echo reads length bytes the host wrote into this module's memory at
// ptr (via writeToGuest's malloc call) and reports them back through
// statusbar_draw, prefixed, so a test can confirm host-to-guest data
// actually crossed the boundary intact.
//
//go:wasmexport echo
func echo(ptr, length uint32) {
	b := unsafe.Slice((*byte)(unsafe.Pointer(uintptr(ptr))), length)
	statusbarDraw("echo:" + string(b))
}

func main() {}
