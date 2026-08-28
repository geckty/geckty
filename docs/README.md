# Documentation

geckty is a GPU-accelerated GUI terminal emulator (Go + [gogpu](https://github.com/gogpu/gogpu)).
Status: early MVP — usable daily-driver features land first; Kitty kitten parity is not a goal.

| Doc | Audience | What it covers |
|-----|----------|----------------|
| [architecture.md](architecture.md) | Contributors | Packages, dependency rules, render path, themes |
| [rendering-migration.md](rendering-migration.md) | Contributors | gogpu/ui compositor migration (phases, RFCs) |
| [memory-optimization.md](memory-optimization.md) | Contributors | RSS budget vs Kitty, geckty + gogpu levers |
| [configuration.md](configuration.md) | Users | Config path, fonts, colors, keybindings, themes |
| [remote-control.md](remote-control.md) | Users | `GECKTY_SOCKET` / `geckty @` |
| [plugins.md](plugins.md) | Plugin authors | WASM plugins (wazero / wasip1) |
| [roadmap.md](roadmap.md) | Contributors | UX polish backlog |
| [tech-debt.md](tech-debt.md) | Contributors | Deferred Kitty-parity items |
| [engineering.md](engineering.md) | Contributors | Code-quality backlog (DRY / godoc) |

Also see:

- [`README.md`](../README.md) — overview and quick build
- [`CONTRIBUTING.md`](../CONTRIBUTING.md) — workflow, style, PR rules
- [`config.example.toml`](../config.example.toml) — annotated defaults
- [`themes/glass.toml`](../themes/glass.toml) — editable copy of the shipped theme
