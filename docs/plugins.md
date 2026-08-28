# WASM plugins

Plugins run in-process via [wazero](https://github.com/tetratelabs/wazero)
(pure Go, no cgo). The host package is `internal/plugin`; the UI decides
when to call hooks (`internal/ui/app`).

## Guest toolchain

Build with the standard Go wasip1 target as a **reactor** (must stay
resident):

```bash
GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
```

`-buildmode=c-shared` exports `_initialize` (WASI Reactor). A default
wasip1 build exports `_start`, runs `main`, and exits — unusable for a
long-lived plugin.

## Manifest

Each plugin lives in its own directory with `plugin.toml`:

```toml
name = "statusbar-clock"
version = "0.1.0"
entry = "plugin.wasm"
permissions = ["log", "statusbar"]
```

Unknown permission strings fail load (deny-by-default). Current set:

| Permission | Capability |
|------------|------------|
| `log` | Host logging |
| `statusbar` | Status-bar text |

## Guest API sketch

- Export hooks with `//go:wasmexport name`
- Import host helpers with `//go:wasmimport geckty name`

See `internal/plugin` package docs and the working example:

[`plugins/examples/statusbar-clock/`](../plugins/examples/statusbar-clock/)

## Current scope

MVP host surface is statusbar (+ log). Key/event hooks and richer actions
are future work — do not assume a Kitty-kitten-sized API yet. Until then,
optional product features (e.g. splits) stay as Go feature flags rather
than WASM (see [roadmap.md](roadmap.md)).
