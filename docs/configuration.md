# Configuration

Copy [`config.example.toml`](../config.example.toml) to:

```text
$XDG_CONFIG_HOME/geckty/config.toml
# or, if XDG_CONFIG_HOME is unset:
~/.config/geckty/config.toml
```

On first launch geckty creates a default file if missing. Any omitted field
falls back to built-in defaults (`internal/config`). Hot-reload applies to
many visual settings without restarting.

## CLI flags

| Flag | Purpose |
|------|---------|
| `--list-themes` | Print theme names (user + embedded) and exit |
| `--log-level` | `debug` / `info` / `warn` / `error` (overrides `log_level` in config) |
| `@ …` | Remote-control client — see [remote-control.md](remote-control.md) |

## Fonts

| Section | Default when `family` is empty | Role |
|---------|--------------------------------|------|
| `[font]` | Bundled **IBM Plex Mono** (`assets/fonts/mono`) | Terminal grid |
| `[ui_font]` | Bundled **PT Sans** (`assets/fonts/ui`) | Tab bar / chrome |

Set `family = "monospace"` for the platform default, or a concrete family
name looked up in system font directories. `bold` / `italic` toggle real
weight faces vs regular. Ligatures are reserved (`# ligatures = false`) —
see [tech-debt.md](tech-debt.md).

## Themes and colors

```toml
theme = "glass"   # optional; loads themes/<name>.toml then merges inline keys
```

Search order for a named theme:

1. `~/.config/geckty/themes/<name>.toml` (or next to your config file)
2. Embedded `assets/themes/<name>.toml`

Inline `[colors]`, `[ui]`, and `[ui.glass]` merge on top (Kitty/Rio style).
Unset chrome colors (`active_tab_background`, …) are derived from the
background via glass blends.

```bash
geckty --list-themes
```

Shipped builtin: **glass**. Editable example: [`themes/glass.toml`](../themes/glass.toml).

### Quiet chrome defaults

```toml
[ui]
command_border_enabled = false   # no window border while a command runs
command_dot_enabled = false      # no OSC 133 status dot on tabs
```

## Keybindings

Configured under `[[keybindings]]` (see `config.example.toml`). Defaults
include tabs, splits, scrollback, search, URL open / hints, font zoom,
clipboard, and quit. Unknown actions are rejected at load time.

## Shell, clipboard, scrollback

See annotated sections in `config.example.toml`:

- `[shell]` — program / args / cwd
- `[clipboard]` — OSC 52 read/write policy
- `[scrollback]` — history lines (default 10000)
- `[cursor]`, `[window]`, plugins path, logging

## Plugins

```toml
# plugins = ["/path/to/my-plugin"]  # each dir has plugin.toml + entry wasm
```

Empty by default. See [plugins.md](plugins.md).
