package app

import (
	"sync/atomic"

	gogpulib "github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// fakeApp is a gpuApp test double: no live window/GPU stack, just enough
// bookkeeping to assert what uiState's setup and event-handling code
// actually calls. redrawCount/quit/inSizeMove are atomics because
// RequestRedraw is genuinely called concurrently in production — a
// session's PTY-read goroutine invokes it via OnDirty — and tests here
// spawn real sessions the same way, so the fake must be just as
// concurrency-safe as the real gogpulib.App it replaces.
type fakeApp struct {
	redrawCount atomic.Int64
	quit        atomic.Bool
	inSizeMove  atomic.Bool

	onFocusFn            func(bool)
	onCloseFn            func()
	onSurfaceAvailableFn func()

	clipboard    string
	clipReadErr  error
	clipWriteErr error
	cursor       gpucontext.CursorShape

	events *fakeEventSource
}

func newFakeApp() *fakeApp {
	return &fakeApp{events: newFakeEventSource()}
}

func (f *fakeApp) Run() error       { return nil }
func (f *fakeApp) RequestRedraw()   { f.redrawCount.Add(1) }
func (f *fakeApp) Quit()            { f.quit.Store(true) }
func (f *fakeApp) InSizeMove() bool { return f.inSizeMove.Load() }

func (f *fakeApp) OnFocus(fn func(bool)) *gogpulib.App {
	f.onFocusFn = fn
	return nil
}

func (f *fakeApp) OnClose(fn func()) *gogpulib.App {
	f.onCloseFn = fn
	return nil
}

func (f *fakeApp) OnSurfaceAvailable(fn func()) *gogpulib.App {
	f.onSurfaceAvailableFn = fn
	return nil
}

// PrimaryWindow always returns nil: gpuApp's exact signature ties this to
// the concrete *gogpulib.Window type (see platform.go's doc comment on
// why that can't be narrowed to gpuWindow), and a fake can't fabricate a
// working one. Tests that need to exercise wireWindow's body call it
// directly with a fakeWindow instead of going through this path.
func (f *fakeApp) PrimaryWindow() *gogpulib.Window { return nil }

func (f *fakeApp) EventSource() gpucontext.EventSource { return f.events }

func (f *fakeApp) ClipboardWrite(text string) error {
	if f.clipWriteErr != nil {
		return f.clipWriteErr
	}
	f.clipboard = text
	return nil
}

func (f *fakeApp) ClipboardRead() (string, error) {
	if f.clipReadErr != nil {
		return "", f.clipReadErr
	}
	return f.clipboard, nil
}

func (f *fakeApp) SetCursor(shape gpucontext.CursorShape) {
	f.cursor = shape
}

// fakeEventSource implements gpucontext.EventSource plus the
// PointerEventSource/ScrollEventSource extensions wireEventSource type-
// asserts for, recording exactly which callbacks got registered.
type fakeEventSource struct {
	gpucontext.NullEventSource

	keyPressFn    func(gpucontext.Key, gpucontext.Modifiers)
	keyReleaseFn  func(gpucontext.Key, gpucontext.Modifiers)
	textInputFn   func(string)
	pointerFn     func(gpucontext.PointerEvent)
	scrollEventFn func(gpucontext.ScrollEvent)
}

func newFakeEventSource() *fakeEventSource { return &fakeEventSource{} }

func (f *fakeEventSource) OnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	f.keyPressFn = fn
}

func (f *fakeEventSource) OnKeyRelease(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	f.keyReleaseFn = fn
}

func (f *fakeEventSource) OnTextInput(fn func(string)) {
	f.textInputFn = fn
}

func (f *fakeEventSource) OnPointer(fn func(gpucontext.PointerEvent)) {
	f.pointerFn = fn
}

func (f *fakeEventSource) OnScrollEvent(fn func(gpucontext.ScrollEvent)) {
	f.scrollEventFn = fn
}

var (
	_ gpucontext.EventSource        = (*fakeEventSource)(nil)
	_ gpucontext.PointerEventSource = (*fakeEventSource)(nil)
	_ gpucontext.ScrollEventSource  = (*fakeEventSource)(nil)
)

// fakeWindow is a gpuWindow test double recording which callback each
// SetOnXxx call registered, so tests can invoke them directly.
type fakeWindow struct {
	onDraw       func(*gogpulib.Context)
	onResize     func(int, int)
	onKeyPress   func(gpucontext.Key, gpucontext.Modifiers)
	onKeyRelease func(gpucontext.Key, gpucontext.Modifiers)
	onTextInput  func(string)
	onPointer    func(gpucontext.PointerEvent)
	onScroll     func(gpucontext.ScrollEvent)
	onClose      func() bool
}

func (f *fakeWindow) SetOnDraw(fn func(*gogpulib.Context))                        { f.onDraw = fn }
func (f *fakeWindow) SetOnResize(fn func(int, int))                               { f.onResize = fn }
func (f *fakeWindow) SetOnKeyPress(fn func(gpucontext.Key, gpucontext.Modifiers)) { f.onKeyPress = fn }
func (f *fakeWindow) SetOnKeyRelease(fn func(gpucontext.Key, gpucontext.Modifiers)) {
	f.onKeyRelease = fn
}
func (f *fakeWindow) SetOnTextInput(fn func(string))                { f.onTextInput = fn }
func (f *fakeWindow) SetOnPointer(fn func(gpucontext.PointerEvent)) { f.onPointer = fn }
func (f *fakeWindow) SetOnScroll(fn func(gpucontext.ScrollEvent))   { f.onScroll = fn }
func (f *fakeWindow) SetOnClose(fn func() bool)                     { f.onClose = fn }

var _ gpuWindow = (*fakeWindow)(nil)
