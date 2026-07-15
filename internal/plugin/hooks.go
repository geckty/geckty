package plugin

import "context"

// callHook calls the guest-exported function named name, if it exists,
// with no arguments and no return values — the shape every M9 hook uses
// (on_activate, draw_statusbar). ok is false if the plugin doesn't export
// a function by that name, which is not an error: a plugin only
// implements the hooks it cares about.
func (p *Plugin) callHook(ctx context.Context, name string) (ok bool, err error) {
	fn := p.mod.ExportedFunction(name)
	if fn == nil {
		return false, nil
	}
	if _, err := fn.Call(ctx); err != nil {
		return true, err
	}
	return true, nil
}

// Activate calls the plugin's on_activate hook, if it exports one. Call
// once per plugin after Host.Load, before any other hook.
func (p *Plugin) Activate(ctx context.Context) error {
	_, err := p.callHook(ctx, "on_activate")
	return err
}

// DrawStatusbar calls the plugin's draw_statusbar hook, if it exports
// one. The plugin is expected to call back into the host's
// statusbar_draw function (see api.go) with the text it wants shown,
// retrievable afterward via StatusText — DrawStatusbar itself returns no
// text directly, since a plugin without the "statusbar" permission can
// still export draw_statusbar but have nothing displayed (its
// statusbar_draw calls are silently denied, see registerHostFunctions),
// and a plugin might reasonably call statusbar_draw zero or more than
// once per hook invocation.
//
// on_output and on_key (the other two hooks the project plan names) are
// deliberately not implemented: they'd need geckty to route PTY output
// and key events into the plugin — a real integration into
// session.Session's read loop and internal/ui's input handling — which
// this milestone's one example plugin (a statusbar clock) has no use
// for. Adding them is a clean extension on top of this same callHook
// mechanism when a plugin that needs them exists.
func (p *Plugin) DrawStatusbar(ctx context.Context) error {
	_, err := p.callHook(ctx, "draw_statusbar")
	return err
}

// StatusText returns the text most recently pushed via the plugin's
// statusbar_draw host-function call (see api.go), or "" if it has never
// called it (e.g. no draw_statusbar export, or it lacks the "statusbar"
// permission).
func (p *Plugin) StatusText() string {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	return p.statusText
}
