# Remote control

Minimal Kitty-style remote control over a socket. Disabled unless the
running instance has a listen address.

## Enable

Start geckty with a socket path in the environment:

```bash
# macOS / Linux — Unix domain socket
export GECKTY_SOCKET=/tmp/geckty-$USER.sock
geckty &

# Alias: GECKTY_LISTEN is accepted if GECKTY_SOCKET is unset
```

On **Windows**, the path is a TCP listen address (`host:port` or a bare
port, which binds `127.0.0.1:<port>`).

## Client

```bash
geckty @ <command> [args…]
```

The client reads `GECKTY_SOCKET` / `GECKTY_LISTEN` and sends one line; the
server replies with `OK`, `OK <payload>`, or `ERR <msg>`.

## Commands

| Command | Arguments | Effect |
|---------|-----------|--------|
| `new_tab` | — | Open a new tab |
| `close_tab` | — | Close the active tab |
| `list_tabs` | — | List tab titles |
| `get_text` | — | Screen / selection text (host-defined) |
| `send_text` | `<text…>` | Type text into the active session |

Example:

```bash
geckty @ new_tab
geckty @ send_text echo hello
geckty @ list_tabs
```

## Scope

This is intentionally small — not a full Kitty `@` / kitten clone. Extra
commands can be added in `internal/rc` when a concrete workflow needs them.
Tabs + in-window splits already cover most multi-session use without a
second OS window (see [tech-debt.md](tech-debt.md)).
