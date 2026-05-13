# Scenario manifest

Cross-cut inventory of every `.exp` scenario the implementer must
migrate. Built by reading every `e2e/scenarios/*.exp` end-to-end.

## Terminal geometry distribution

| Geometry      | Scenarios            |
| ------------- | -------------------- |
| 24 × 80 (default) | 01–15, 17–23         |
| 24 × 40       | 16, 24               |
| 10 × 80       | 25                   |

Implication for harness: `pty.SetWinsize` must be called BEFORE the
binary spawns its picker, and the picker must read it via TIOCGWINSZ
at startup. Default 24×80 fallback already exists in the binary
(see `internal/app/run.go` t.Size() zero handling).

## Key catalog

All raw byte sequences the existing scenarios send. The Go harness
should expose a typed `Key` constant for each.

| Symbol | Bytes              | Used by |
| ------ | ------------------ | ------- |
| CtrlR (open picker) | `\x12` | every scenario |
| CtrlU (kill-line)   | `\x15` | 01, 03, 11, 17, 19, 22 |
| CtrlW (kill-word)   | `\x17` | 19 |
| Enter               | `\r`   | every scenario |
| Esc                 | `\x1b` | 03, 11, 14, 16, 22 |
| Backspace           | `\x7f` | 17, 19, 20, 21 |
| Alt-Backspace       | `\x1b\x7f` | 20 |
| Arrow Down          | `\x1b[B` | 02, 06, 11, 25 |
| Home                | `\x1b[H` | 07 (and end-bias variant) |
| PageUp              | `\x1b[5~` | 06 |
| PageDown            | `\x1b[6~` | 06 |
| F1 (SS3)            | `\x1bOP` | 23 |
| F2 (SS3)            | `\x1bOQ` | 23 |
| Bracketed paste open | `\x1b[200~` | 05, 22 |
| Bracketed paste close | `\x1b[201~` | 05, 22 |
| Raw Ctrl-C byte inside paste | `\x03` | 22 |

Plus printable ASCII / UTF-8 typing (`café`, `git`, `command`, etc.).

## History fixture inventory

| Scenario | Fixture |
| -------- | ------- |
| 01–12, 15, 17–20, 23 | default seed in `e2e/run.sh write_seed_history` |
| 13 (unicode entries) | `exec sh -c` rewrites HISTFILE inline with multi-byte entries |
| 14 (long-line wrap) | `exec sh -c` inserts a >100-char entry |
| 16 | default seed, but combined with 24×40 geometry |
| 21 (input wrap edit) | `exec sh -c` inserts a long entry that wraps the input row |
| 24 (multiline narrow wrap submit) | dedicated `/tmp/multiline-narrow-history` with multi-line `printf` entry |
| 25 (multiline scroll-down no stick) | dedicated `/tmp/multiline-stick-history` with 6 singles + 1 multi-line |

Implication: the new harness must support both
(a) a default seed shared across most scenarios, and
(b) per-scenario override via a dedicated `testdata/seeds/<name>.histfile`
mounted read-only into the container.

## Special scenarios warranting extra design attention

- **05, 22 — bracketed paste**: the parser FSM must round-trip through
  the picker; assertions check filter content rather than focused entry.
- **15 — vi-keymap**: types `bindkey -v\r` first; tests that ^R remains
  bound after switching to vi mode. Needs picker re-open after keymap
  change. The 5s `fx.StartTimeout` change in followups is relevant.
- **18 — flag-shaped lbuffer**: types `git log --pretty=fuller` style
  text that contains `--` and `--version`. Verifies the binary's
  doc-fastpath does NOT trigger on user input. Reproduces the
  `plugin.zsh` `-- $LBUFFER` contract.
- **23 — F-key no-cancel**: sends `\x1bOP \x1bOQ` and asserts the picker
  stays open (was previously cancelling on incomplete SS3 prelude).
- **24, 25 — multi-line interactions**: need precise `WaitQuiescent`
  semantics because rendering involves dynamic-limit reflow.

## Cross-cutting assertions

Most scenarios assert on the form `expect -- "echo ok"` (text presence).
The Go harness should expose the **cell-grid level** equivalents:

- `grid.RowContains(rowIdx, substring)` — direct port.
- `grid.AnyRowContains(substring)` — substring anywhere on screen.
- `grid.Cell(row, col).Rune == '▎'` — focus indicator presence.
- `grid.FocusedRow()` — the row whose first non-whitespace cell is `▎`.
- `grid.Golden(t, name)` — full-grid snapshot with `-update` regenerate.

Color/SGR assertions (token highlight: bold cyan) are needed for
scenarios that emphasize "search token is colored":
- `grid.Cell(row, col).FG == color.Cyan && Bold`
- Use sparingly — most scenarios don't care about color.

## Post-migration assertion coverage worth ADDING (out of scope for v1)

Items the expect-layer cannot easily assert today but the new harness
unlocks. Not blocking the migration but worth noting:

- Cursor position after wrapped-input edit (scenario 21 has to settle
  for "match revealed" rather than "cursor at col X").
- SGR sequence ordering after `NO_COLOR=1` (currently no e2e scenario).
- Inline rendering boundary (renderer never touches rows above InitRow);
  expect cannot verify this — the cell grid can.

## Reading order for the implementer

1. `e2e/run.sh` (current container entrypoint logic)
2. `e2e/debian/Dockerfile` + `e2e/alpine/Dockerfile`
3. `plugin/zsh-history-enquirer.plugin.zsh` (^R binding)
4. `internal/app/run.go` (binary's contract)
5. This manifest
6. `e2e-v2/DESIGN.md` (architect output — read AFTER architect completes)
