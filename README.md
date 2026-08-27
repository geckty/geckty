# geckty

A GUI terminal emulator written in Go, built on [gogpu](https://github.com/gogpu/gogpu)
— in the spirit of kitty, Rio, and Alacritty.

**Status:** early MVP under active development.  
**Go:** 1.27+

## Features (MVP / in progress)

- OSC 52 clipboard, bracketed paste, focus events
- Kitty keyboard protocol (partial), Kitty graphics protocol (basic)
- Custom window chrome (glass tab bar) via TOML themes
- Tabs, pane splits, scrollback search, URL open / hints
- WASM plugin host (statusbar)
- Remote control: `GECKTY_SOCKET` + `geckty @`
- macOS, Windows (ConPTY), Linux (X11 + Wayland)

## Building

```bash
go build ./cmd/geckty
# or
task build
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full workflow.

## Configuration

Copy [`config.example.toml`](config.example.toml) to
`~/.config/geckty/config.toml`. Set `theme = "glass"` (or another name) to
load a theme file; inline `[colors]` / `[ui]` merge on top. List themes with
`geckty --list-themes`.

## Documentation

| Doc | Topic |
|-----|-------|
| [docs/README.md](docs/README.md) | Index |
| [docs/architecture.md](docs/architecture.md) | Package layout & render path |
| [docs/configuration.md](docs/configuration.md) | Config, fonts, themes |
| [docs/remote-control.md](docs/remote-control.md) | `geckty @` |
| [docs/plugins.md](docs/plugins.md) | WASM plugins |
| [docs/roadmap.md](docs/roadmap.md) | Polish backlog |
| [docs/tech-debt.md](docs/tech-debt.md) | Deferred Kitty parity |

## License

[MIT](LICENSE)
