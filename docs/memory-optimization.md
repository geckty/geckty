# Memory optimization plan: geckty vs Kitty

**Status:** draft  
**Baseline:** macOS, idle window ~1000×650, 1 tab, default config  
**Reference:** Kitty ~132 MB · geckty ~450 MB · Terminal.app ~270 MB

## 1. Executive summary

The **~3× gap** is architectural, not a single leak:

| Layer | Kitty (approx.) | geckty today | Est. share |
|-------|-----------------|--------------|------------|
| GPU runtime | Metal/OpenGL, own atlas | wgpu + gogpu + (opt.) gogpu/ui | **80–120 MB** |
| Scrollback | text store, compression | `[]emu.Line` with full `Cell` + `Hyperlink` | **50–100 MB/tab** |
| Frame buffer | partial damage, GPU atlas | full CPU RGBA + GPU texture duplicate | **15–25 MB** @2x |
| Fonts | CoreText, no TTF byte cache | platform scan + `fontCandidatesCache` | **15–30 MB** |
| Glyph cache | shared GPU atlas | up to 4× CPU atlas × 4096 glyphs | **5–20 MB** |
| Alt-screen | swap without duplicate history | `swapScreen()` keeps both buffers | **0–80 MB** after vim/htop |

**Realistic target after Phase A–C:** **~200–280 MB** idle (1 tab).  
**Kitty parity (~130 MB)** likely needs a native text path (CoreText/Metal), not compositor migration alone.

## 2. Measurement

### 2.1 Local baseline

```bash
/usr/bin/time -l ./geckty 2>&1 | rg "maximum resident"
yes | head -n 10000 | ./geckty -e cat   # full scrollback
```

Record: `GOOS`, Retina scale, `scrollback.lines`, tab count, `GECKTY_UI_COMPOSITOR`.

### 2.2 Upstream (gogpu)

After RFC `Context.GPUStats()` — track `TextureBytesAllocated` separately from Go heap.

### 2.3 Go heap

```bash
GODEBUG=gctrace=1 ./geckty
```

## 3. geckty levers

### 3.1 Scrollback

Default `scrollback.lines = 10000` (`internal/config/defaults.go`). Each `emu.Cell` carries a `Hyperlink string`.

| # | Change | Files | Savings |
|---|--------|-------|---------|
| A1 | Lower default to **2000–5000** | `defaults.go`, `config.example.toml` | 20–40 MB full buffer |
| A2 | Intern hyperlinks (index, not per-cell string) | `emu/`, `session/url.go` | 10–30 MB URL-heavy |
| A3 | Compact history (`[]rune` + attrs, opt-in) | new store | ~40–60% scrollback |
| A4 | Clear **alt history** on alt-screen exit | `emu/state.go` | up to 50–80 MB post-vim |

### 3.2 Fonts

When `font.family = ""`, code still loads Menlo/Helvetica into `fontCandidatesCache` (`internal/ui/termview/font.go`).

| # | Change | Savings |
|---|--------|---------|
| B1 | `family == ""` → embedded only | 5–15 MB |
| B2 | Drop candidate bytes after face open | 5–10 MB |
| B3 | Lazy Bold/Italic load | 2–5 MB |

### 3.3 Frame + GPU

Monolithic `s.frame` + `uploadAndPresent` full-window upload (`internal/ui/app/app.go`).

| # | Change | Savings |
|---|--------|---------|
| C1 | `Texture.UpdateRegion` (upstream) | less staging churn |
| C2 | Compositor Phase 3 per-pane boundaries | 10–20 MB @2x |
| C3 | `ExternalTextureLayer` — skip CPU roundtrip | −duplicate buffer |

### 3.4 Glyph atlas

| # | Change | Savings |
|---|--------|---------|
| D1 | LRU cap 512–1024 (not 4096) | 5–15 MB |
| D2 | Shared GPU atlas | less Go heap |
| D3 | One emboldened atlas vs separate Bold face | 2–5 MB |

## 4. gogpu upstream

See `docs/upstream/gogpu-texture-subrect-rfc.md`, [VEE #468](https://github.com/orgs/gogpu/discussions/468).

| Task | Impact |
|------|--------|
| `Texture.UpdateRegion` + staging pool | partial upload, less alloc |
| `GPUStats()` | CI observability |
| Lazy adapter/device init | lower idle RSS |
| macOS CoreText → atlas (research) | font path parity |

## 5. gogpu/ui upstream

See `docs/rendering-migration.md`, `docs/upstream/gogpu-ui-custom-raster-rfc.md`.

| Phase | Memory benefit |
|-------|----------------|
| Phase 2 tab boundary | tab strip decoupled from terminal buffer |
| Phase 3 per-pane boundary | smaller textures vs one giant frame |
| Phase 5 remove legacy `s.tex` | single present path |
| ExternalTextureLayer | no `frameRGBA` copy |

## 6. Roadmap

### Phase A — Quick wins (geckty-only, ~1–2 weeks)

- [ ] B1, B2 — font path when `family=""`
- [ ] A4 — alt-history cleanup on alt-screen exit
- [ ] A1 — default scrollback review + docs
- [ ] D1 — atlas LRU 1024
- [ ] Baseline measurement script

**Expected:** 450 → **~320–360 MB** idle (1 tab, empty scrollback)

### Phase B — Compositor split (~2–4 weeks)

- [ ] Phase 2–3 from `rendering-migration.md`
- [ ] Drop `frameRGBA` copy where possible

**Expected:** **~280–340 MB** idle

### Phase C — Upstream gogpu (parallel)

- [ ] Subrect upload RFC → implementation
- [ ] Staging pool + GPUStats in CI

### Phase D — Structural (~1–2 months)

- [ ] A2 hyperlink intern
- [ ] A3 compact scrollback opt-in
- [ ] GPU shared glyph atlas
- [ ] macOS CoreText spike

**Expected:** **~200–280 MB**

## 7. User tuning (no code)

```toml
[scrollback]
lines = 2000

[font]
bold = false
italic = false
```

## 8. CI gates (proposal)

| Gate | Threshold |
|------|-----------|
| Linux Xvfb RSS idle 5s | < 350 MB (post Phase A) |
| macOS manual weekly | trend only |
| Patch coverage | unchanged |

## 9. Out of scope

- Full Kitty text-store rewrite (A3 epic)
- Removing wgpu (cross-platform tradeoff)
- Chasing 132 MB without native text/GPU path

## 10. Top-5 backlog

1. Font embedded-only when `family=""`
2. Alt-history cleanup
3. Default scrollback + docs
4. gogpu `UpdateRegion`
5. Compositor Phase 3 (per-pane)

## Related

- [rendering-migration.md](rendering-migration.md)
- [upstream/gogpu-texture-subrect-rfc.md](upstream/gogpu-texture-subrect-rfc.md)
- [upstream/gogpu-ui-custom-raster-rfc.md](upstream/gogpu-ui-custom-raster-rfc.md)
