# plan/20-followups — discovered during execution

Items added here come from doing the work, not from up-front planning.
Each entry must include:

- **Date** in `YYYY-MM-DD`
- **Why** — what surfaced it
- **Where** — file/line if applicable
- **State** — `open` / `addressed in <commit>` / `wontfix because <reason>`

## Open

* **2026-05-07** — `expect` cannot reliably pass an `stty rows N cols N`
  to the spawned process across hosts. The "narrow-terminal multi-line
  wrap" scenario was deferred from `e2e/scenarios/`. The renderer's
  wrap math is exhaustively unit-tested in `internal/ui/wrap_test.go`,
  which limits the gap, but a real e2e wrap scenario remains nice-to-
  have. Why open: would catch a class of bug where wrap math diverges
  from what the terminal actually does. How to apply: revisit when
  swapping `expect` for `tmux send-keys` or a Go-based pty driver.

* **2026-05-07** — Initial post-load DSR cursor probe always falls back
  inside docker (expect's pty doesn't reply). Production terminals do
  reply; the user-facing impact is "first run looks fine." The fallback
  draws starting at column 1 instead of inline at the prompt column.
  Why open: when running in a real terminal the probe works, so this
  only matters for tests. How to apply: don't try to fix it for
  expect — the renderer's correctness is verified at the model layer.

(no current open items beyond the two pty-related ones above)

## Addressed

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
