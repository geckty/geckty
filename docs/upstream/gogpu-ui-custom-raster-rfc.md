# RFC: Custom raster apps on gogpu/ui compositor

**Repo:** `gogpu/ui`  
**Status:** draft (geckty upstream proposal)  
**Related:** ADR-007, ADR-034, ADR-036, [VEE #468](https://github.com/orgs/gogpu/discussions/468)

## Problem

`gogpu/ui` has retained-mode compositor features (RepaintBoundary, damage blit,
frame skip) but no documented pattern for apps that CPU-rasterize custom content
(terminals, editors) without importing `core/*`.

geckty bypasses the compositor entirely via `PresentTexture`.

## Proposal

Document and support **custom raster leaf** pattern:

1. `desktop.Run` + `app.New` with `RenderModeHostManaged` (current desktop default)
2. Root or per-pane `primitives.RepaintBoundary`
3. Leaf `widget.Widget` whose `Draw` calls domain `Painter` → `canvas.DrawImage`
4. PTY / data dirty → `ctx.Invalidate()` + `gogpu.RequestRedraw`

### BYO-kit (VEE-aligned)

```go
import (
    "github.com/gogpu/ui/app"
    "github.com/gogpu/ui/desktop"
    "github.com/gogpu/ui/primitives"
    "github.com/gogpu/ui/widget"
    // no github.com/gogpu/ui/core/*
)
```

### Optional: external texture layer

For leaves that already hold a GPU texture, compositor should blit via
`ExternalTextureLayer` without an extra CPU roundtrip.

## Deliverables

- `docs/sdk/custom_raster.md`
- `examples/terminal_compositor/` (minimal leaf + boundary)
- geckty as reference consumer in VEE catalog

## geckty status

Phase 1 (opt-in `GECKTY_UI_COMPOSITOR=1`) uses this pattern with a single root
`RepaintBoundary` around `compositorFrame`. Later phases split tab bar and panes
into separate boundaries.
