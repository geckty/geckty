# Tech debt — deferred Kitty parity

Items consciously deferred from the daily-driver Kitty catch-up roadmap.
Not forgotten bugs — deprioritized by complexity vs selection, scrollback,
splits, links, and shell integration.

## P2 — Font ligatures (`liga` / `calt`)

**Status:** Reserved only (`# ligatures = false` in `config.example.toml`).  
**Effort:** L.

Painting is per-cell via a glyph atlas (`internal/ui/termview`). Real
ligatures need shaping (HarfBuzz or equivalent), cursor-cell disable, and
selection/hit-test alignment. Font stack is `golang.org/x/image/font` /
opentype rasterization only — no GSUB.

## P3 — Multi OS window

**Status:** Optional after in-window splits.  
**Effort:** M–L.

Tabs + splits + remote control (`GECKTY_SOCKET` / `geckty @`) already cover
most multi-session workflows. A second top-level window multiplies focus,
GPU surfaces, config reload, and quit across platforms.

## P4 — Full Kitty keyboard protocol

**Status:** Partial — Disambiguate + Report event types work.  
**Location:** `internal/protocol/kittykbd`, `emu.KeyState()`.

| Kitty flag | Blocker |
|---|---|
| Repeat | No OS autorepeat event from `gpucontext` |
| Report alternate keys | Layout / base-vs-shifted info missing |
| Report all keys as escapes | Text-input vs key ownership redesign |
| Associated text | Depends on Report all keys |

Common apps usually need only disambiguate + event types.

## P4 — Kitty graphics z-index

**Status:** Images always paint above text.  
**Location:** `internal/protocol/kittygfx`, paint placements.

MVP covers `a=T` / `a=d` / `a=q` and direct payloads. Negative-z “behind
text” is a niche TUI case.

## Suggested revisit order

1. Graphics z-index (smallest paint/protocol delta)  
2. Multi OS window (product decision)  
3. Full Kitty keyboard (driven by a real app need)  
4. Ligatures (largest dependency change)

## Related

- Shipped around this debt: AbsLine selection, search, splits, OSC 8/133,
  hints, dirty-row paint, OSC 52 read (allow), RC socket, gfx delete/query.
- Active polish: [roadmap.md](roadmap.md).
