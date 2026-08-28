package plugin

import (
	"context"
	"log/slog"

	"github.com/tetratelabs/wazero/api"
)

const (
	hostExportLog       = "log"
	hostExportStatusbar = "statusbar_draw"
)

// registerHostFunctions declares the "geckty" namespace's host functions.
// The project plan's "Core seams" names a fuller eventual set (grid_cell,
// grid_size, register_osc_handler, statusbar_draw, config_get, log); M9
// implements only what its one example plugin (a statusbar clock) needs:
// log, for debug output, and statusbar_draw, for a plugin to push the
// text it wants shown. grid_cell/grid_size (reading the terminal grid),
// register_osc_handler (intercepting OSC sequences), and config_get
// (reading geckty config) are deferred — clean extensions on top of this
// same registration mechanism once a plugin actually needs them.
//
// Every host function's mod api.Module parameter is bound by wazero to
// the *calling* (guest) module at each invocation, not to this host
// module itself — that's what lets one shared "geckty" host module serve
// every loaded plugin: callerOf(mod) identifies which Plugin made this
// particular call, for permission checks and to route results (e.g.
// which Plugin's statusText to update). A call from a mod wazero doesn't
// recognize (shouldn't happen — every guest module is instantiated
// through Host.Load, which registers it before the guest can call
// anything) or one whose manifest didn't declare the needed permission is
// silently a no-op, not a trap: a plugin missing a permission should
// behave as if the capability just does nothing, not crash.
func (h *Host) registerHostFunctions(ctx context.Context) error {
	builder := h.rt.NewHostModuleBuilder("geckty")

	builder.NewFunctionBuilder().
		WithFunc(func(ctx context.Context, mod api.Module, ptr, length uint32) {
			p := h.callerOf(mod)
			if p == nil || !p.hasPermission(PermissionLog) {
				return
			}
			msg, ok := readGuestString(mod, ptr, length)
			if !ok {
				return
			}
			slog.InfoContext(ctx, msg, "plugin", p.Name)
		}).
		WithParameterNames("ptr", "length").
		Export(hostExportLog)

	builder.NewFunctionBuilder().
		WithFunc(func(_ context.Context, mod api.Module, ptr, length uint32) {
			p := h.callerOf(mod)
			if p == nil || !p.hasPermission(PermissionStatusbar) {
				return
			}
			text, ok := readGuestString(mod, ptr, length)
			if !ok {
				return
			}
			p.statusMu.Lock()
			p.statusText = text
			p.statusMu.Unlock()
		}).
		WithParameterNames("ptr", "length").
		Export(hostExportStatusbar)

	_, err := builder.Instantiate(ctx)
	return err
}

// callerOf identifies which loaded Plugin owns mod, the guest module
// wazero bound a host function call to.
func (h *Host) callerOf(mod api.Module) *Plugin {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.byName[mod.Name()]
}

func readGuestString(mod api.Module, ptr, length uint32) (string, bool) {
	buf, ok := mod.Memory().Read(ptr, length)
	if !ok {
		return "", false
	}
	return string(buf), true
}

// writeToGuest allocates len(s) bytes in mod's own memory via its
// exported malloc and writes s into it, returning the resulting pointer —
// the host-to-guest half of the malloc/free-based buffer-passing
// convention the project plan commits to ("byte buffers cross the
// boundary via guest-exported malloc/free and shared linear memory, not
// string marshaling tricks"). ok is false if mod doesn't export malloc, or
// the allocation/write failed.
//
// Unused by M9's actual hooks — on_activate and draw_statusbar both take
// no arguments, so nothing yet needs to hand a plugin host-owned data.
// Still implemented and directly tested here (see api_test.go) rather
// than left until some future hook (on_key, config_get) needs it: those
// hooks will need exactly this mechanism, and proving the ABI works now,
// against a real compiled guest, is worth more than a plan-document
// promise.
func writeToGuest(ctx context.Context, mod api.Module, s string) (ptr uint32, ok bool) {
	malloc := mod.ExportedFunction("malloc")
	if malloc == nil {
		return 0, false
	}
	b := []byte(s)
	res, err := malloc.Call(ctx, uint64(len(b)))
	if err != nil || len(res) == 0 {
		return 0, false
	}
	ptr = uint32(res[0])
	if len(b) == 0 {
		return ptr, true
	}
	if !mod.Memory().Write(ptr, b) {
		return 0, false
	}
	return ptr, true
}
