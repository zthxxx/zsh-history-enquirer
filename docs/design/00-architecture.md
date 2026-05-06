# design/00-architecture — Go implementation map

> **Design layer** — maps spec items to concrete Go packages, types, and
> external dependencies. Read alongside the matching `spec/` doc.

## High-level shape

```
cmd/zsh-history-enquirer/                 # main package, fx app
└─ main.go                                # version fast-path + fx.New

internal/
├─ app/         # composes everything else into a runnable graph
│   ├─ module.go         # fx.Module + invokeRun + named writer types
│   ├─ config.go         # NewConfig (flag parsing) + VersionLine
│   ├─ run.go            # Run() entrypoint + PrintResult/HandleError
│   ├─ init.go           # geometry probe, cursor fallback, computeInitCol
│   ├─ loop.go           # event loop + throttle + trailing flush
│   └─ debug.go          # ZHE_DEBUG diagnostic logger
├─ history/     # spec/20 — load, reverse, dedupe, unescape
│   ├─ loader.go         # Loader interface, zshLoader, FixtureLoader
│   └─ transform.go      # ReverseDedupeUnescape (pure)
├─ search/      # spec/30
│   ├─ tokens.go         # Tokenize
│   └─ filter.go         # AndFilter (case-insensitive AND substring)
├─ tty/         # spec/10 §interactive_output, spec/40 §inline placement
│   ├─ tty.go            # TTY struct, Open/NewFromFile/NewDevTTY
│   ├─ cursor.go         # DSR probe via unix.Poll
│   ├─ raw.go            # EnterRaw/LeaveRaw termios mutation
│   └─ termios_{linux,darwin}.go  # GET/SET termios req constants
├─ ansi/        # CSI primitives, kept tiny
│   └─ ansi.go
├─ keys/        # spec/50, spec/60
│   ├─ reader.go         # poll-based byte → Event stream
│   ├─ parser.go         # FSM (Normal/Esc/CSI/Paste states)
│   └─ events.go         # Event types (Rune, Key, Paste, Resize)
└─ ui/          # spec/40 + spec/50 — pure FSM + renderer
    ├─ model.go          # state struct + rotateUp/rotateDown
    ├─ update.go         # event dispatch + scroll/page/end logic
    ├─ render.go         # Frame builder + token highlight
    ├─ wrap.go           # WrappedRowCount (rune-count estimate)
    └─ throttle.go       # leading-edge throttle (72 ms)

pkg/                                       # public-ish reusable bits
└─ version/version.go                      # injected at build via -ldflags
```

Every package above has a `*_test.go` peer; `internal/keys`, `internal/ui`,
`internal/history`, and `internal/search` additionally have property-
based tests under `pgregory.net/rapid`. `internal/keys` also has a
Go-native fuzz test against `Parser.Feed`.

There is no `internal/widget/` package; the plugin file ships as a
standalone `.zsh` source under `plugin/` and is not compiled in.

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

## Why a hand-rolled FSM (not bubbletea)?

Considered alternatives:

- **gocui / tcell**: full-screen TUI focus. Reject — they assume the
  alternate screen and would break inline rendering.
- **bubbletea**: Elm-style update loop. Tried first; surfaced two
  concrete frictions:
  1. Bubbletea's built-in key parser splits bracketed-paste payloads
     across keystrokes; we need the payload as one event.
  2. The picker throttles renders, not events. Bubbletea's render
     lifecycle is keyed to the message dispatch, which doesn't match
     the throttle-the-output, keep-mutating-the-state pattern we want
     for paste storms.
- **Hand-rolled FSM**: ~250 lines across `internal/ui/{model,update,
  render}.go`. Pure functions, no goroutines, no I/O. Each event type
  is a switch case in `Update`; `Render` returns a `Frame` struct
  without writing anywhere.

**Decision**: hand-rolled FSM. The hand-rolled code is the same shape
as a bubbletea Model (Update + View) without the framework, so the
testability is identical (or better — every render produces a Frame
that can be inspected without intercepting bytes from a pseudo-tty).
The escape emission lives in `internal/ansi/` so future contributors
can swap the renderer without touching the FSM.

## External dependencies (kept minimal)

Five direct Go-module dependencies — verified by `go mod graph`:

- `go.uber.org/fx` — DI
- `golang.org/x/sys/unix` — termios + ioctls (pure Go syscall layer,
  no CGO)
- `github.com/creack/pty` — pty pair for tty unit tests (test-only)
- `pgregory.net/rapid` — property-based tests
- `github.com/stretchr/testify` — assert/require

There is **no** bubbletea, lipgloss, or termenv. The earlier draft of
this doc claimed bubbletea was used; it never was. SGR escapes are
emitted directly from `internal/ui/render.go`, the wire bytes for
the four colours we use (bold cyan for highlight, plain reset) are
constants in the same file.

Standard library is preferred everywhere it's enough. Every
dependency above must compile with `CGO_ENABLED=0` so the resulting
binary is statically linked and runs on glibc, musl, and OpenWrt
without recompiling.
