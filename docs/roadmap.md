# Roadmap — MVP polish

Active UX backlog on top of the current MVP. Goal: **quiet chrome** (Apple
Terminal–like), Kitty-level capability without noisy indicators.

Status legend: **Done** · **Bug** · **Todo** · **Debt** (see [tech-debt.md](tech-debt.md)).

## Track A — URLs & hints

| ID | Item | Status | Notes |
|----|------|--------|-------|
| A1 | URL hover highlight before click | Todo | OSC 8 underline exists; plain URLs need soft hover |
| A2 | `open_url_hints` chord (`Cmd/Ctrl+Shift+E`) | Bug | Overlay exists; stabilize chord / keyboard capture |

## Track B — Splits as optional feature

| ID | Item | Status | Notes |
|----|------|--------|-------|
| B1 | Extract multiplex behind a feature flag | Todo | Core stays single-pane; splits + bindings opt-in |

## Track C — Keyboard / feedback

| ID | Item | Status | Notes |
|----|------|--------|-------|
| C1 | Font zoom with main-row `+` (`Shift+=`) | Bug | Alias bindings for `increase_font_size` |
| C2 | Visual bell on BEL | Bug | Flash path exists; harden end-to-end |

## Track D — Chrome

| ID | Item | Status | Notes |
|----|------|--------|-------|
| D1 | New-tab plus affordance | Done | Glass rim + quiet hover |
| D2 | Command border / tab dot | Done | Defaults off (`command_*_enabled = false`) |

## Track E — Debt & coverage

| ID | Item | Status |
|----|------|--------|
| E1 | Deferred Kitty items in tech-debt.md | Debt |
| E2 | Raise coverage (≥85% overall; ui/app + termview ≥80%) | Todo |
| E3 | Triage geckty-owned TODOs vs vendored emu | Debt |

## Suggested order

1. C1 font zoom aliases  
2. C2 visual bell  
3. A2 hints chord  
4. A1 URL hover  
5. B1 splits feature extract  
6. E2 / E3 coverage & TODO triage  
7. E1 tech-debt items (optional / separate)

## Out of scope (unless pulled in)

- Full Kitty kitten / remote-control clone  
- WASM key hooks before `internal/plugin` API grows  
- Ligatures / multi-window — only via [tech-debt.md](tech-debt.md)

## Related code

- Defaults: `internal/config/defaults.go`
- Hints / zoom / bell / paint: `internal/ui/app/`
- Splits: `internal/session/layout.go`
- Plugins seam: `internal/plugin/`
