// Command statusbar-clock is a geckty plugin: it shows the current local
// time in geckty's status area. It's the reference example for the
// internal/plugin WASM plugin host — see that package's doc comment for
// the guest-side ABI (wasmimport/wasmexport, the malloc/free convention,
// why it must be built with -buildmode=c-shared).
//
// Build with:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o entry.wasm .
//
// Then add this directory to geckty's config.toml:
//
//	plugins = ["/path/to/plugins/examples/statusbar-clock"]
package main

import (
	"time"
	"unsafe"
)

//go:wasmimport geckty log
func hostLog(ptr, length uint32)

//go:wasmimport geckty statusbar_draw
func hostStatusbarDraw(ptr, length uint32)

// keepAlive retains every buffer handed out by malloc for the module's
// lifetime. A plugin this small has no need for a real free-list
// allocator — see malloc/free below.
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

// malloc and free are exported for the host's use, per the malloc/free
// buffer-passing convention internal/plugin's ABI commits to — this
// plugin's own hooks (on_activate, draw_statusbar) don't happen to need
// host-to-guest data, but exporting them costs nothing and keeps every
// plugin uniformly ready for hooks that will (see internal/plugin/api.go's
// writeToGuest).
//
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
	logString("statusbar-clock activated")
}

//go:wasmexport draw_statusbar
func drawStatusbar() {
	statusbarDraw(time.Now().Format("15:04:05"))
}

func main() {}
