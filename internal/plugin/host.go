// Package plugin runs geckty's user-authored WASM plugins via wazero
// (pure Go, no cgo — avoids stacking a second cgo dependency on top of
// gio's own cgo GPU bindings). Like internal/session and internal/vt, this
// package has no gio import; internal/ui decides when to call a plugin's
// hooks and what to do with the results (see internal/ui/app.go's plugin
// wiring and the project plan's M9 write-up for the redraw-cadence
// decision).
//
// # Guest toolchain
//
// Plugins are plain Go, built with GOOS=wasip1 GOARCH=wasm
// -buildmode=c-shared — no TinyGo or other external toolchain required.
// -buildmode=c-shared matters, not just GOOS/GOARCH: Go's default wasip1
// build exports "_start", which is WASI Command-module semantics (it runs
// main() and then calls proc_exit, closing the module) — fine for a
// one-shot CLI, useless for a plugin that needs to stay resident and
// answer further calls. -buildmode=c-shared exports "_initialize" instead
// (WASI Reactor semantics): it initializes the Go runtime without running
// main() or exiting, leaving the module resident for the host to call
// exported functions on repeatedly. This was verified empirically (a
// throwaway spike) before committing to it — Go's wasip1 runtime panics
// with "wasmexport function called before runtime initialization" if a
// wasmexport function is invoked without _initialize (or _start) having
// run first, and the default _start build closes the module immediately
// after running, so c-shared's _initialize is the only one of the three
// that actually stays usable.
//
// Guest code exports functions with "//go:wasmexport name" and imports
// host functions with "//go:wasmimport geckty name" (see api.go for what
// host functions exist). See plugins/examples/statusbar-clock/main.go for
// a complete, real example.
package plugin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// Host runs plugins loaded through it. One Host owns one wazero Runtime,
// shared by every plugin instantiated through it — each plugin still gets
// its own isolated module instance (own linear memory, own globals), only
// the compiler/engine and the "geckty" host-function namespace are shared.
type Host struct {
	rt wazero.Runtime

	mu      sync.Mutex
	ordered []*Plugin
	byName  map[string]*Plugin // wazero module name -> owning Plugin, see api.go's callerOf
}

// NewHost starts a wazero runtime with a WASI preview 1 environment (Go's
// wasip1 runtime imports WASI functions for things like time.Now() and its
// own startup, even though geckty's own host functions in api.go don't use
// WASI) plus geckty's custom "geckty" host-function namespace.
func NewHost(ctx context.Context) (*Host, error) {
	rt := wazero.NewRuntime(ctx)
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, rt); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("instantiate WASI: %w", err)
	}

	h := &Host{rt: rt, byName: make(map[string]*Plugin)}
	if err := h.registerHostFunctions(ctx); err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("register host functions: %w", err)
	}
	return h, nil
}

// Plugin is one loaded, running plugin instance.
type Plugin struct {
	Name string

	mod         api.Module
	permissions map[string]bool

	statusMu   sync.Mutex
	statusText string
}

// Load reads dir/plugin.toml and instantiates dir/<entry> (the manifest's
// entry field, e.g. "entry.wasm"), returning the running Plugin. The
// plugin is not activated — call Activate once every plugin for this
// session has finished loading.
func (h *Host) Load(ctx context.Context, dir string) (*Plugin, error) {
	m, err := loadManifest(dir)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	_, exists := h.byName[m.Name]
	h.mu.Unlock()
	if exists {
		return nil, fmt.Errorf("plugin %q: a plugin with this name is already loaded", m.Name)
	}

	wasmPath := filepath.Join(dir, m.Entry)
	// wasmPath is built from an operator-configured directory
	// (config.Config.Plugins) and the manifest's own entry field, not
	// remote or otherwise untrusted input.
	wasmBytes, err := os.ReadFile(wasmPath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("plugin %q: reading %s: %w", m.Name, wasmPath, err)
	}

	compiled, err := h.rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: compiling %s: %w", m.Name, wasmPath, err)
	}

	perms := make(map[string]bool, len(m.Permissions))
	for _, p := range m.Permissions {
		perms[p] = true
	}
	p := &Plugin{Name: m.Name, permissions: perms}

	modCfg := wazero.NewModuleConfig().
		WithName(m.Name).
		WithStartFunctions("_initialize") // reactor entry point — see the package doc comment

	mod, err := h.rt.InstantiateModule(ctx, compiled, modCfg)
	if err != nil {
		return nil, fmt.Errorf("plugin %q: instantiating: %w", m.Name, err)
	}
	p.mod = mod

	h.mu.Lock()
	h.ordered = append(h.ordered, p)
	h.byName[m.Name] = p
	h.mu.Unlock()

	return p, nil
}

// Plugins returns every plugin currently loaded, in load order.
func (h *Host) Plugins() []*Plugin {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]*Plugin(nil), h.ordered...)
}

// Close shuts down the wazero runtime and every plugin instantiated
// through it.
func (h *Host) Close(ctx context.Context) error {
	return h.rt.Close(ctx)
}

// hasPermission reports whether the plugin's manifest declared name.
func (p *Plugin) hasPermission(name string) bool {
	return p.permissions[name]
}
