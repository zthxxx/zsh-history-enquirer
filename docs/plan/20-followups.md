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

* **2026-05-07** — `End` semantics interact subtly with the dynamic-
  limit math when multi-line entries are involved. The current model
  (matching the legacy Node.js port) rotates by the *previous*
  render's limit and sets `Idx = limit - 1`. If the post-rotation
  visible window contains a multi-line entry, the renderer's
  recomputed limit can shrink, which clamps `Idx` to `newLimit - 1`
  and the focus may not land on the actual last match. Why open: the
  legacy port shipped with this same behavior and users have lived
  with it for years; fixing it is a UX refinement, not a regression.
  How to apply: rewrite `scrollToEnd` to do an iterative rotation
  search that produces a layout where the last filtered entry sits
  at the bottom of the eventual visible window. E2E scenario 07 has
  been narrowed to assert `Home` only; an `End` follow-up scenario
  should be added once the model is fixed.

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
