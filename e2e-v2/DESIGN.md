# e2e-v2 — Go-Native TUI Harness Design

This document is the architectural plan for replacing the 25
Tcl/expect scenarios under `e2e/scenarios/*.exp`
(`e2e/scenarios/01-basic-pick.exp` … `25-multiline-scroll-down-no-stick.exp`)
with a modern Go-based end-to-end harness.

The new harness still drives **a real `zsh -il`** under a **real
pty inside Docker** and triggers `^R` exactly as a user would
(`\x12` over a real pty). It must not bypass zsh by invoking the
picker binary directly: that path is already covered by
`internal/...` unit tests and would lose the
plugin → BUFFER capture contract pinned in
`docs/spec/10-widget-contract.md` and surfaced in
`cmd/zsh-history-enquirer/main.go:75-105` (the four
preserve-input paths called out in `AGENTS.md`).

The legacy expect harness is preserved verbatim throughout
this design's transition window (existing `task test:e2e`,
`task test:e2e:one`, `e2e/run.sh`, `e2e/scenarios/*.exp`,
`e2e/{debian,alpine}/Dockerfile`); v2 lands alongside as
`task test:e2e:v2` and only after parity is reached does the old
path get removed.

## Goals + non-goals

Goals: deterministic, debuggable, easy to extend; same
two-libc coverage; richer assertions than substring sleeps;
human-watchable docs artifacts.

Non-goals: replacing unit tests; running on macOS as the
primary path; reducing what the user-facing scenario actually
exercises (we still need zsh + pty + Docker).

## Decisions

### 1. pty library — `creack/pty`

**Decision: `github.com/creack/pty`.** It is already a top-level
dependency of the main module (`go.mod:7`, used by
`internal/keys`'s pty tests) so the team has institutional
familiarity with its quirks; it provides `pty.StartWithSize`,
`pty.Setsize`, and exposes the master fd as an `*os.File` (which
is what `creack/pty` and `unix.Poll` agree on — same primitives
used by the production binary's reader at
`internal/tty/cursor.go`). Linux-only is fine because v2 only
ever runs inside our Docker images.
Rejecting `aymanbagabas/go-pty`: its abstraction targets
cross-platform (Windows ConPTY) which we do not need and
which adds a layer over what is otherwise a 200-line `unix.Open`
wrapper.

### 2. Terminal emulator — `hinshun/vt10x`

**Decision: `github.com/hinshun/vt10x`.** It is a pure-Go,
in-process VT100/xterm parser that produces a snapshot-able
cell grid (rune + fg/bg/attrs per cell). Crucially, the
production picker emits via `charmbracelet/x/ansi`
(`internal/app/run.go:11`), and the SGR / CUP / CSI / ED sequences
the renderer uses (`HideCursor`, `SetModeBracketedPaste`,
`CursorToCol`, etc.) are all squarely in vt10x's vocabulary.
Rejecting `gdamore/tcell` SimulationScreen: it is designed for
re-hosting tcell programs and assumes the program *is* the
screen. The picker is an **inline overlay** — it renders only
rows `InitRow..InitRow+state.size` (see the "do not
EraseDisplayDown" gotcha in `AGENTS.md`) — and a tcell
SimulationScreen forces us to fight a full-screen abstraction
the picker explicitly does not use.

### 3. Wait policy — `WaitQuiescent`

**Decision: a single primitive `WaitQuiescent(min, max time.Duration)
time.Duration`** on the harness session. Semantics:

* read bytes from the pty master into the vt10x parser in a
  background goroutine,
* every byte resets a "last activity" timestamp,
* return as soon as the parser has been idle for `min` (default
  60 ms — slightly above the `RenderInterval = 72 * time.Millisecond`
  in `internal/app/run.go:26`'s throttle window so a frame in
  flight is included),
* hard-cap at `max` (default 2 s; configurable per scenario for
  the SIGWINCH-style slow paths).

For the docker DSR-timeout fallback (the always-fires `CursorTimeout =
250 * time.Millisecond` from `internal/app/run.go:23` documented in
`docs/plan/20-followups.md:38-44`), `WaitQuiescent` after the
initial `\x12` press uses `max = 2s` so the 250 ms probe wait plus
the history fetch plus the first render all happen inside one
call. The legacy expect harness used `sleep 1.2` after `\x12`
(see `e2e/scenarios/01-basic-pick.exp:20`) for the same reason —
that becomes `s.SendCtrlR(); s.WaitQuiescent()` in v2, no fixed
sleep.

There is also `Send(bytes []byte)` which simply writes to the pty
master with no implicit wait, plus `Type(s string)` =
`Send([]byte(s))` for ergonomics. Helpers `SendKey(KeyDown)`,
`SendCtrlR`, `SendEsc`, `SendEnter`, `SendBracketedPaste(payload)`
generate the same raw byte sequences expect was sending
(`\x12`, `\x1b`, `\r`, `\x1b[200~...\x1b[201~`, `\x1b[B`, …).

### 4. Assertion model — hybrid, `RowContains` default

**Decision: default to `RowContains(row int, substr string)` and
`Screen().Contains(substr)`, with cell-grid golden snapshots as an
opt-in for layout-heavy scenarios.** Substring-on-row tracks how
the existing expect scenarios already think
(`e2e/scenarios/09-multiline-submit.exp:40-41` =
`expect -- "beta"`; `expect -- "gamma"`) which keeps the
migration mechanical and the failure messages readable
("row 5 had `% echo o`, wanted contains `echo ok`").

For the wrap / dynamic-limit boundary scenarios
(`16-narrow-terminal-wrap.exp`, `21-input-wrap-edit.exp`,
`24-multiline-narrow-wrap-submit.exp`,
`25-multiline-scroll-down-no-stick.exp`), the default is
extended with `s.AssertGoldenScreen("testdata/golden/24.txt")`
which dumps the vt10x cell grid (printable rune per cell,
spaces for empties) into a text file and diffs against the
golden. Goldens regenerate with `go test ./... -update`
(standard Go convention; the flag is registered once in the
package via `flag.Bool`). SGR colour is asserted on the side via
`s.RowSGR(row, col)` for the bold-cyan token highlight
(`internal/ui/render.go:highlight`) and the `NO_COLOR` opt-out
path — golden files stay rune-only because SGR ordering can vary
within a frame depending on the SGR-reset emitter and we want
goldens to be stable across small renderer touch-ups.

### 5. Module layout — independent `e2e-v2/` submodule

**Decision: a separate Go module rooted at `e2e-v2/`.** The
directory holds its own `go.mod`:

```text
module github.com/zthxxx/zsh-history-enquirer/e2e-v2

go 1.25.0

require (
  github.com/creack/pty v1.1.24
  github.com/hinshun/vt10x v0.0.0-...
  github.com/stretchr/testify v1.11.1
)
```

The release binary's module remains `github.com/zthxxx/zsh-history-enquirer`
with the dependency set already pinned in `go.mod:5-13`. A
**`go.work` is deliberately NOT introduced** — `go.work` would
unify the dependency graph at `go test ./...` from the root, and
the constraint says vt10x must never appear in the production
binary's tree. Without `go.work`, the root `go build ./...` and
`go test ./...` simply will not see `e2e-v2/` as part of the
main module, so its dependencies are truly orphaned from the
main build. A new task `task lint:no-test-deps` runs
`go list -deps -f '{{.Module}}' ./... | sort -u` from the main
module and asserts the output contains neither `vt10x` nor any
`e2e-v2` import path — fails CI loud if anyone accidentally
adds `go.work` or a stray import. This mirrors the same belt-
and-braces discipline as the existing
`file ... | grep -v 'dynamically linked'` check
(`AGENTS.md` "Static linkage is mandatory" section).

### 6. Container model — option (A): precompiled test binary

**Decision: precompile the test binary on the host with
`GOOS=linux GOARCH=amd64 go test -c -o e2e-v2/bin/harness.test
./e2e-v2/scenarios/...`** then bind-mount it into the
existing minimal image alongside the picker binary. The image
stays as it is today (zsh + ca-certificates + locales +
[ncurses on alpine] — `e2e/debian/Dockerfile`,
`e2e/alpine/Dockerfile`); no Go inside the image.

Rationale: constraint #2 (test deps never in release binary) is
already enforced by the separate module + the no-`go.work`
discipline; but option (A) also keeps the image cache key
unchanged (which is what `task ci:e2e:run` and `task dev`
optimise for, see `Taskfile.yml:121-128` and `:354-367`),
shaves `apt-get install golang-go` (~250 MB) off the image, and
sidesteps the Go toolchain version-skew problem (the host and
the container would have to be kept in lockstep on
`go 1.25.0` for the module to even build inside). The test
binary is statically linked (`CGO_ENABLED=0`) so it runs on
both debian glibc and alpine musl without issue — the same
discipline the picker binary already follows.

### 7. History seed model — files in `testdata/`, mounted read-only

**Decision: seed histories live as files under
`e2e-v2/testdata/history/<NAME>.history`**, one file per
fixture. The default fixture is a Go-level transcription of the
heredoc in `e2e/seed-home.sh:53-100` (`write_seed_history`),
checked in as `testdata/history/seed.history`. Scenarios that
need a custom seed (the multiline-narrow + multiline-stick
scenarios that today do `exec sh -c { printf … > /tmp/… }` in
`e2e/scenarios/24-multiline-narrow-wrap-submit.exp:44-48` and
`25-multiline-scroll-down-no-stick.exp:46-51`) instead reference
a dedicated file, e.g. `testdata/history/multiline-stick.history`.
The whole `testdata/history/` directory is mounted at
`/seed-history/` read-only; a per-scenario setup helper copies
the chosen file into the scratch HOME so the picker's `fc -R`
reload path runs against a real, writable-but-rewritten file
(mirrors how `seed-home.sh:write_seed_history` rewrites
`$HOME/.zsh_history` between scenarios in
`e2e/run.sh:38-40`).

The legacy `e2e/seed-home.sh:write_zshrc` content — disabling
SHARE_HISTORY / INC_APPEND_HISTORY etc. — moves into a
checked-in `testdata/zshrc.template` and is rendered into the
scratch HOME by the same helper, with the plugin path
substituted in. **No host data is reachable:** the only volumes
the container sees are the binary, the plugin, the
read-only `testdata/`, and the precompiled test binary itself.
Real `~/.zsh_history`, the host `$HOME`, and any other host file
are completely outside the container's filesystem view.

### 8. Scenario isolation — per-test scratch `$HOME` via `t.TempDir`

**Decision: the harness exposes
`s := harness.NewSession(t, opts)` where `t` is the
`*testing.T`.** Each session creates a fresh scratch directory
via `t.TempDir()` (Go's stdlib auto-cleanup), writes the
chosen seed history and rendered zshrc into it, then
`pty.Start`s `zsh -il` with `HOME=<scratch>` and
`HISTFILE=<scratch>/.zsh_history`. Two sessions running back-
to-back never share state because `t.TempDir()` returns a
unique path per `t` and the kernel cleans the contents after
the test returns. No equivalent of `e2e/run.sh`'s
`write_zshrc; write_seed_history` global rewrite is needed —
isolation is structural rather than serial. This is also
what makes `go test -parallel N` safely usable in the future
(each test gets its own pty, its own zsh process, its own
HOME), although phase 1 keeps `-parallel 1` until we measure
docker overhead with multiple concurrent zsh sessions per
container.

### 9. Two-libc coverage — same test binary, two `docker run`

**Decision: build the test binary once on the host, run it
twice — once in the debian image, once in the alpine image.**
The Taskfile target `test:e2e:v2` loops
`for: { var: TARGETS, as: TARGET, split: ' ' }` exactly the
way `test:e2e` already does (`Taskfile.yml:196-203`).
Both runs mount the same `harness.test` binary at
`/usr/local/bin/harness.test` and invoke it as the container's
command; the binary discovers its scenarios through Go's
testing package (they're all `Test*` funcs registered by
`go test -c`). Output flows back via stdout / stderr just like
the current expect run does.

### 10. VHS recordings — separate `.tape` files, opt-in target

**Decision: `.tape` files live under `e2e-v2/tapes/<NN>-<name>.tape`,
output `.mp4` artifacts under `e2e-v2/tapes/out/`** (gitignored;
they regenerate on demand). VHS runs only via an explicit
`task record:examples` target that loops over every tape under
`e2e-v2/tapes/`. It is **NOT a dependency of `task check` /
`task test` / `task ci`** — recordings are documentation
artifacts, not assertions. The `.tape` scripts are deliberately
distinct from the assertion-bearing Go scenarios: a tape's job
is to look pretty (slower keystroke pacing, no implicit
quiescence waits), the Go scenarios' job is to be fast and
deterministic. The two-track split is the same pattern bubbletea
and lipgloss use upstream.

Naming convention: tapes mirror the Go scenario name where
applicable (`01-basic-pick.tape` ↔ `TestBasicPick`), but
`tapes/` may also contain marketing-only tapes (a 60-second
README hero loop) that have no test counterpart.

### 11. Taskfile integration — `test:e2e:v2` alongside

**Decision: introduce `task test:e2e:v2` and
`task test:e2e:v2:one TARGET=...` modelled on the existing
`test:e2e` / `test:e2e:one` (`Taskfile.yml:196-211`).**
Migration order:

1. Land v2 scenarios for the first 6 expect scripts (covers the
   plain-cancel, submit, paste, navigation surfaces) alongside
   the existing expect harness.
2. CI runs both jobs in parallel for one release cycle. Old
   harness stays green; new harness gains parity.
3. Migrate the remaining 19 scenarios.
4. Delete `e2e/scenarios/`, `e2e/run.sh`, the expect dependency
   from both Dockerfiles, and the `test:e2e` Taskfile target.
   `test:e2e:v2` is renamed to `test:e2e` in the same commit;
   the symlink is invisible to `task test`.

A new helper `task build:e2e-harness` (depends on
`build:linux`) precompiles `harness.test` into `bin/` and is
listed in `test:e2e:v2`'s `deps:` so a fresh checkout `task
test:e2e:v2` works without surprises.

### 12. Failure debugging — `t.Cleanup` dumps grid + raw bytes

**Decision: every `harness.NewSession` registers a
`t.Cleanup` that, *if the test failed*, writes three artifacts
into `e2e-v2/_artifacts/<TestName>/`:**

1. `screen.txt` — the final vt10x cell grid (printable runes,
   ' ' for empty cells, '\n' between rows). This is the same
   format the goldens use.
2. `raw.bin` — every byte the harness ever read off the pty
   master, ungrouped, in order. Lets a human replay the session
   in their head when the screen dump is ambiguous (paste
   payloads, partial CSI sequences, control bytes — all the
   stuff that the expect `log_user 1` output currently surfaces
   noisily in CI).
3. `events.jsonl` — one JSON line per send/wait/assert call
   the harness made, with timestamps. Lets us correlate a
   late-arriving render frame to the wait that should have
   covered it.

On the CI side, `task test:e2e:v2` `tar`s up `_artifacts/`
on failure and the GitHub Actions job uploads it as a workflow
artifact. This is the structural fix for the legacy harness's
"timeout in scenario N; here are 200 lines of `log_user 1`
output, good luck" pattern.

## How the design satisfies each hard constraint

**(1) Docker-only, no host data:** the container mounts exactly
four read-only volumes (binary, plugin, `testdata/`, harness
test binary) and one writable scratch `$HOME` created via
`t.TempDir()` inside the container's filesystem — never bind-
mounted from the host. The pattern is identical to today's
`task ci:e2e:run` mount surface (`Taskfile.yml:368-375`),
minus the host `e2e/scenarios:/scenarios:ro` line. No host
`~/.zsh_history`, host `$HOME`, or host config is reachable.

**(2) Test deps never leak into release binary:** the new
module lives at `e2e-v2/` with its own `go.mod`; we explicitly
do not create a `go.work` file that would otherwise unify the
graph. The `task lint:no-test-deps` CI check parses
`go list -deps -f '{{.Module}}' ./...` from the main module and
fails on any line matching `vt10x` or `e2e-v2`. The release
build (`task build`, `task build:all`,
`scripts/release/build-npm.sh`) runs against the main module
only and cannot see the `e2e-v2/` module's go.sum.

**(3) Real `zsh -il` + real `^R`:** the harness's
`session.Open()` does `pty.StartWithSize(exec.Command("zsh",
"-il"), ws)` and then `session.Send([]byte{0x12})` — exactly the
same wire sequence as expect's `spawn zsh -il` +
`send -- "\x12"` in `e2e/scenarios/01-basic-pick.exp:10,19`. zsh
loads the plugin via the rendered `.zshrc`'s
`source /opt/zsh-history-enquirer/plugin.zsh` (same path the
existing harness uses, `e2e/seed-home.sh:32,49`), so the
widget's `BUFFER=$(zsh-history-enquirer -- "$LBUFFER")` capture
path (`plugin/zsh-history-enquirer.plugin.zsh:29`) is exercised
end-to-end. The harness never invokes the picker binary
directly.

**(4) VHS for docs only:** see decision 10. `record:examples`
is a standalone Taskfile target with no `deps:` on the
assertion path. `task check`, `task ci`, `task test`, and the
GitHub Actions ci.yml workflow do not call it. The MP4
artifacts under `e2e-v2/tapes/out/` are gitignored and
regenerate on demand for documentation embedding.

## Top 5 risks + mitigations

1. **vt10x ANSI parser gaps.** vt10x has historically lagged
   xterm on newer SGR (256-colour, truecolour, RGB-via-CSI 38;2)
   and on the obscure DECPM / DECRPM private modes. The
   renderer emits `ansi.SetModeBracketedPaste` + `ansi.HideCursor`
   (`internal/app/run.go:61`) and `\e[0m` after every row
   (`docs/plan/20-followups.md:671-675`). Mitigation: add a
   smoke test under `e2e-v2/internal/term/` that pipes every
   SGR/cursor/mode escape the production renderer emits through
   the parser and asserts no "unhandled CSI" warning. If
   anything new in `internal/ui/render.go` lands a sequence
   vt10x doesn't parse, the smoke test fails and we fix it in
   one place before the scenario tests start producing weird
   cell-grid noise.
2. **Docker pty DSR cursor probe always times out.** Already
   documented at `docs/plan/20-followups.md:38-44`. v2 inherits
   this; `WaitQuiescent`'s `max=2s` after the initial `^R`
   absorbs the 250 ms probe timeout (`internal/app/run.go:23`)
   inside one quiescent window. The renderer correctness when
   the probe fails is verified by unit tests at the model
   layer; v2 does not assert a column-0 vs column-N starting
   position because that's a probe-success-only artefact users
   never see broken in production.
3. **Golden flakiness from SGR ordering.** The picker emits SGR
   in the order `\e[1;36m`-prefix → payload → `\e[0m`-reset; a
   different renderer micro-refactor could legitimately emit
   `\e[36;1m` (same effect, different bytes) and a raw-byte
   golden would diff. Mitigation: goldens store **printable
   runes only** (vt10x cell `.Char()`), not raw bytes. SGR is
   asserted on a separate axis via `s.RowSGR(row, col)` →
   `vt10x.Style` value, which is order-independent (vt10x folds
   SGR into a single composed style).
4. **VHS Linux/macOS divergence.** VHS itself is cross-platform
   but it shells out to ttyd + ffmpeg, both of which have
   different system fonts on macOS vs Linux. A `.tape` recorded
   on macOS may look subtly different on the GitHub Actions
   Linux runner. Mitigation: VHS recording is opt-in per
   developer (`task record:examples` runs locally), MP4 outputs
   are checked into a separate `docs-assets` release channel,
   not into the source tree. The Go scenario harness does NOT
   rely on VHS, so visual divergence does not affect tests.
5. **creack/pty winsize race on SIGWINCH.** Setting the pty
   size via `pty.Setsize` and then reading from the master
   races with the child's `SIGWINCH` handler — if the harness
   pumps a key during the resize, the picker may read the key
   before the new geometry, render at the old size, then
   reflow when WINCH arrives. Mitigation: `session.Resize(rows,
   cols)` calls `pty.Setsize` *then* `WaitQuiescent(80ms,
   1s)` *then* returns control to the test. The 80 ms floor
   covers the typical zsh-side WINCH handler delay measured
   empirically; the 1 s ceiling covers the worst-case render
   reflow. Scenarios that exercise mid-session resize
   (currently zero — see the open followup at
   `docs/plan/20-followups.md:46-62`) can be authored
   confidently because the Resize helper is the single
   synchronisation point.

## What lands first (informational, not part of this design)

Phase 1 (scope of the next implementation session): the
`e2e-v2/` module skeleton, the `harness` package with
`Session` / `Send*` / `WaitQuiescent` / `Screen` /
`RowContains`, the `_artifacts` cleanup hook, and v2
implementations of scenarios 01 (basic-pick), 03 (cancel-
preserves-input), 09 (multiline-submit), and 16 (narrow-
terminal-wrap) to prove the surface against representative
boundary cases. The remaining 21 scenarios migrate
mechanically in phase 2, after we know the harness API holds
up under real assertion shapes.
