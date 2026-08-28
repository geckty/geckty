# RFC: Subrect texture upload and GPU memory stats

**Repo:** `gogpu/gogpu`  
**Status:** draft (geckty upstream proposal)  
**Consumer:** [geckty](https://github.com/geckty/geckty)  
**Related:** [VEE #468](https://github.com/orgs/gogpu/discussions/468)

## Problem

CPU-raster apps (terminals, custom canvases) upload a full-window RGBA buffer
every dirty frame. geckty tracks dirty terminal rows but cannot express partial
uploads to gogpu.

## Proposal

### `Texture.UpdateRegion`

```go
type Texture interface {
    UpdateData(rgba []byte) error
    UpdateRegion(x, y, w, h int, rgba []byte) error
    // ...
}
```

### `Context.GPUStats()` (debug)

```go
type GPUStats struct {
    TextureBytesAllocated int64
    LastUploadBytes       int64
    LastUploadRegion      image.Rectangle
}
```

### Staging buffer pool

Reuse staging allocations across `UpdateData` / `UpdateRegion` calls.

## Acceptance

- Visual correctness on macOS / Linux / Windows
- geckty: dirty 3 rows → upload &lt; 10% of full frame bytes
- Documented in changelog + `examples/terminal_upload`
