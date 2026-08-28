# RFC: Subrect texture upload and GPU memory stats

**Repo:** `gogpu/gogpu`  
**Status:** draft (geckty upstream proposal)  
**Consumer:** [geckty](https://github.com/geckty/geckty)  
**Related:** [VEE #468](https://github.com/orgs/gogpu/discussions/468) · [memory-optimization.md](../memory-optimization.md)

## Summary

geckty CPU-rasterizes the terminal into an RGBA buffer and uploads it to a
single window texture every dirty frame. The emulator already tracks dirty
terminal rows (`vt.DirtyRows`), but gogpu only exposes full-texture
`UpdateData`. Partial uploads would cut staging churn and GPU bandwidth on
typical typing/scrolling workloads.

## Problem

| Today | Cost |
|-------|------|
| `uploadAndPresent` → `Texture.UpdateData(s.frame)` | `W × H × 4` bytes per dirty frame |
| CPU `s.frame` + GPU texture duplicate | ~2× window pixels @ scale |
| Compositor path copies `frame` → `frameRGBA` | optional 3rd copy |
| No observability | cannot gate RSS/GPU bytes in CI |

On a 2000×1300 @2× Retina window, a single keystroke that dirties 1–3 rows
still uploads ~10 MB.

## Proposal

### 1. `Texture.UpdateRegion`

```go
type Texture interface {
    // UpdateData replaces the entire texture (existing behaviour).
    UpdateData(rgba []byte) error

    // UpdateRegion uploads a sub-rectangle. rgba is exactly w*h*4 bytes,
    // row-major RGBA8. (x,y) is top-left in texture space (same origin as
    // UpdateData). Out-of-bounds returns an error.
    UpdateRegion(x, y, w, h int, rgba []byte) error

    Width() int
    Height() int
    Destroy()
}
```

**Semantics**

- Same pixel format and coordinate system as `UpdateData`.
- May be implemented as full upload on backends that lack subresource copy
  (document in backend matrix), but macOS Metal / Vulkan / D3D12 should use
  native partial updates.
- Safe to call from the render thread (same as `UpdateData` today).

### 2. Staging buffer pool

```go
type StagingPool interface {
    // Acquire returns a []byte of at least need bytes. Caller returns it via
    // Release when the upload completes (same frame).
    Acquire(need int) []byte
    Release(buf []byte)
}
```

Attach an optional pool to `Context` or `Texture` so repeated partial uploads
do not allocate `w*h*4` on every frame. geckty would acquire one staging buf
per dirty band per frame.

### 3. `Context.GPUStats()` (debug / CI)

```go
type GPUStats struct {
    TextureBytesAllocated int64
    LastUploadBytes       int64
    LastUploadRegion      image.Rectangle // zero = full texture
}

func (c *Context) GPUStats() GPUStats
```

Enables geckty CI to assert `LastUploadBytes < 0.1 * fullFrame` when only a
few terminal rows changed.

## geckty integration plan

Once upstream lands:

1. **`uploadAndPresent`** (`internal/ui/app/app.go`): when `dirtyRows` spans
   ≤25% of frame height, extract row bands from `s.frame` and call
   `UpdateRegion` per contiguous dirty run.
2. **Fallback**: if region upload fails or dirty area &gt; threshold, use
   existing full `UpdateData`.
3. **Compositor path**: same helper; drop full `frameRGBA` copy when
   `ExternalTextureLayer` is available (separate RFC).
4. **CI**: Linux Xvfb job logs `GPUStats().LastUploadBytes` on a synthetic
   single-row dirty test (optional gate).

### Helper sketch (geckty-side, blocked on upstream)

```go
func uploadFrameRegion(tex *gogpu.Texture, frame []byte, fw, fh int, dirty vt.DirtyRows) error {
    if dirty.Full || dirtyRowFraction(dirty, fh) > 0.25 {
        return tex.UpdateData(frame)
    }
    for _, band := range dirty.Bands(fh) {
        sub := extractRGBABand(frame, fw, band)
        if err := tex.UpdateRegion(0, band.Y0, fw, band.H, sub); err != nil {
            return tex.UpdateData(frame) // fallback
        }
    }
    return nil
}
```

## Acceptance criteria

| Check | Target |
|-------|--------|
| Visual correctness | macOS, Linux (X11/Wayland), Windows |
| Partial upload size | 3 dirty rows @ 80×24 grid → upload &lt; 10% of full frame |
| Full upload fallback | unchanged behaviour when `UpdateRegion` unsupported |
| Staging pool | no per-frame heap alloc on steady-state typing benchmark |
| Changelog + example | `examples/terminal_upload` in gogpu repo |

## Non-goals

- Replacing CPU raster with GPU text (Kitty-style native path — separate research)
- Multiple textures per window (compositor Phase 3 handles pane split on geckty side)

## References

- Kitty damage rendering + GPU cell atlas
- Alacritty partial display updates (grid damage)
- geckty dirty rows: `internal/vt/dirty.go`, `TakePaintDirty()` in `termview/painter.go`
