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

## Configuration

Copy [`config.example.toml`](config.example.toml) to
`~/.config/geckty/config.toml`. Colors are free-form keys under `[colors]`
(Kitty/Rio style). Optionally set `theme = "name"` to load
`themes/name.toml` next to the config (or under `~/.config/geckty/themes/`);
inline `[colors]` keys merge on top. See [`themes/glass.toml`](themes/glass.toml)
for a theme-file example. Tab chrome colors (`active_tab_background`, …)
are optional — unset keys keep the glass-derived look.

<picture>
  <source media="(prefers-color-scheme: dark)"
    srcset="https://api.starhistory.io/png?repos=geckty/geckty&style=dark" />
  <source media="(prefers-color-scheme: light)"
    srcset="https://api.starhistory.io/png?repos=geckty/geckty&style=professional" />
  <img src="https://api.starhistory.io/png?repos=geckty/geckty"
    width="800" alt="Star History" />
</picture>


## License

[MIT](LICENSE)
