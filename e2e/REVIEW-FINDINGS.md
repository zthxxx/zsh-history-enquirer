# PoC Review Findings (golang-pro lens)

Review of `e2e/harness/*.go` + `e2e/scenarios/basic_pick_test.go`
after PoC verification on debian + alpine. Done in-coordinator
context (the dispatched reviewer agent stalled at ~7 min without
producing output; took over with the golang-pro skill).

## Must-fix before scaling to 24 scenarios

### F1. Promote `waitFor` into the harness as `Session.WaitFor`
- **Where:** `e2e/scenarios/basic_pick_test.go:103-113`
- **Why:** Every migrated scenario will need predicate-driven waits
  (picker-closed, output-appeared, prompt-returned). Inline helpers
  per-test will produce 24× duplicated code with subtly different
  bug surfaces. The screen-state poll is also a clear `Session`
  responsibility — it already owns the parser and the artifact log.
- **Fix:**
  ```go
  // in harness/session.go
  func (s *Session) WaitFor(
      label string,
      timeout time.Duration,
      pred func(Screen) bool,
  ) {
      s.t.Helper()
      deadline := time.Now().Add(timeout)
      for time.Now().Before(deadline) {
          if pred(s.Screen()) {
              s.artifacts.logEvent("waitfor.hit", label)
              return
          }
          time.Sleep(20 * time.Millisecond)
      }
      scr := s.Screen()
      s.artifacts.logEvent("waitfor.miss", label)
      s.t.Fatalf("WaitFor(%q) timed out after %s\nscreen:\n%s",
          label, timeout, scr.Dump())
  }
  ```
  Then delete `waitFor`, `countOccurrences`, `indexFrom` from
  scenarios. Replace `countOccurrences(scr.Dump(), "ok") >= 2` with
  `strings.Count(scr.Dump(), "ok") >= 2`.

### F2. Replace hand-rolled `countOccurrences` / `indexFrom` with stdlib
- **Where:** `e2e/scenarios/basic_pick_test.go:115-141`
- **Why:** `strings.Count` and `strings.Index` are stdlib, well-tested,
  and faster. The hand-rolled versions add 27 lines of cognitive load
  for zero benefit. Goes against "MUST DO: Use idiomatic Go".
- **Fix:** drop both functions; `strings.Count(s, substr)` is a 1:1
  replacement.

### F3. `Opts.SeedHistory` leaks container-internal paths
- **Where:** `e2e/harness/session.go:36-40, 90`
- **Why:** Callers wanting a custom fixture (scenarios 13, 14, 21, 24,
  25 per the manifest) currently must write
  `Opts{SeedHistory: "/seed/history/multiline-stick.history"}`. That
  string couples 24 future tests to the container mount layout. If
  `runner.sh` ever changes the mount root, every scenario edits.
- **Fix:** rename to `Opts.HistoryFixture string` (a logical name
  like `"multiline-stick"`); the harness resolves it to
  `/seed/history/<name>.history`. Empty stays the default
  `"seed"`. Document the convention in the Opts doc comment.

### F4. `Opts.PreFilter` for scenario 08 (filter-from-LBUFFER)
- **Where:** `e2e/harness/session.go:26-55` (Opts struct)
- **Why:** Scenario 08 (`08-prefilter-from-lbuffer.exp`) types text
  BEFORE pressing ^R so the picker opens with that text as initial
  filter (the `--$LBUFFER` arg path in plugin.zsh:29). The current
  harness has no helper for this — every migration site would
  duplicate `s.Type("git log"); s.SendCtrlR(); ...` which is fine but
  loses the documented contract that "pre-filter is a thing".
- **Fix:** add `Opts.PreFilter string` whose value is typed BEFORE
  `^R` during `NewSession`'s setup, *after* the initial prompt
  settle. Or: add `Session.OpenPickerWithFilter(filter string)`
  helper. Either works — pick one and use it consistently.

### F5. `WaitExit` is racy with `cleanup()` calling `cmd.Wait` twice
- **Where:** `e2e/harness/session.go:449-458` (WaitExit) and
  `e2e/harness/session.go:475-484` (cleanup)
- **Why:** `WaitExit` does `s.cmd.Wait()` in a goroutine. `cleanup`
  does `s.cmd.Process.Wait()` (different method, also blocking).
  When both fire, the second one returns
  `os.ErrProcessDone` or similar — currently ignored via `_, _ =`,
  but it's a subtle bug that will bite under `-race` once we add
  multi-test sessions in phase 2.
- **Fix:** mark `cmd.Wait` as a one-shot. Either:
  (a) gate it on a sync.Once and store the result, or
  (b) move it entirely into `cleanup` and have WaitExit poll
  `s.exitErr.Load()`. Option (b) is cleaner because `t.Cleanup`
  runs deterministically last.

## Should-fix before review-fix phase closes

### F6. `moduleRoot()` is fragile — use `runtime.Caller` instead
- **Where:** `e2e/harness/session.go:209-229`
- **Why:** Walks up to 8 dirs looking for `go.mod` whose contents
  match `/e2e`. If the worktree is renamed or this code is
  vendored, the walk breaks. The function is only used for the
  artifacts path — failing it silently falls back to `cwd`, which
  inside the docker container is `/home/tester` → artifact dump
  goes to `/home/tester/_artifacts/...` (lost on container exit).
- **Fix:** use `runtime.Caller(0)` to locate the source file, then
  `filepath.Dir(filepath.Dir(file))` to get the e2e root.
  Or, since we always run from `task ci:e2e:v2:run`, hard-code the
  artifacts dir to `/_artifacts/<TestName>` and bind-mount it. The
  former is simpler.

### F7. `WaitQuiescent` polls with `time.Sleep(10ms)` — burns CPU
- **Where:** `e2e/harness/session.go:392-412`
- **Why:** Tight 10 ms polling on `lastActivity` runs even when
  nothing happens. 100 polls/sec × 25 scenarios × 2 libcs = ~5000
  poll cycles per CI run. Per-poll cost is small but the pattern is
  un-Go-ish. The reader goroutine already has the activity timestamp
  → publish it via a `chan struct{}` notification instead.
- **Fix:** replace polling with a notify-on-activity broadcast.
  Sketch:
  ```go
  // in Session
  activityCond *sync.Cond  // signaled by readLoop on any byte
  // WaitQuiescent waits on activityCond with a deadline
  ```
  Or simpler: keep poll but raise interval to 25 ms — same
  semantics, fewer wakeups.

### F8. `SendCtrlR / SendEsc / SendEnter` are 1-line wrappers over `SendKey`
- **Where:** `e2e/harness/session.go:314-329`
- **Why:** Three named methods that just call `s.SendKey(KeyX)` add
  API surface without expressiveness. Migration scenarios that
  send unusual keys (Alt-Backspace, F1, PageUp) will call `SendKey`
  anyway, so the inconsistency forces readers to remember which
  keys have a wrapper.
- **Fix:** keep `SendKey(Key)` as the canonical API; delete the
  three wrappers. Tests become `s.SendKey(harness.KeyCtrlR)` —
  uniform and 1 char longer.
  *(Counter-argument: SendCtrlR reads naturally. Acceptable to keep
  these three as a deliberate ergonomic concession — but document
  it.)*

### F9. The picker's startup warning embeds in scrollback — assertions can match it
- **Where:** observation from `_artifacts` dumps + manual test runs
- **Why:** Inside docker the "warning: DSR cursor probe failed"
  banner is always present on row 0. A future scenario that asserts
  `screen.Contains("warning")` would silently match. Need a clean
  way to scope assertions past the warning row.
- **Fix:** add `Session.ResetScrollback()` that calls
  `term.Write([]byte("\e[2J\e[H"))` (clear screen + cursor home).
  Tests that need a clean baseline call it AFTER the initial
  `Settle()`. Alternatively: emit a known sentinel row before
  starting the test (`s.Type("echo --START--\r"); s.WaitFor(...)`).

## Nice-to-have (defer)

### F10. Add `go test -race` mode for the harness itself
- The harness compiles for `linux/amd64` and runs in docker without
  `-race` (the test binary is `go test -c` without flags).
- For local development, allow `task build:e2e-harness:race` that
  embeds `-race` so concurrency bugs in the reader+wait+snapshot
  paths get caught early.
- Note: `-race` requires CGO, so the resulting binary is not the
  release-grade static binary. Acceptable for the dev-mode target.

### F11. `events.jsonl` has no schema version
- Adding a `"schema":"v1"` field to each event lets future tools
  parse it confidently across changes. Trivial to add.

### F12. `cleanup()` silently swallows mkdir errors
- `artifacts.go:82-85` writes to stderr but continues. Acceptable,
  but a comment explaining why ("never want to mask the real test
  failure") would help maintenance.

## Verification I ran

```sh
$ cd /Users/zthxxx/Project/Node/zsh-history-enquirer-2/.claude/worktrees/e2e-modernize
$ task lint:no-test-deps
no test-only modules in main dep graph        # F: PASS

$ find . -name go.work -maxdepth 2
(empty)                                       # F: PASS

$ task test:e2e:v2:one TARGET=debian
--- PASS: TestBasicPick (0.93s)               # F: PASS

$ task test:e2e:v2:one TARGET=alpine
--- PASS: TestBasicPick (0.91s)               # F: PASS

$ task record:examples TAPE=01
e2e/tapes/out/01-basic-pick.mp4 (8.1 KB)   # F: PASS
e2e/tapes/out/01-basic-pick.gif (6.3 KB)   # F: PASS
```

## What the PoC got right

- **Module isolation**: separate `e2e/go.mod` + no `go.work` + the
  belt-and-braces `lint:no-test-deps` task is exactly the constraint
  the user asked for. Verified empirically.
- **`Screen` interface**: read-only snapshot pattern via
  `snapshotVT(t.Lock/Unlock)` is the right concurrency shape for
  test code reading from a parser fed by a background goroutine.
- **Artifacts on failure only**: `t.Cleanup` + `t.Failed()` gate
  means a green test leaves no detritus, a red test leaves a full
  reproduction. Mirrors what production debug tools do.
- **DSR-probe-tolerant timing**: the `400ms / 5s` quiescent for the
  first ^R correctly anticipates the always-fires 250 ms cursor
  probe timeout that legacy `docs/plan/20-followups.md:38-44`
  documented.
- **Docker mount surface**: `runner.sh` plus the `ci:e2e:v2:run`
  task target keeps the container hermetic — only the picker
  binary, plugin, fixtures, and harness.test are visible. No host
  $HOME path appears anywhere.

## Bottom line

**5 must-fix, 4 should-fix, 3 nice-to-have.** None of the must-fixes
require structural rework; they're API tightening (F1/F2/F3/F4) and
one small concurrency cleanup (F5). The next iteration can apply
F1-F5 in a single ~30-min pass, then proceed to migrate scenarios
02-25. F6-F9 fit naturally into the migration loop's per-batch
review windows. F10-F12 are post-migration polish.

**Migration can proceed in parallel with F6-F12** once F1-F5 land.
F1 in particular is required because every migrated scenario will
want `s.WaitFor` — implementing it inline in 24 test files is
unmaintainable.
