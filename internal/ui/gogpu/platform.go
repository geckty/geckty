package gogpu

import (
	gogpulib "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// gpuApp is the subset of *gogpulib.App's method set uiState actually
// calls. Defining it as an interface (rather than depending on the
// concrete *gogpulib.App type directly) lets tests exercise Run's setup
// helpers, dispatchAction's clipboard calls, and every pointer/keyboard
// handler against a lightweight fake instead of a live windowing/GPU
// stack — constructing a real gogpulib.App/Window requires an actual
// platform surface, which isn't available in a `go test` process.
//
// Deliberately NOT abstracted: *gogpulib.Context (onDraw's texture
// upload/present calls, which need a real GPU surface to mean anything).
// OnFocus/OnClose/OnSurfaceAvailable return *gogpulib.App for a fluent-
// chaining style this package never actually uses (the return value is
// always discarded at the call site); narrowing that return type would
// need covariant interface methods Go doesn't support, for no testing
// benefit since nothing here reads the chained result. Run is included
// so top-level Run(cfg) can still call gapp.Run() through the interface —
// a fake's Run can simply return nil rather than actually pumping a
// platform event loop.
type gpuApp interface {
	Run() error
	RequestRedraw()
	Quit()
	InSizeMove() bool
	OnFocus(fn func(focused bool)) *gogpulib.App
	OnClose(fn func()) *gogpulib.App
	OnSurfaceAvailable(fn func()) *gogpulib.App
	PrimaryWindow() *gogpulib.Window
	EventSource() gpucontext.EventSource
	ClipboardWrite(text string) error
	ClipboardRead() (string, error)
}

// gpuWindow is the subset of *gogpulib.Window's method set wireWindow
// calls. See gpuApp's doc comment for why this exists.
type gpuWindow interface {
	SetOnDraw(fn func(*gogpulib.Context))
	SetOnResize(fn func(int, int))
	SetOnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers))
	SetOnTextInput(fn func(string))
	SetOnPointer(fn func(gpucontext.PointerEvent))
	SetOnScroll(fn func(gpucontext.ScrollEvent))
	SetOnClose(fn func() bool)
}

var (
	_ gpuApp    = (*gogpulib.App)(nil)
	_ gpuWindow = (*gogpulib.Window)(nil)
)
