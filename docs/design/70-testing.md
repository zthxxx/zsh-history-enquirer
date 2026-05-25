# design/70-testing — what runs where

## Test layers

| Layer | Where | What |
| --- | --- | --- |
| **unit** | `internal/**/*_test.go` | Pure Go. No zsh, no docker. Property-based tests via `pgregory.net/rapid` plus targeted table tests. Runs with `task test:unit`. |
| **integration** | same package | Components that need a pty (via `creack/pty`) — keys reader, tty cursor probe. Skipped on Windows. Still no docker. |
| **e2e** | `e2e/` (separate Go module) | Real binary inside docker, real zsh, real pty, driven by a Go-native harness using `creack/pty` + `hinshun/vt10x`. Two libcs (glibc / musl). **Never** runs against the user's machine. |

## Why e2e in docker

The legacy `tests/zsh-widget.test.zsh` ran against `zsh -il` on the
developer's box. It worked, but:

- Risked mutating `~/.zsh_history` if `$HOME` wasn't sandboxed.
- Depended on whatever zsh + plugins were installed locally.
- Tickled the developer's terminal emulator with raw escapes — bad
  UX and flaky on certain emulators.

Putting it in docker:

- Pins zsh and OS versions (debian:bookworm-slim and alpine:3.20).
- Runs against a per-test fresh `$HOME` (created by `t.TempDir()` —
  no cross-scenario leakage even when tests run back-to-back).
- Exits cleanly even on a CI-killed run because the container is
  torn down.

## Why Go, not Tcl/expect

The May 2026 migration replaced 25 Tcl/expect scripts with Go test
files. The Go harness gives us:

- **Quiescence-based waits** instead of magic `sleep 0.6` calls —
  fewer flakes, lower CI wall time.
- **Cell-grid awareness** via `hinshun/vt10x` — assertions can ask
  "is the focus glyph `›` on row 3" instead of substring-grepping
  the raw byte stream.
- **Structured failure artifacts** — every failed test writes
  `screen.txt`, `raw.bin`, and `events.jsonl` to
  `e2e/_artifacts/<TestName>/` for post-mortem.
- **Single-language stack** — debugger, `go test -run`, and `-v`
  all work; no Tcl context-switch for contributors.
- **Module isolation** — the harness's `creack/pty` and `vt10x`
  dependencies live in a separate `e2e/go.mod` and are blocked
  from the release binary's dep graph by `task lint:no-test-deps`.

The full design is in `e2e/DESIGN.md`. Review findings from the
PoC iteration are in `e2e/REVIEW-FINDINGS.md`.

## Docker images

We run e2e on **two** libc surfaces because users actually run on
both:

- `e2e/docker/Dockerfile.debian` — bookworm-slim, glibc.
- `e2e/docker/Dockerfile.alpine`  — alpine 3.20, musl.

Both install only what the test binary needs to run a zsh session
(no Go inside the image — the test binary is precompiled on host
with `GOOS=linux GOARCH=amd64 go test -c`, per DESIGN.md decision 6).

The picker binary (`bin/zsh-history-enquirer-linux-amd64`) is
mounted at `/usr/local/bin/zsh-history-enquirer`. The plugin file
is mounted under `/opt/zsh-history-enquirer/plugin.zsh`. Per-test
`~/.zshrc` and `~/.zsh_history` are rendered from
`e2e/testdata/zshrc.template` and the chosen
`e2e/testdata/history/<name>.history` fixture into a scratch
HOME created by `t.TempDir()`.

## Interactive dev shell

`task dev` reuses the same `e2e/docker/Dockerfile.debian` image as
the automated harness, overriding its `/runner.sh` entrypoint with
`e2e/dev.sh`. `dev.sh` renders a scratch `$HOME` from the **same
canonical seed sources** the Go harness consumes —
`e2e/testdata/zshrc.template` (with `{{PLUGIN}}` substituted) and
`e2e/testdata/history/<fixture>.history` — then exec's `zsh -i`.
Because both surfaces read the identical files, the manual repro
path can never silently drift from what the scenarios assert.

Pick a fixture with `task dev FIXTURE=<name>` (default `seed`);
names match the files under `e2e/testdata/history/` minus the
`.history` suffix. The host's real `~/.zsh_history` is never
touched — the seeded entries live in the container only. Auto-runs
`task build:linux` so the binary you exercise is the one currently
in your working tree; re-running `task build:linux` between
sessions picks up code changes without rebuilding the image. Use
this to reproduce e2e-only bugs by hand or to sanity-check render
edges that don't yet have a scenario.

## Scenario coverage

`e2e/scenarios/<NN>_<topic>_test.go` — 24 active + 1 skip (25 total,
one-to-one with the retired `.exp` scripts):

| # | Test | Asserts |
| --- | --- | --- |
| 01 | `TestBasicPick` | empty input → ^R → echo ok appears, Enter submits |
| 02 | `TestMultiLineScroll` | scroll past a multi-line entry |
| 03 | `TestCancelPreservesInput` | Esc returns the typed-input invariant |
| 04 | `TestMultiWordSearch` | AND-filter narrows to entries with both tokens |
| 05 | `TestPasteBracketed` | `\e[200~ … \e[201~` reaches Input as one event |
| 06 | `TestPageUpPageDown` | rotation by limit |
| 07 | `TestHomeEnd` | head + tail focus |
| 08 | `TestPrefilterFromLBuffer` | LBUFFER pre-types into the picker filter |
| 09 | `TestMultilineSubmit` | select + run a multi-line entry |
| 10 | `TestMultilineRenderAndCancel` | filter to a multi-line entry, render, cancel |
| 11 | `TestMultilineScrollIntoView` | arrow-down a multi-line entry into the visible window |
| 12 | `TestEmptyHistory` (SKIP) | coverage merged into 03 — see test file rationale |
| 13 | `TestUnicodeEntries` | CJK / accented / emoji entries render and filter |
| 14 | `TestLongLineWrap` | a 200-char entry wraps onto multiple rows correctly |
| 15 | `TestViKeymap` | ^R works in viins and vicmd keymaps |
| 16 | `TestNarrowTerminalWrap` | 40-col × 24-row terminal forces multi-line wrap |
| 17 | `TestInputEdit` | Backspace + Ctrl-U re-filter |
| 18 | `TestFlagShapedLBuffer` | LBUFFER beginning with `--` is forwarded as input |
| 19 | `TestCtrlWWordDelete` | Ctrl-W kills the previous word + trailing whitespace |
| 20 | `TestAltBackspace` | `\e\x7f` aliases Ctrl-W; Esc-prefix does NOT cancel |
| 21 | `TestInputWrapEdit` | input row that wraps to a second visual line still edits cleanly |
| 22 | `TestPasteWithControlBytes` | bracketed paste payload containing control bytes lands verbatim |
| 23 | `TestFKeyNoCancel` | F1/F2 (`\eOP` / `\eOQ`) are silently swallowed |
| 24 | `TestMultilineNarrowWrapSubmit` | multi-line entry on 40-col terminal where each line also wraps; submit preserves every byte |
| 25 | `TestMultilineScrollDownNoStick` | ↓ at visible bottom advances focus onto a multi-line entry below |

Each test is one function with a per-call quiescence-based wait
budget. Failures dump the screen and raw bytes to
`e2e/_artifacts/<TestName>/`.

## VHS recordings

`e2e/tapes/*.tape` files render to MP4 + GIF via VHS for human-
watchable demos. These are documentation artifacts, NOT automated
assertions — they live in a separate `task record:examples` target
that is not part of `task check` / `task ci`. Outputs land in
`e2e/tapes/out/` (gitignored) and are committed selectively to
`docs/examples/` for README embedding.

## act compatibility

`task ci:e2e:run TARGET=debian` (or `TARGET=alpine`) is the
**single recipe** invoked both by GitHub Actions and by `act`. It
builds the image (cache-keyed by Dockerfile hash) and runs the
precompiled `harness.test` binary. The CI workflow file passes the
same `task` command to the runner; there is no "works locally,
fails on CI" gap.

`.actrc` pins `catthehacker/ubuntu:act-latest` because the
official guidance is to use an image that keeps `node` available
for `go-task/setup-task@v2`.

## Property tests (unit-layer)

Used in:

- `internal/history`: reverse + dedupe + unescape invariants.
- `internal/search`: tokenize / AndFilter monotonicity, payload-
  preservation.
- `internal/ui/wrap`: row-count monotonicity in input length.
- `internal/ui/highlight`: payload preservation under SGR
  stripping.
- `internal/keys/parser`: chunk-boundary invariance — splitting
  the same input at any byte boundary yields the same Event
  sequence as feeding it whole.

Test names use the `TestProperty_` prefix so `task test:property`
runs them in isolation.

## Fuzz tests

`internal/keys/parser_test.go` contains
`FuzzParser_NoPanicOnArbitraryBytes` — a Go-native fuzz target
that asserts `Parser.Feed` never panics regardless of input.
Standard `go test` runs each seed once as a regression check;
longer fuzzing runs via `go test -fuzz=... -fuzztime=...`.

## Smoke tests

`task release:smoke` exercises the rendered npm shim end-to-end:

1. `--version` locates the platform binary and execs it.
2. `--print-install-hint` emits the source-line hint.
3. Missing-binary fallback echoes argv back to stdout (the
   widget contract).

The smoke runs `release:dry-run` first so it always tests the
freshly-rendered packages.

## Coverage gate

CI fails if total coverage drops below **70%**. Actual coverage
hovers around 89–90%, so the gate has ~20 points of headroom for
legitimate untestable additions while still flagging
regressions. The pull-up gate applies to `coverage.out` produced
by `task ci:unit` (which is
`go test -race -count=1 -coverprofile=coverage.out ./...`).

Per-package targets:

| Package | Coverage | Notes |
| --- | --- | --- |
| `internal/search` | 100% | tokenize + AND-filter, pure functions. |
| `pkg/version` | 100% | -ldflags-injected fields. |
| `internal/ui` | ~98% | renderer + FSM; only debug-format strings uncovered. |
| `internal/history` | ~95% | ZshLoader's exec wrapper requires zsh. |
| `internal/keys` | ~94% | parser + reader; pty-driven master-close exit pinned, the remaining couple of percent is the EINTR retry path on the live read syscall. |
| `internal/tty` | ~75% | termios + cursor probe; pty-driven Size + Close-while-raw paths now pinned. /dev/tty Open() and NewDevTTY remain uncovered (require a real controlling terminal). |
| `internal/app` | ~74% | Run() body needs a TTY; pty tests cover runEventLoop submit / cancel / preEvents / channel-closed / trailing-flush, fetchInitialState's twin panic-recovery defenses, and readGeometry's three arms. e2e covers the orchestration layer. |
| `cmd/zsh-history-enquirer` | ~45% | main() needs a TTY; the panic-recovery helpers and version/help fast-paths are unit-tested, full main() flow is smoke-tested via the binary. |
