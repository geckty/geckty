# Engineering notes — quality backlog

Follow-ups from the enterprise review.

## Done

### Wave 1 (P0/P1)
- RC: Accept retry + dial/I/O deadlines; env/command constants
- Shared `config.Action*` / `Mod*` / `ContentBracketsOff`
- Map aliases: `vt.DirtyRows`, `plugin.PermissionSet`, `input.*Table`
- DRY: tab bar sizing via `chrome.*`; `theme.DefaultSelection`; ANSI indices
- Package docs: `termview`, `input`; widget/helper godoc

### Wave 2 (P2)
- `internal/ui/raster` — shared buffer / roundrect / glyph atlas
- Theme merge field tables; `defaultUI()` from embedded glass.toml
- `protocol.Modifiers` shared by mouse + kittykbd
- Split: kittygfx Feed, frost, dispatch, ConPTY Open, main helpers

### Wave 3
- Buffer/roundrect/atlas tests live in `internal/ui/raster`; app/termview call `raster.*` directly (shims removed)
- `config.Glass*` vars loaded from embedded `glass.toml` in `init()`; chrome alphas sync from them

## Still open (optional)

| Item | Notes |
|------|-------|
| `dispatchAction` map table | Only if actions keep growing |

Do not expand scope into vendored `internal/vt/emu`.
