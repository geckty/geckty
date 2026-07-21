# geckty

A GUI terminal emulator written in Go, built on [gogpu](https://github.com/gogpu/gogpu)
and [cy/pkg/emu](https://github.com/cfoust/cy) — in the spirit of kitty, Rio,
and Alacritty.

**Status:** early MVP under active development. See
[CONTRIBUTING.md](CONTRIBUTING.md) for the project structure and how to
build from source.

## Features (target)

- OSC 52 clipboard, bracketed paste, focus events
- Kitty keyboard protocol, Kitty graphics protocol
- Custom window chrome (titlebar, tabs) configurable via TOML
- WASM plugin system
- macOS, Windows (ConPTY), and Linux (X11 + Wayland)

## Building

```bash
go build ./cmd/geckty
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development workflow.

## License

[MIT](LICENSE)
