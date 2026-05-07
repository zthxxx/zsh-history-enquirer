# AGENTS.md

Guidance for AI agents (Claude Code, Cursor, others) and human
contributors working on `zsh-history-enquirer`.

> **`CLAUDE.md` is a symlink to this file.** Edit `AGENTS.md` directly.

## What this project is

A **zsh widget** bound to <kbd>Ctrl</kbd>+<kbd>R</kbd> that replaces
zsh's native `history-incremental-search-backward` with an inline,
multi-line, deduplicated, multi-word-fuzzy-matched history picker.

It is **not** a general-purpose CLI/TUI tool. The whole project
exists to make `^R` better.

## Architecture (one-screen overview)

```
zsh widget (plugin/zsh-history-enquirer.plugin.zsh)
   │ on ^R: BUFFER=$(zsh-history-enquirer "$LBUFFER")
   │
   ▼
binary entrypoint (cmd/zsh-history-enquirer/main.go)
   │ fx.New(app.Module).Start(...) → Run() runs sync inside OnStart
   │
   ▼
internal/app — Run() orchestrates:
   ├─ tty.NewDevTTY  + EnterRaw  + HideCursor + BracketedPasteOn
   ├─ goroutine: tty.Probe.Cursor(250ms timeout via unix.Poll)
   ├─ goroutine: history.Loader.Load (zsh fc -R; fc -ln 1, with
   │                                  test-only FixtureLoader fallback)
   ├─ ui.NewModel(input, choices, geom, max-limit) — pure state
   ├─ keys.NewReader(t).Events(ctx) — byte-stream → Event channel
   │     ├─ feedNormal / feedEsc / feedCSI / feedPaste state machine
   │     └─ flushTimer: 50ms ESC-alone debounce
   ├─ trailingFlush: 72ms timer; flushes after a render burst so the
   │                 last state of fast typing actually paints
   └─ event loop: ev → model.Update(ev) → render(ev triggers throttle)

internal/ui — model.go (state) + update.go (transitions) +
              render.go (Frame: Pre, Body, Post) + wrap.go (geometry)
internal/search — Tokenize + AndFilter (case-insensitive, AND-only)
internal/history — ZshLoader + FixtureLoader + ReverseDedupeUnescape
internal/tty — TTY handle, raw-mode RAII, DSR cursor probe via Poll
internal/keys — byte parser + Event types + bracketed-paste FSM
pkg/version — -ldflags-injected version stamp (--version flag)

ANSI escape emission is delegated to charmbracelet/x/ansi (shared
with bubbletea / lipgloss); cell-width counting is rivo/uniseg
(grapheme-cluster aware so decomposed accents and emoji ZWJ
families measure correctly). No bespoke wrapper packages.
```

## Distribution

Static Go binary (`CGO_ENABLED=0`) for the four platforms we support:

| Target | Notes |
| --- | --- |
| `darwin/arm64` | Apple Silicon |
| `darwin/amd64` | Intel Mac |
| `linux/arm64` | works on glibc, musl, OpenWrt |
| `linux/amd64` | works on glibc, musl, OpenWrt |

Three distribution channels:

1. **npm** — `zsh-history-enquirer` is the umbrella; per-platform
   binaries live in `@zsh-history-enquirer/<os>-<arch>` packages,
   wired via `optionalDependencies` (esbuild-style). The umbrella
   ships a `bin/cli.js` that detects platform and execs the matching
   sub-package's binary.
2. **Homebrew** — `zthxxx/homebrew-tap` `Formula/zsh-history-enquirer.rb`,
   bumped automatically by the release workflow.
3. **GitHub Releases** — raw binaries + `checksums.txt`.

Per-platform npm packages and the homebrew formula are **rendered at
release time** from templates in `npm/templates/platform/`
and `scripts/ci/bump-homebrew-tap.sh`, respectively. They are not
hand-edited.

## Common commands

```bash
task build            # build host binary (CGO disabled → static)
task build:linux      # build linux/amd64 for e2e
task build:all        # cross-compile every release target
task test:unit        # Go unit tests (no docker)
task test:js          # Node tests for the npm shim (cli.js)
task test:e2e         # docker-driven e2e for both libcs
task lint             # go + arch + md + sh + zsh + js (parse-only)
task check            # fmt + lint + all tests (incl. e2e)
task check:fast       # fmt + lint + unit only
task ci               # run the full CI workflow locally via `act`
task release:dry-run  # render npm packages without publishing
task release:smoke    # exec the rendered shim — validates the publish path
```

Run a single Go test: `go test ./internal/ui -run TestRender_PointerOnFocused`.

## Conventions

- **Static linkage is mandatory.** Every Linux build runs through a
  `file ... | grep -v 'dynamically linked'` check in `build:all` and
  in CI. Reach for any C library, and the build fails by design.
- **Spec → design → plan.** New behaviour starts in `docs/spec/`
  (user-facing wording), then gets a counterpart in `docs/design/`
  (Go implementation map). `docs/plan/10-tasks.md` is the only
  source of truth for "what's left."
- **No automatic shell-config edits.** The legacy v1.x port edited
  `~/.zshrc` and oh-my-zsh's plugin list. The Go port refuses to.
  Users wire the plugin in themselves; the `npm install` hint is the
  only thing the package emits.
- **Cancel preserves input.** Esc, Ctrl+C, and Enter-on-no-match all
  emit the user's typed input as the picker's stdout. The widget
  sets `BUFFER=$(...)` to that string. Breaking this invariant means
  losing typed work, so it has e2e coverage in
  `e2e/scenarios/03-cancel-preserves-input.exp`.
- **Tests are layered.** `internal/**/*_test.go` use property-based
  generation where the behaviour is amenable (history transform,
  search filter, wrap math, parser FSM) plus a Go-native fuzz target
  on `Parser.Feed`. The e2e layer lives only in docker
  (`e2e/scenarios/*.exp`, run by `e2e/{debian,alpine}/Dockerfile`).
  The legacy port shipped a `tests/zsh-widget.test.zsh` that ran
  against the developer's host shell — that path mutated
  `~/.zsh_history` and broke terminal state, and is **not** to be
  recreated. New e2e scenarios go in `e2e/scenarios/`.
- **All comments and docs in English.** The single exception is
  `README.zh-CN.md`.
- **fx.NopLogger is mandatory in main.** The widget contract requires
  stderr to stay quiet during normal operation.
- **`os.Exit(0)` on every termination path.** A non-zero exit aborts
  `BUFFER=$(...)` and loses the user's typed input — see
  `docs/spec/10-widget-contract.md`.

## Gotchas you'll inevitably hit

- **`os.File.SetReadDeadline` on /dev/tty is unreliable** in Docker's
  pty. The cursor probe uses `unix.Poll` directly. Don't replace it.
- **The cursor probe leftover must replay through the keystream.**
  When the probe times out, it returns whatever bytes it consumed in
  `TimeoutError.Leftover`. The reader's `Prefeed` injects them into
  the parser before the read goroutine starts, so user input typed
  during a slow DSR window is not silently dropped.
- **The throttle is leading-edge with a trailing flush.** A burst of
  events without the trailing flush would drop the last frame —
  the picker would show "you typed 6 chars" instead of "you typed
  7 chars". `internal/app/loop.go:trailingFlush` is what stops that
  (lives in `loop.go`, not `run.go` — the Run() body was split during
  the architect refactor; see [docs/plan/20-followups.md](./docs/plan/20-followups.md)).
- **fx providers for `os.Stdout` and `os.Stderr` need distinct types.**
  Two anonymous `io.Writer` providers fail dependency resolution;
  `Stdout` and `StderrWriter` named types in `internal/app/module.go`
  exist purely to disambiguate.
- **Do not `EraseDisplayDown` or do anything full-screen.** Inline
  rendering is the entire UX claim. The renderer only ever touches
  rows from `InitRow` down to `InitRow + state.size`, and only
  columns from `InitCol` rightward.
- **The "preserve `$LBUFFER`" invariant has four uncovered paths
  the layered code already handles — don't break them.** When you
  edit any of these, run `task release:smoke` to verify:
  1. `BUFFER=$(...)` resolves to user-typed text on **submit**
     (focused entry) and on **cancel** (typed input echoed).
  2. **Hard early-error in Run()** — `t.EnterRaw()` or
     `readGeometry()` fails. `preserveOnError` in
     `internal/app/module.go` synthesizes a `RunResult` from
     `cfg.Input` so `PrintResult` still fires.
  3. **fx-provider startup failure** — `/dev/tty` unopenable in
     a headless container. `recoverStartFailure` in
     `cmd/zsh-history-enquirer/main.go` reconstructs `cfg.Input`
     via `app.NewConfig` and writes it to stdout when `a.Start()`
     errors.
  4. **External `kill -TERM <pid>`** — `invokeRun` wraps
     `context.Background()` with
     `signal.NotifyContext(SIGINT, SIGTERM, SIGHUP)` so the event
     loop's `<-ctx.Done()` case fires, the cancel-preserves-input
     path runs, and fx OnStop hooks restore termios.
- **The plugin passes `--` before `$LBUFFER`** so the binary's
  doc fast-paths (`--version`, `--help`, `-h`) cannot be triggered
  by user input that happens to look like a flag. The npm shim
  also strips a leading `--` from its missing-binary-fallback
  echo. If you change either side, change both — and update
  `task release:smoke` step 3/4 accordingly.
- **Unrecognized SS3 / aborted CSI sequences are silently consumed,
  not bounced as `KeyEsc`.** F1-F4 emit `\eO[PQRS]` and a flaky-link
  split arrow emits `\e[` paused mid-sequence. Earlier code emitted
  the buffered prelude as `KeyEsc + body bytes` so the leading
  `KeyEsc` cancelled the picker on every fat-fingered F-key or
  every >50ms-paused arrow. Both arms (`feedSS3` default,
  `feedCSI` default, and `FlushEsc` for `stateSS3` and `stateCSI`)
  now reset state and emit nothing — the picker stays open. Only
  the `stateEsc` flush still emits `KeyEsc` (preserving the user's
  intent to cancel via a bare Esc). See `docs/design/40-keys.md`
  for the full table.

## Triggers for `/ship`

Don't auto-run `/ship` on conversational replies; the user invokes it
explicitly. `/ship-multi` is allowed when a change touches three or
more of: `internal/{ui,history,search,tty,keys,app}`, `e2e/`,
`npm/`, `.github/workflows/`.
