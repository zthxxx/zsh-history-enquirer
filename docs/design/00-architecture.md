# design/00-architecture — Go implementation map

> **Design layer** — maps spec items to concrete Go packages, types, and
> external dependencies. Read alongside the matching `spec/` doc.

## High-level shape

```
cmd/zsh-history-enquirer/                 # main package, fx app
└─ main.go

internal/
├─ app/         # fx module wiring everything together
│   └─ module.go
├─ history/     # spec/20 — load, reverse, dedupe, unescape
│   ├─ loader.go        # Loader interface + zshLoader (fc -R; fc -ln 1)
│   ├─ fixture.go       # FixtureLoader for unit tests
│   ├─ transform.go     # ReverseDedupeUnescape (pure)
│   └─ *_test.go        # property-based via pgregory.net/rapid
├─ search/      # spec/30
│   ├─ tokens.go        # Tokenize
│   ├─ filter.go        # AndFilter
│   └─ *_test.go        # property-based + table
├─ tty/         # spec/10 §interactive_output, spec/40 §inline placement
│   ├─ tty.go           # /dev/tty open helpers
│   ├─ cursor.go        # DSR query + parse
│   ├─ raw.go           # raw mode RAII guard
│   └─ *_test.go        # use creack/pty for unit tests
├─ ansi/        # cursor/erase escape primitives, kept tiny
│   └─ ansi.go
├─ keys/        # spec/50, spec/60
│   ├─ reader.go        # raw-byte → Event stream
│   ├─ events.go        # Event types (Rune, Key, Paste, Resize)
│   └─ *_test.go
├─ ui/          # spec/40 + spec/50 driver
│   ├─ model.go         # state struct (input, visible, idx, limit, …)
│   ├─ update.go        # state transitions for events
│   ├─ render.go        # produces a frame string + cursor target
│   ├─ wrap.go          # wrapped_row_count helper
│   ├─ throttle.go      # leading-edge throttle
│   └─ *_test.go        # golden-frame tests
└─ widget/      # plugin file constants (path resolution etc.)
    └─ widget.go

pkg/                                       # public-ish reusable bits
└─ version/version.go                      # injected at build via -ldflags
```

## Layered dependency rules

```
cmd ─→ internal/app ─→ internal/{history, ui, tty, keys}
                          │            │     │     │
                          ▼            ▼     ▼     ▼
                     internal/search   ─── internal/ansi
```

- No package may import a package "above" it in the diagram.
- No package may import `cmd/`.
- `pkg/` may be imported by anything.
- The dependency rule is enforced statically by a `task lint:layers`
  recipe (using `go-arch-lint` or a hand-rolled grep — TBD; see
  `plan/30-architecture-enforcement.md`).

## Why fx?

`go.uber.org/fx` is the project's DI framework. Justification:

- The app has one binary entrypoint but composes 5+ stateful components
  (TTY handle, history loader, key reader, throttled renderer, model).
- Tests want to swap implementations: `historyLoader` ↔ `fixtureLoader`,
  real `tty` ↔ pty, real keystreamer ↔ scripted events.
- An fx module makes the dependency graph explicit and machine-readable
  (`fx.New(... fx.Visualize)`).
- Cancellation and shutdown order are handled by fx's lifecycle hooks,
  saving us a per-component `defer` mess.

The cost is the fx runtime overhead (~5-10 ms cold start). For an
interactive widget that already pays a syscall for `fc -R` plus a DSR
round-trip (~5 ms), this is invisible.

## Why bubbletea + lipgloss + termenv?

Considered alternatives:

- **Hand-rolled** (closest to legacy): full control over every escape
  byte, no abstractions. Won by the legacy port; led to gnarly clear/
  restore logic in `historySearcher.ts`. Reject — the original
  implementation's bugs were exactly here.
- **gocui / tcell**: full-screen TUI focus. Reject — they assume the
  alternate screen and would break inline rendering.
- **bubbletea**: Elm-style update loop; supports inline rendering via
  `tea.WithOutput(os.Stderr)` + custom render mode. Trade-off is that
  inline mode is less battle-tested than full-screen mode, so the
  rendering layer needs a thin custom escape emitter on top.

**Decision**: bubbletea **for the event/update plumbing**, with a custom
inline renderer (`internal/ui/render.go`) that emits the escapes
directly. We get the testability of bubbletea's `tea.Program.Send`
event injection without inheriting full-screen assumptions.

## External dependencies (kept minimal)

- `go.uber.org/fx` — DI
- `github.com/charmbracelet/bubbletea` — event loop
- `github.com/charmbracelet/lipgloss` — style helpers (only used for
  the highlight pass; no heavyweight layout)
- `golang.org/x/sys/unix` — termios + ioctls (pure Go syscall layer,
  no CGO)
- `github.com/creack/pty` — pty pair for tty unit tests (test-only
  dependency — also pure Go on linux/darwin)
- `pgregory.net/rapid` — property-based tests
- `github.com/stretchr/testify` — assert/require

(Standard library is preferred everywhere it's enough. Every
dependency above must compile with `CGO_ENABLED=0` so the resulting
binary is statically linked and runs on glibc, musl, and OpenWrt
without recompiling.)
