# plan/20-followups — discovered during execution

Items added here come from doing the work, not from up-front planning.
Each entry must include:

- **Date** in `YYYY-MM-DD`
- **Why** — what surfaced it
- **Where** — file/line if applicable
- **State** — `open` / `addressed in <commit>` / `wontfix because <reason>`

## Open

* **2026-05-07** — Initial post-load DSR cursor probe always falls back
  inside docker (expect's pty doesn't reply). Production terminals do
  reply; the user-facing impact is "first run looks fine." The fallback
  draws starting at column 1 instead of inline at the prompt column.
  Why open: when running in a real terminal the probe works, so this
  only matters for tests. How to apply: don't try to fix it for
  expect — the renderer's correctness is verified at the model layer.

(One open item — the docker pty's DSR limitation. Intrinsic to the
test harness, not a code bug. The earlier narrow-terminal-wrap
companion was resolved in the
\`test(e2e): + narrow-terminal-wrap scenario\` commit.)

## Addressed

* **2026-05-07** — Added `NO_COLOR` opt-out per the
  [no-color.org](https://no-color.org) convention. Setting any
  non-empty value suppresses both the bold-cyan token highlight
  AND the per-entry SGR reset. Search / filter behavior unchanged.
  Two regression tests pin the on/off toggling. Documented in
  README, README.zh-CN, and design/50-ui.md.

* **2026-05-07** — Modifier-key arrow / Home / End / PgUp / PgDn /
  Delete sequences were silently swallowed. xterm encodes them as
  `\e[1;<modifier><letter>` (arrows + Home/End) or
  `\e[<row>;<modifier>~` (PgUp/PgDn/Delete). The parser's dispatch
  table only matched the plain forms, so Shift+Up / Ctrl+Up /
  Alt+Up etc. fell through to the "unknown CSI; ignore" branch —
  every modified press did nothing visible. Added
  `stripCSIModifier` (`internal/keys/parser.go`) that normalizes
  both encoding forms to the plain counterpart before dispatch.
  The picker has no per-modifier behavior, so swallowing the
  modifier is friendlier than swallowing the press. 13 regression
  cases (every reasonable modifier × arrow/home/end/pgup/pgdn/del)
  + 6 passthrough cases (DSR replies stay unchanged).

* **2026-05-07** — Arrow keys via SS3 sequences (`\eOA` / `\eOB`
  / `\eOC` / `\eOD`) were not handled by the parser. Some terminals
  (xterm in DECCKM application-keypad mode, certain VT-series
  emulators, embedded firmware) send `\eO<key>` instead of the CSI
  `\e[<key>` form. Falling through to the "ESC + unrelated byte"
  branch, the parser emitted `KeyEsc + 'O' rune + arrow letter
  rune` — every arrow press would CANCEL the picker. Added
  `stateSS3` + `feedSS3` to internal/keys/parser.go covering A/B/C/D
  (arrows) and H/F (Home/End). `FlushEsc` also drains a hung SS3
  prelude (`\eO` with no follow-up byte) to avoid blocking input.
  Three regression tests cover arrow recognition, unknown-byte
  safety fallback, and the flush case.

* **2026-05-07** — Architectural mismatch surfaced by an added e2e
  scenario (19-ctrl-w-word-delete): `fx.StartTimeout(5s)` and a
  matching 5-second `context.WithTimeout` in main were intended to
  prevent stuck startup, but `invokeRun.OnStart` runs the picker
  synchronously, so any interactive session > 5s tripped
  `context.DeadlineExceeded` mid-render. The picker would die with
  terminal stuck in raw mode and `recoverStartFailure` echoing
  argv to BUFFER. Fix: bump both timeouts to 1 hour. The picker is
  bounded by user attention, not a wall clock; the
  `signal.NotifyContext` wrapper around runCtx already provides
  proper teardown on real signals (SIGINT / SIGTERM / SIGHUP).
  Lock-down: scenario 19's expect/sleep total is ~5.5s — without
  the bump, that scenario would fail; with the bump, it passes
  cleanly. Documented in AGENTS.md "preserve-input invariant" so a
  future contributor doesn't tighten the timeout thinking it's a
  defensive default.

* **2026-05-07** — Cross-cutting bug surfaced by the previous fix:
  the plugin started passing `bin -- "$LBUFFER"` (to neutralize
  flag-fast-path collisions), but the npm shim's missing-binary
  fallback path joined `argv.slice(2)` verbatim, so a missing
  platform package would echo `-- $LBUFFER` to stdout instead of
  just `$LBUFFER`. BUFFER would land as `-- typed text`. Fixed
  `npm/packages/zsh-history-enquirer/bin/cli.js` to strip a leading
  `--` from `argv` before joining. Updated `task release:smoke`
  step 3/4 to test BOTH the direct-invocation shape (no `--`) and
  the widget-mode shape (with `--`); both must produce the user's
  typed text on stdout.

* **2026-05-07** — Bracketed-paste of multi-line text scribbled
  across the terminal. The picker's input row was rendered verbatim,
  so a `\r` in the paste payload carriage-returned into the prompt
  prefix and a `\n` pushed the picker rendering down. Useless as
  filter input either way. Added `sanitizeInputRune` /
  `sanitizeInputString` in `internal/ui/update.go` that translate
  `\n` / `\r` / `\t` to space at both the per-keystroke and paste
  entry points. Six regression cases. Token-boundary semantics
  preserved because space is the same separator
  `search.Tokenize` already splits on.

* **2026-05-07** — Two input-editing gaps surfaced under critical
  scrutiny:
  - **Backspace deleted one BYTE, not one rune.** For ASCII this is
    fine (1 byte == 1 rune); for CJK / emoji / accented Latin it
    leaves a trailing UTF-8 continuation byte and corrupts the
    input into invalid UTF-8 — subsequent renders show `\xe4\xbd`
    mojibake. Switched `internal/ui/update.go:KeyBackspace` to
    `utf8.DecodeLastRuneInString` so multi-byte runes delete
    atomically. Six regression cases (chinese / emoji / accented /
    prefixed variants).
  - **Ctrl-W silently no-op.** Common zsh muscle memory
    (`backward-kill-word`); the FSM parser already produced
    `KeyCtrlW` from `0x17` but `update.go` had no case for it.
    Added `deleteLastWord` helper (rune-walk, strips trailing
    whitespace then the preceding word). Seven regression cases
    documented in `model_test.go` covering ASCII, multi-word, CJK,
    emoji, edge cases. Spec/50 and both READMEs updated.

* **2026-05-07** — Two flag-fast-path foot-guns, fixed in one
  iteration:
  - **`$LBUFFER="--version"` blanked input.** The widget invoked the
    binary as `bin "$LBUFFER"`, so when LBUFFER literally was
    `--version` (or any other doc fast-path token), `args=["bin",
    "--version"]` matched `isVersionFlag` and the binary printed
    the version into `BUFFER` instead of opening the picker. The
    `isVersionFlag` comment claimed protection ("the picker opens
    normally because there's a positional arg alongside the flag")
    that wasn't actually true for the bare-flag case. Fix: the
    plugin now passes `--` before `$LBUFFER`, so widget invocations
    are always `len(args) == 3` and never trip the fast-path. The
    `--` is a stdlib `flag` package terminator that stops flag
    parsing.
  - **`--help` stacked help text under "startup failed:".** The
    flag parser returned `flag.ErrHelp`, which propagated through
    fx as a provider failure. Users saw the help text *and* a
    stack-trace-shaped error. Added `app.PrintHelp` helper plus an
    `isHelpFlag` fast-path in main.go. Clean help, exit 0, no
    spurious error. Drift-detection test
    (`TestPrintHelp_MatchesNewConfigFlags`) catches future
    regressions where one declares a flag and the other forgets.
  
  Both fixes documented in spec/10-widget-contract.md as a new
  "doc fast-paths" row in the binary contract table, plus the `--`
  separator behavior in the widget definition.

* **2026-05-07** — Highlighter emitted invalid UTF-8 on Unicode
  case-fold mismatches. The match-detection layer (`search.AndFilter`)
  is correct because it only checks `strings.Contains(lc, t)`. But
  `highlight()` was using `strings.Index(lc, t)` to locate spans and
  then slicing `s` with those byte indices to wrap them in SGR codes.
  For runes whose ToLower changes byte length (Turkish `İ` (2B) →
  `i` (1B); some expansions grow), `lc`'s byte indices no longer
  point to character boundaries in `s` — slicing `s` produces broken
  `\xb0...` partial-rune mojibake that gets written verbatim to the
  terminal. Added a `len(lc) != len(s)` guard in
  `internal/ui/render.go:highlight` that falls back to returning the
  original string unhighlighted (cosmetic-only fallback; the user
  still sees their match, just without bold-cyan markup). Two
  regression tests pin Turkish-İ; the existing `[a-z]` rapid property
  unaffected. The "real" fix (parallel byte-offset map between lc
  and s) is more code than this is worth — the failure mode is
  intermittent, only the rendering is broken, and a fallback to "no
  highlight" is genuinely fine.

* **2026-05-07** — Third widget-contract gap surfaced under critical
  scrutiny: `Run()` received `context.Background()`, which is never
  canceled, even on SIGTERM / SIGHUP. If the user sent
  `kill -TERM <pid>` while the picker was rendering — or closed the
  containing terminal emulator — the Go runtime tore the process
  down without running fx OnStop hooks. The `*os.File` for `/dev/tty`
  was abandoned in raw mode (no echo, no canonical, no signals),
  forcing the user to run `stty sane`. `invokeRun` now wraps
  `context.Background()` with `signal.NotifyContext(SIGINT, SIGTERM,
  SIGHUP)` so the runEventLoop's `<-ctx.Done()` case fires, the
  cancel-preserves-input path runs, fx OnStop runs, and the
  terminal is restored. SIGINT from Ctrl-C inside the picker
  continues to arrive as byte 0x03 (ISIG is off in raw mode); this
  fix is purely for *external* kills.

* **2026-05-07** — Widget contract had a SECOND uncovered gap: even
  with `preserveOnError` patched in inside `app.Module.invokeRun`,
  the safety net does not fire if fx itself fails to start (e.g.
  `tty.NewDevTTY` returns an error because `/dev/tty` is unopenable).
  In that case `invokeRun` is never called and the binary exits
  without writing to stdout — `BUFFER=$(...)` blanks `$LBUFFER`
  identically to the previously-fixed early-Run-error case. Added
  `recoverStartFailure` in `cmd/zsh-history-enquirer/main.go` that
  reconstructs `cfg.Input` via `app.NewConfig` and writes it back to
  stdout when `a.Start()` returns an error. Three table tests
  (PreservesInputOnArgs, NoArgsLeavesStdoutEmpty, TolerantOfBadArgs)
  pin the contract.

* **2026-05-07** — Widget contract had a latent gap on hard
  early-error paths in the binary. The npm shim correctly echoed
  argv back when the platform sub-package was missing, but if the
  Go binary itself started and then failed in `t.EnterRaw()` or
  `readGeometry()`, `Run` returned `(nil, err)` and the umbrella
  `invokeRun` skipped `PrintResult` (guarded on non-nil result).
  Stdout was then empty, `BUFFER=$(...)` resolved to "", and the
  user's `$LBUFFER` was silently destroyed. Added
  `preserveOnError(result, err, cfg.Input)` in `internal/app/module.go`
  that synthesizes a `RunResult` from `cfg.Input` whenever the
  result is nil and an error fired. Property-style table test in
  `module_test.go` pins all four (result, err) × (input) quadrants.
  The widget contract now holds across every early-error code path,
  not just the ones I happened to test interactively.

* **2026-05-07** — `build-npm.sh` published every version under
  npm's `latest` dist-tag because the publish call had no `--tag`.
  The release.yml workflow comment said pre-releases (`v1.0.0-rc.1`)
  should publish under `next`, but the implementation didn't honour
  it — a pre-release would have replaced the stable `latest` install.
  Fix: derive `NPM_TAG` from the version string (`*-*` → `next`,
  else `latest`) and pass `--tag` to both platform-package and
  umbrella publishes. The contract in spec/80-release-process is
  now actually enforced.

* **2026-05-07** — `task lint:sh` and `task lint:arch` had a
  silent-masking bug from a single-line shell chain:

  ```yaml
  command -v X >/dev/null && X check || (echo "not installed" >&2)
  ```

  When `X` IS installed AND finds violations, the `&&` branch
  fails, the `||` fires, the script prints the misleading "not
  installed" message and exits 0. Real shellcheck / go-arch-lint
  failures were silently masked — particularly bad for arch-lint
  because the gate exists exactly to catch layering violations.
  Replaced both with `if/else` blocks that exit cleanly only when
  the binary is truly absent, otherwise propagate the lint exit
  code. Resolved in this iteration's commits.

  Note: the markdownlint version of the same chain (`bun ... ||
  npx ...`) is *correct* because both branches are full lint
  commands, so a bun-install-but-no-package failure correctly falls
  back to npx. Tried to "fix" that one too in the same iteration
  and broke the fallback path; reverted with an inline comment so
  future readers don't try the same revert.

* **2026-05-07** — Homebrew formula installed only the binary, not
  the plugin file. The README and the formula's caveats both
  pointed at `$(brew --prefix)/share/zsh-history-enquirer/plugin.zsh`,
  but `def install` did `bin.install Dir["zsh-history-enquirer-*"].first`
  and nothing else. Fixed in three coordinated places:
  (1) `task ci:release:package` now copies plugin/zsh-history-enquirer.plugin.zsh
      into release/ alongside the binaries (and lists it in checksums.txt);
  (2) the formula now declares a `resource "plugin"` block pointing
      at the same GitHub Release;
  (3) `def install` stages the resource into pkgshare as plugin.zsh.
  Plus a migration guard: if an existing tap formula has no
  `resource "plugin"`, regenerate the file from the template instead
  of just bumping versions. Resolved in this iteration's commits.

* **2026-05-07** — npm umbrella + per-platform packages shipped a
  one-line stub LICENSE pointing back at GitHub. Downstream
  license-compliance scanners (Snyk, FOSSA, BlackDuck, ...) read
  the LICENSE file directly and would mark the package as
  license-unclear. Replaced both LICENSE files with the full MIT
  text from the project root. The build script now sources the
  umbrella's LICENSE from project root for the same single-source-
  of-truth reason as plugin.zsh. Resolved in this iteration's
  commits.

* **2026-05-07** — `npm/packages/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh`
  was a stale duplicate of the project-root `plugin/zsh-history-enquirer.plugin.zsh`.
  When the project-root file was fixed in the keymap commit, the npm
  copy was NOT updated, so `npm install -g zsh-history-enquirer`
  users would have received the broken plugin. Removed the duplicate;
  build-npm.sh now copies from the project root. Resolved in this
  iteration's commits.

* **2026-05-07** — `task setup` pinned golangci-lint@v2.1.6 — built
  against Go 1.24, refused to load against go.mod's `go 1.25.0`.
  CI was already on v2.12.2; `task setup` got missed during that
  bump. A fresh contributor running `task setup` would hit
  `ERRO Running error: go-version: invalid Go version` and get
  stuck. Bumped `task setup` to v2.12.2 with an inline comment
  explaining why the version matters. Resolved in this iteration's
  commit.

* **2026-05-07** — `scripts/release/build-npm.sh` had `rm -rf "${BUILD}"/*`.
  shellcheck flagged this (SC2115): if `${BUILD}` is ever empty,
  the shell expands to `rm -rf /*` — catastrophic. Added the `:?`
  guard. Plus wired shellcheck into `task lint:sh` and the CI lint
  job so future regressions are caught. Resolved in this iteration's
  commits.

* **2026-05-07** — `expect` cannot reliably pass an `stty rows N cols N`
  to the spawned process across hosts. The "narrow-terminal multi-line
  wrap" scenario was deferred from `e2e/scenarios/`. **Resolved**: the
  trick is to set the slave-pty geometry AFTER spawn but on the
  `$spawn_out(slave,name)` fd, not the client's stdio: `stty rows 24
  columns 40 < $spawn_out(slave,name)`. Added scenario
  16-narrow-terminal-wrap.exp; passes on debian and alpine.

* **2026-05-07** — Task's `vars: VERSION: { sh: git describe ... }`
  unconditionally resolved via git describe, ignoring any env var.
  Real CI ergonomics bug: `VERSION=2.0.0 task release:dry-run`
  rendered npm packages with the commit hash, not 2.0.0. CI itself
  happens to work because tag-push checkouts produce a clean tree
  where `git describe` returns the tag — but locally, the documented
  usage was broken. Fixed `sh:` block to honour `${VERSION}` env
  first, fall back to git describe second. Resolved in this
  iteration's commit.

* **2026-05-07** — `plugin/zsh-history-enquirer.plugin.zsh` still
  contained the OLD bindkey-swap fallback even though the followups
  entry below claimed it was fixed. Real divergence between the doc
  and the code, found by re-reading both. Fixed: fallback uses
  `zle .history-incremental-search-backward` (no keymap mutation),
  and ^R is bound explicitly in emacs/viins/vicmd via `bindkey -M`.
  Added e2e scenario 15-vi-keymap.exp to lock the regression.
  Resolved in this iteration's commit.

* **2026-05-07** — Two anonymous `func() io.Writer` providers in fx
  failed dependency resolution silently. Introduced `Stdout` and
  `StderrWriter` named types in `internal/app/module.go`. Resolved in
  the lint-clean commit.

* **2026-05-07** — `os.File.SetReadDeadline` on `/dev/tty` is
  unreliable in docker's pty (reads block past the deadline). Replaced
  with `unix.Poll`. Resolved in the e2e-harness commit.

* **2026-05-07** — Leading-edge-only throttle dropped the trailing
  state of fast bursts (paste, fast-typed multi-char input). Added a
  `trailingFlush` timer in `internal/app/run.go`. Resolved in the
  e2e-harness commit.

* **2026-05-07** — `t.Size()` (TIOCGWINSZ) returned `(0, 0)` inside
  docker's pty when expect created the slave without SIGWINCH ever
  firing. `heightLimit` then clamped to 1 and PageDown was
  effectively a single arrow-down. Run() now falls back to 24x80 if
  `t.Size()` reports zero. Resolved in the
  test(e2e):+pageup/pagedown commit.

* **2026-05-07** — `--version` printed nothing in environments where
  `/dev/tty` is unusable (Claude Code's bash tool, CI without `-t`),
  because the eager `tty.NewDevTTY` provider failed and fx silently
  swallowed the start error. Detect `--version`/`-version` directly
  in `main.go` before `fx.New` runs. Resolved in the
  fix:short-circuit-version commit.

* **2026-05-07** — `--version` was writing to stderr. Per CLI
  convention (and so `zsh-history-enquirer --version | grep` works)
  it should go to stdout. The picker output and the version output
  are mutually exclusive, so reusing stdout for both is safe.
  Resolved in this iteration's commit.

* **2026-05-07** — Reader sub-goroutine could leak across the
  ctx-cancel boundary: a separate byte-reader goroutine blocked in
  `r.tty.Reader().Read(buf)` would not see ctx cancellation until
  the next byte arrived (typically when fx OnStop closed the fd).
  Replaced with a single goroutine that uses `unix.Poll` with a
  100ms timeout, so each iteration checks ctx, drains SIGWINCH and
  flushTimer, then polls for input. Resolved in this iteration's
  commit.

* **2026-05-07** — `internal/app/run.go` opened the `ZHE_DEBUG` log
  file twice: once for probe diagnostics with a synchronous Close
  and once for events with a deferred Close. On early-return code
  paths (probe error, invariant fail) the second deferred Close
  never fired. Consolidated to a single open + single defer at the
  top of Run(). Resolved in this iteration's commit.

* **2026-05-07** — Renderer did NOT reset SGR state between rows:
  a history entry containing an unterminated `\e[31m` would bleed
  red into every subsequent rendered row until the next frame.
  Added a belt-and-braces `\e[0m` after every entry. Resolved in
  this iteration's commit.

* **2026-05-07** — Picker had no token-match highlighting, even
  though the README's "multi-word fuzzy match" promise implied
  visible feedback. The legacy Node.js code computed match spans in
  `historySearcher.ts:choiceMessage` but returned the un-highlighted
  string by accident — so the legacy port shipped with this latent
  bug for years. Added `highlight()` in `internal/ui/render.go`
  that wraps every matched token in bold-cyan SGR. Property tests
  guarantee the un-highlighted payload is preserved exactly when
  the SGR escapes are stripped. Resolved in this iteration's
  commit.

* **2026-05-07** — `plugin/zsh-history-enquirer.plugin.zsh` fallback
  swapped the global `bindkey '^R'` mapping during the fallback
  call, which left inconsistent state in vicmd/viins keymaps. Use
  `zle .history-incremental-search-backward` to invoke the builtin
  widget directly without touching keymaps. Also bind to emacs,
  viins, and vicmd keymaps explicitly so the picker is reachable
  from every common zsh keymap. Resolved in this iteration's
  commit.

* **2026-05-07** — `End` semantics interacted subtly with the
  dynamic-limit math when multi-line entries reshuffled into the
  visible window after rotation. Rewrote `scrollToEnd` to walk
  Filter from the back, accumulating wrapped row counts until the
  height limit is hit; that gives the precise count of "tail"
  entries that fit, which is then used as the rotation amount and
  the new Idx. Unit tests in `internal/ui/model_test.go` exercise
  the regression case. Resolved in this iteration's commit.

* **2026-05-07** — Multi-line entry interactions had only one e2e
  scenario (scroll-past). Per user emphasis on
  "多行、多行换行以及多行换行交互" (multi-line, multi-line wrap,
  multi-line interaction), added three more: 09-multiline-submit
  (select + run a multi-line entry), 10-multiline-render-and-cancel
  (filter to a multi-line entry, verify all rows render, cancel
  cleanly), 11-multiline-scroll-into-view (arrow-down a multi-line
  entry into the visible window without breaking the renderer).
  Both targets pass 11/11. Resolved in this iteration's commit.

* **2026-05-07** — Architectural layering was enforced only by
  convention. Added `.go-arch-lint.yml` and `task lint:arch`. CI
  workflow installs go-arch-lint and runs the check. A future
  contributor who imports `internal/app` from `internal/ui` will
  now fail CI. Resolved in this iteration's commit.

* **2026-05-07** — `npm` shim echoed nothing back when the platform
  sub-package was missing. The widget's `BUFFER=$(...)` would then
  set BUFFER to an empty string — silently destroying the user's
  typed input. The shim now echoes argv back to stdout on the
  missing-binary path, matching the widget contract. Resolved in
  this iteration's commit.

* **2026-05-07** — Verified `act` works locally for the unit-test
  CI job (act 0.2.87 on macOS arm64 + Docker 29.4). The
  `go-task/setup-task@v2` step requires `node` in the runner image,
  which the default `node:slim` image lacks; the official guidance
  is to use `--container-architecture linux/amd64` with an image
  that keeps `node` available, e.g. `catthehacker/ubuntu:act-latest`.
  Documented in this iteration's commit; the Taskfile already
  carries act-based recipes ready to use once an image is
  configured in `.actrc`. Resolved in this iteration's commit.

* **2026-05-07** — CI workflow's `on: push: branches: '*'` was
  silently never triggering for the `refactor/golang/dev` branch
  because GitHub Actions' `*` glob in branch filters matches
  single-segment refs only. The first run after the branch was
  pushed to origin had to be discovered via `gh api .../events`
  showing pushes-without-runs. Fixed by switching to `**`. Without
  this, every "CI runs on push" promise in the README was
  unenforced. Resolved in this iteration's commit.

* **2026-05-07** — `golangci-lint v2.1.6` was built with Go 1.24
  and refused to load against go.mod's `go 1.25.0` (forced by the
  `go.uber.org/fx@v1.24` dependency). Bumped to `v2.12.2` in CI
  and dropped the redundant `toolchain go1.26.2` from go.mod.
  Resolved in this iteration's commit.

* **2026-05-07** — `internal/app/run.go` was ~280 lines doing
  configuration, fx orchestration, TTY setup, render loop, and
  debug logging in one function (architect agent's earlier
  flagged this). Split into init.go (geometry + cursor probe
  fallback), loop.go (event loop + trailing flush), debug.go
  (ZHE_DEBUG file open + structured loggers). Each file ≤ ~120
  lines; the new pure helpers (computeInitCol, clampCursor,
  handleProbeFallback) are individually unit-tested, lifting
  internal/app coverage 5% → 22%. Resolved in this iteration's
  commit.

* **2026-05-07** — Repo was missing CONTRIBUTING.md, CHANGELOG.md,
  ISSUE_TEMPLATE/, PULL_REQUEST_TEMPLATE.md — standard hygiene
  for a "world-wide collaboration" project. Added all four with
  prompts that nudge contributors toward project conventions
  (spec/design/plan workflow, multi-line e2e, conventional
  commits). Resolved in this iteration's commit.

* **2026-05-07** — The pnpm workspace lived under `npm-workspace/`
  but the user spec called for it under `npm/` ("npm 包管理使用
  pnpm workspace, workspace 放在 npm"). Renamed via `git mv`
  (history preserved) and updated every reference in
  pnpm-workspace.yaml, .gitignore, Taskfile.yml, docs/spec,
  docs/design, docs/plan, AGENTS.md, CONTRIBUTING.md, and
  scripts/release/build-npm.sh. Build + render-umbrella +
  per-platform packages reproduce the right shape after the
  rename. Resolved in this iteration's commit.

* **2026-05-07** — `recomputeFilter` aliased `m.Choices` whenever
  the input had no tokens (search.AndFilter's documented zero-copy
  fast path). The model's `rotateUp` / `rotateDown` mutate Filter
  in place, so scrolling the empty-input view scrambled Choices,
  and a subsequent `recomputeFilter` (e.g. after Ctrl-U) returned
  the rotated permutation rather than reverse-chronological order.
  Real user impact: after a few Up presses + a typo + Ctrl-U, the
  picker showed history out of order. Fixed by cloning the slice
  in `recomputeFilter` when tokens is empty (cost is one
  slice-header copy on the rare empty-input recompute, not on
  every keystroke). Pinned by `TestModel_RotateDoesNotMutateChoices`
  (direct invariant) and
  `TestModel_ScrollThenClear_RestoresChronologicalOrder`
  (user-visible scenario). Resolved in this iteration's commit.

* **2026-05-07** — Alt+Backspace (`\e\x7f`) was a UX footgun: the
  parser saw the lone Esc, dispatched it as KeyEsc which cancels
  the picker, and the trailing Backspace was orphaned. Mac/iTerm
  /GNOME Terminal users who reach for word-delete muscle memory
  (zsh's emacs-keymap default) had the picker drop them out
  every time. Fixed by handling `\e\x7f` and `\e\x08` in
  `feedEsc` as a single chord routed to KeyCtrlW (the existing
  word-delete path). Plain Esc → ...later... → Backspace still
  works because the reader's 50ms flushTimer separates them into
  distinct Feed calls. Pinned by `TestParser_AltBackspaceMapsToCtrlW`,
  `TestParser_PlainEscThenBackspaceStillCancels`, and e2e
  scenario 20-alt-backspace.exp (validated on both glibc and
  musl). Resolved in this iteration's commit.
