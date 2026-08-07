# Tech debt — deferred Kitty parity work

Items consciously deferred from the daily-driver Kitty catch-up roadmap.
They are not forgotten bugs; they were deprioritized by complexity and
user-facing value relative to selection, scrollback, splits, links, and
shell integration.

## P2 — Font ligatures (`liga` / `calt`)

**Status:** Reserved only (`# ligatures = false` in `config.example.toml`).  
**Plan difficulty:** L.

### Why deferred

- Painting is per-cell / per-rune via a glyph atlas (`internal/ui/gogpu/painter.go`).
  Ligatures need a shaping pass that maps multi-codepoint sequences to one
  glyph spanning multiple columns.
- The font stack uses `golang.org/x/image/font` / `opentype` for rasterization
  only — no GSUB shaping. Real ligatures need HarfBuzz (or equivalent) plus
  cross-platform build wiring.
- Extra requirements: disable ligatures under the cursor, correct cell-width
  advances, selection/hit-test alignment, atlas rebuild on DPI/font zoom.

### Follow-up

1. Integrate HarfBuzz (or a Go shaping library) for the mono face.
2. Shape runs before grid paint; cache shaped clusters.
3. Disable ligatures at the cursor cell; expose `font.ligatures` in config.
4. Tests for common ligatures (`!=`, `->`, `=>`) and cursor disable.

## P3 — Multi OS window

**Status:** Explicitly optional in the Phase 3 plan (“after splits”).  
**Plan difficulty:** M–L.

### Why deferred

- Phase 3 prioritized **native pane splits inside one OS window** (Kitty-style
  windows-in-a-tab). That closed the main multiplexer gap vs tmux/Kitty.
- A second top-level window multiplies lifecycle: focus, GPU surfaces,
  config hot-reload, plugins, clipboard, confirm-close — across macOS /
  Windows / Linux.
- Tabs + splits + remote control (`GECKTY_SOCKET` / `geckty @`) already cover
  most multi-session workflows without a second OS frame.

### Follow-up

1. Decide process model: multi-window in one process vs one process per window.
2. Extend `gogpu` app wiring beyond a single primary window.
3. Shared session manager / RC across windows.
4. Platform QA for focus and quit behavior.

## P4 — Full Kitty keyboard protocol

**Status:** Partial — Disambiguate + Report event types (press/release) work.  
**Location:** `internal/protocol/kittykbd`, `emu.KeyState()`.

### Why deferred

| Kitty flag | Blocker |
|---|---|
| Repeat | `gpucontext` only exposes Press/Release; no OS autorepeat event |
| Report alternate keys | Needs layout / base-vs-shifted key info not in key events |
| Report all keys as escapes | Requires redesigning the text-input → literal path |
| Associated text | Depends on Report all keys |

Emu already tracks the progressive-enhancement flags; the encoder does not
fully act on them. Common apps (editors, shells) usually need only
disambiguate + event types.

### Follow-up

1. If a concrete app requires missing flags, implement that flag first.
2. For repeat: obtain autorepeat from the platform layer or synthesize carefully.
3. For Report all keys: redesign key vs text-input ownership in `ui/gogpu`.

## P4 — Kitty graphics z-index

**Status:** Images always paint above text.  
**Location:** `internal/protocol/kittygfx` (package doc), `paintPlacements`.

### Why deferred

- MVP graphics support covers `a=T` / `a=d` / `a=q` and direct (`t=d`) payloads.
- Z-index needs per-placement `z`, layered paint (behind text → text → above),
  and usually `a=p` / multi-placement-by-id.
- Typical CLI image use (`icat`, etc.) places images above text; negative-z
  “behind text” is a niche TUI case.

### Follow-up

1. Parse and store `z` on placements.
2. Split paint into behind-text and above-text passes.
3. Add `a=p` (place by image id) if reuse is required.
4. Tests for z ordering vs cell glyphs.

## Suggested priority (if revisiting)

1. Graphics z-index (smallest protocol/paint delta)
2. Multi OS window (product decision + platform work)
3. Full Kitty keyboard (driven by a real app requirement)
4. Ligatures (largest dependency and rendering change)

## Related

- Roadmap context: Kitty daily-driver parity (not a full kitten/`@` clone).
- Already shipped around this debt: AbsLine selection, search, splits, OSC 8/133,
  hints, dirty-row paint, OSC 52 read (allow), RC socket, gfx delete/query.
