# Contributing to geckty

Thank you for your interest in contributing to geckty!

---

## Requirements

- **Go 1.26+**
- **golangci-lint** for code quality checks
- Linux: X11/Wayland/Vulkan headers (`pkg-config`, `libx11-dev`, `libxkbcommon-dev`, `libwayland-dev`, `libgles2-mesa-dev`, `libegl1-mesa-dev`, `libvulkan-dev`) — gio's GPU backend needs cgo
- Windows: ConPTY (Windows 10 1809+ / Windows 11) for the shell session

---

## Quick Start

```bash
# Clone the repository
git clone https://github.com/geckty/geckty
cd geckty

# Build
go build ./...

# Run tests
go test -race ./...

# Run linter
golangci-lint run --timeout=5m
```

---

## Development Workflow

### 1. Fork & Clone

```bash
git clone https://github.com/YOUR_USERNAME/geckty
cd geckty
git remote add upstream https://github.com/geckty/geckty
```

### 2. Create Feature Branch

```bash
git checkout -b feat/your-feature
# or
git checkout -b fix/issue-number-description
```

### 3. Make Changes

- Follow code style guidelines below
- Add tests for new functionality
- Update documentation if needed

### 4. Validate Before Commit

```bash
# Format code
go fmt ./...

# Run pre-release checks
bash scripts/pre-release-check.sh --quick
```

### 5. Create Pull Request

**All contributions must go through Pull Requests:**

```bash
git add .
git commit -m "feat(vt): description"
git push origin feat/your-feature
```

Then open a PR on GitHub: `https://github.com/geckty/geckty/compare`

---

## Pull Request Guidelines

### PR Requirements

- [ ] All tests pass (`go test -race ./...`)
- [ ] Linter passes (`golangci-lint run`)
- [ ] Code is formatted (`go fmt ./...`)
- [ ] Documentation updated (if applicable)

### PR Title Format

```
feat(vt): add scrollback search
fix(pty): resolve resize race on Windows ConPTY
docs: update README with usage examples
test(session): add fake-PTY lifecycle coverage
chore(ci): add linter step to github actions
refactor(protocol): simplify OSC dispatch
```

### PR Description Template

```markdown
## Summary
Brief description of changes.

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Related Issues
Closes #123
```

---

## Code Style

### Go Conventions

- Use `gofmt` for formatting (tabs, not spaces)
- Follow [Effective Go](https://go.dev/doc/effective_go)
- Use pointer receivers for structs with mutexes

### Naming

| Type | Convention | Example |
|------|------------|---------|
| Exported | PascalCase | `NewSession`, `EncodeText` |
| Unexported | camelCase | `resolveShell`, `peekNamedPipe` |
| Acronyms | Uppercase | `PTY`, `ConPTY`, `OSC52` |
| Constants | PascalCase | `DefaultCols`, `DefaultRows` |

### Error Handling

```go
// Always check errors
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}

// Or explicitly ignore
_ = file.Close()
```

---

## Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description

[optional body]

[optional footer]
```

### Types

| Type | Description |
|------|-------------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation |
| `test` | Tests |
| `refactor` | Code refactoring |
| `perf` | Performance |
| `ci` | CI/CD changes |
| `chore` | Maintenance |

### Scopes

| Scope | Description |
|-------|-------------|
| `term` | Terminal emulation core |
| `vt` | VT/ANSI state machine wrapper (internal/vt) |
| `pty` | PTY / ConPTY process management |
| `gio` | gio UI integration |
| `tabs` | Tab/session management |
| `config` | Configuration |
| `wasm` | WASM plugin runtime |
| `plugin` | Plugin manifest / host API / example plugins |
| `protocol` | OSC52, bracketed paste, focus events, mouse reporting |
| `gfx` | Kitty graphics protocol |
| `deps` | Dependencies |

---

## Project Structure

```
geckty/
├── cmd/geckty/              # Application entry point
├── internal/
│   ├── pty/                 # PTY (POSIX) / ConPTY (Windows) process management
│   ├── vt/                  # VT/ANSI state machine wrapper (cy/pkg/emu)
│   ├── session/              # PTY + VT state bundled per tab, UI-agnostic
│   ├── protocol/             # OSC52, Kitty keyboard/graphics, paste, focus, mouse
│   ├── ui/                   # gio window, event loop, grid rendering
│   ├── config/                # TOML configuration
│   └── plugin/                # WASM plugin host
└── scripts/                  # Build/release scripts
```

---

## Testing

### Run All Tests

```bash
go test -race ./...
```

### Run Specific Package

```bash
go test -v ./internal/vt/...
```

### Pre-Release Validation

```bash
bash scripts/pre-release-check.sh
```

---

## Areas Where We Need Help

- **Platform Testing** — Run and verify behaviour on Linux (X11, Wayland), macOS, and Windows.
- **Terminal Emulation Protocols** — Kitty keyboard/graphics protocol encoding, OSC 52, extended OSC/DCS support.
- **Performance** — Glyph atlas caching, incremental (dirty-region) redraw.
- **Documentation** — Configuration guides and API descriptions for core packages.
- **Accessibility** — Screen reader support, high-contrast themes.

---

## Questions?

- Open a [GitHub Issue](https://github.com/geckty/geckty/issues)
- Check existing [Discussions](https://github.com/geckty/geckty/discussions)

---

*Thank you for contributing to geckty!*
