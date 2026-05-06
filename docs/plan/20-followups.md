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
