# Changelog

All notable changes to `zsh-history-enquirer` are recorded here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and the project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed (BREAKING — major version bump)

- **Complete rewrite from Node.js + TypeScript to Go.** The user-
  facing widget contract is unchanged (press <kbd>Ctrl</kbd>+<kbd>R</kbd>,
  pick, <kbd>Enter</kbd>); the binary is now a single static
  ELF/Mach-O with no runtime dependency on Node.
- **No more automatic `~/.zshrc` editing.** The npm postinstall hook
  prints a one-line `source` reminder; users wire the plugin in
  themselves. The legacy auto-edit caused subtle re-install loops
  and surprising diffs in dotfiles repos.
- **No more automatic oh-my-zsh plugin-list mutation.** Users either
  source the plugin file directly or symlink it into
  `$ZSH_CUSTOM/plugins/` themselves.

### Added

- Token highlighting in the rendered list (legacy intended this but
  shipped with a latent bug — `choiceMessage` returned the un-
  highlighted string). Bold-cyan SGR around every matched token.
- `--version`, `--histfile`, `--histsize`, `--max-limit` CLI flags
  for debugging and pinning behaviour.
- `act`-compatible local CI parity — `task ci:e2e:run` is the same
  recipe GitHub Actions runs.
- Static-linkage assertion in `scripts/build-all.sh` and CI's
  `build` job — Linux builds that accidentally pull in CGO fail
  loudly.
- 17 e2e scenarios in Docker (debian + alpine, two libcs) covering:
  basic pick, multi-line scroll, cancel-preserves-input, multi-word
  search, bracketed paste, PageUp/Down, Home/End, LBUFFER prefilter,
  multi-line submit + run, multi-line render-and-cancel, multi-line
  scroll-into-view, empty history, Unicode entries (CJK / accented
  / emoji), long-line wrap, vi-mode keymap, narrow-terminal wrap,
  in-picker input editing.
- Go-native fuzz target on `keys.Parser.Feed` — pinned via the
  test corpus, run for longer windows via
  `go test -fuzz=FuzzParser_NoPanicOnArbitraryBytes`.
- Property-based tests with `pgregory.net/rapid`:
  - `internal/history` — reverse-dedupe-unescape invariants.
  - `internal/search` — AND-filter monotonicity, every-match-
    contains-all-tokens.
  - `internal/ui/wrap` — wrapped row count monotonicity.
  - `internal/ui/highlight` — payload preservation under SGR strip.
  - `internal/keys/parser` — chunk-boundary invariance for the FSM.
- `.go-arch-lint.yml` — package layering enforced in CI.
- `NO_COLOR=1` opt-out for accessibility / non-color terminals.
  Conforms to the [no-color.org](https://no-color.org) convention:
  any non-empty value of `$NO_COLOR` suppresses ALL SGR escapes
  the picker would otherwise emit (the bold-cyan token highlight
  + the per-entry SGR reset). Search and filtering are unaffected;
  only the visual markup is removed.
- <kbd>Ctrl</kbd>+<kbd>W</kbd> deletes the previous word — matches
  zsh's default `backward-kill-word` keymap. Rune-aware so CJK /
  emoji words delete atomically.
- <kbd>Ctrl</kbd>+<kbd>P</kbd> / <kbd>Ctrl</kbd>+<kbd>N</kbd>
  aliases for ↑ / ↓. Power users with zsh's emacs-keymap muscle
  memory now have those keys reach the picker's row navigation
  instead of being silently dropped.

### Fixed (vs. legacy 1.x bugs that survived into the rewrite)

- `End` semantics now correctly land focus on the last match even
  when multi-line entries reshuffle into the visible window after
  rotation.
- Bracketed-paste payloads are emitted as a single `PasteEvent`
  rather than per-byte keystrokes; control bytes inside a paste no
  longer trigger handlers.
- DSR cursor probe uses `unix.Poll` instead of
  `os.File.SetReadDeadline`, which is unreliable on docker-allocated
  pty.
- Trailing-edge render flush after a paste / fast-typed burst —
  the legacy 72ms leading-edge throttle dropped the final frame.
- Reader goroutine no longer leaks past `ctx.Done()`; the
  byte-reader and event-dispatcher are now a single goroutine
  driven by `unix.Poll` with a 100 ms tick.
- **Vi-mode `^R` regression** — the legacy plugin only bound `^R`
  in the default keymap, so vi-mode users lost the picker after
  pressing Esc to enter `vicmd`. Now bound in emacs/viins/vicmd
  explicitly.
- **Plugin fallback no longer mutates keymaps** — uses
  `zle .history-incremental-search-backward` to invoke the builtin
  widget directly rather than swapping `bindkey '^R'` around the
  call (which left transient inconsistent state across keymaps).
- **`npm install` shipped a stale plugin file** — the npm umbrella
  source had its own copy of `plugin/zsh-history-enquirer.plugin.zsh`
  that wasn't kept in sync with the project root. Removed; the
  build script now copies from the project root each release.
- **Homebrew install was missing the plugin file** — the formula's
  `def install` only installed the binary, but the README's
  `source $(brew --prefix)/share/zsh-history-enquirer/plugin.zsh`
  pointed at a path that didn't exist. The formula now declares
  a `resource "plugin"` and stages it into pkgshare.
- **NPM LICENSE was a 2-line stub** — both umbrella and per-platform
  LICENSE files now ship the canonical MIT text, satisfying license-
  compliance scanners (Snyk / FOSSA / BlackDuck).
- **`BUFFER=$(...)` blanked user input on missing platform binary**
  — the npm shim now echoes argv back to stdout when no platform
  sub-package resolves, preserving the widget contract.
- **`VERSION=v2.0.0 task release:dry-run` ignored the env override**
  — Task's `vars: VERSION: { sh: ... }` always resolved via git
  describe. The local override now works.
- **Pre-release versions published under npm `latest`** — the
  release script ran `npm publish --access public` with no
  `--tag`, so a `v1.0.0-rc.1` would have replaced stable `v1.0.0`
  as the `latest` install. Pre-releases now correctly publish
  under `next`; stable under `latest`.
- **`task lint:sh` and `task lint:arch` silently masked real
  failures.** The `&& cmd || (echo "not installed")` chain
  routed real shellcheck / go-arch-lint violations to the "not
  installed" message and exited 0. Replaced with `if/else` so
  violations actually fail the task.
- **`BUFFER=$(...)` blanked user input on hard early errors in the
  binary itself.** The npm shim's missing-binary fallback was wired,
  but if the binary started, hit a raw-mode-enter or geometry-read
  failure, and returned `(nil, err)`, the umbrella `invokeRun` never
  printed anything to stdout — `BUFFER=$(...)` then resolved to the
  empty string and ate the user's typed `$LBUFFER`. Added
  `preserveOnError` which synthesizes a result from `cfg.Input` so
  the widget contract holds even on early-error paths.
- **`BUFFER=$(...)` also blanked input on fx-provider failures.**
  When `/dev/tty` was unopenable in a headless container, the fx
  provider for `tty.NewDevTTY` returned an error during `app.Start`
  — `invokeRun` never ran, so `preserveOnError` never had a chance
  to fire. Added a top-level safety net `recoverStartFailure` in
  `cmd/zsh-history-enquirer/main.go` that reconstructs `cfg.Input`
  via `app.NewConfig` and echoes it back to stdout. Three table
  tests pin the (preserves-input, no-args, malformed-argv) cases.
- **Modifier-key arrows (`\e[1;2A` Shift+Up, `\e[1;5A` Ctrl+Up,
  `\e[1;3A` Alt+Up, etc.) were silently dropped.** xterm encodes
  modified keys as `\e[1;<modifier><letter>` for arrows / Home /
  End and `\e[<row>;<modifier>~` for PgUp / PgDn / Delete. The
  parser only matched plain forms (`\e[A`, `\e[5~`), so any
  modified press fell through to the "unknown CSI; swallow" branch.
  A user pressing Shift+Up to "navigate up" saw nothing happen.
  Added `stripCSIModifier` that normalizes the modifier-encoded
  forms to their plain counterparts before dispatch (the picker
  has no per-modifier behavior anyway, so swallowing the modifier
  is friendlier than swallowing the press). 13 regression cases
  pin every reasonable modifier combo against arrows / Home / End
  / PgUp / PgDn / Delete; 6 passthrough cases pin that unrelated
  CSI sequences (DSR replies etc.) reach the dispatch unchanged.
- **Arrow keys via SS3 sequences (`\eOA` / `\eOB` / etc.) cancelled
  the picker.** Some terminals — xterm in DECCKM/application-keypad
  mode, certain VT-series emulators, embedded firmware terminals —
  send `\eO<key>` instead of the CSI form `\e[<key>` for arrow /
  Home / End keys. The parser only handled CSI; SS3 fell through to
  the "ESC + unrelated byte" branch, emitting `KeyEsc + RuneEvent
  'O' + RuneEvent <key>`. The picker would CANCEL on every arrow
  press in such a terminal (preserving input but never letting the
  user navigate). Added `stateSS3` + `feedSS3` covering A/B/C/D/H/F.
  `FlushEsc` also drains a hung SS3 prelude. Three regression tests
  pin the arrow / unknown-byte fallback / flush behaviors.
- **Picker session > 5 seconds crashed with "context deadline
  exceeded".** `fx.StartTimeout(5*time.Second)` and
  `context.WithTimeout(..., 5*time.Second)` in `main.go` were
  intended as "fail fast on stuck startup" guardrails, but
  `invokeRun` runs the entire picker session synchronously inside
  `OnStart`. A real interactive session (slow human, multiple
  keystrokes, paste, step away for coffee) routinely exceeds 5s.
  The fx layer would then cancel the picker context, propagate
  `context.DeadlineExceeded`, and fall through to
  `recoverStartFailure` — terminal in raw mode, picker frame
  half-erased, BUFFER set to argv echo. Regression caught by e2e
  scenario 19 (Ctrl-W word delete) which has just-barely-over-5s
  sleep totals. Bumped both timeouts to 1 hour — the picker is
  bounded by user attention, not a fixed wall clock; SIGINT /
  SIGTERM / SIGHUP still tear it down via the
  `signal.NotifyContext`-wrapped `runCtx`.
- **Builds embedded the contributor's absolute filesystem paths.**
  `go build` without `-trimpath` writes the absolute build-time
  source path into the produced binary's symbol table. For an
  open-source release this leaks the maintainer's `$HOME` layout
  and breaks per-build reproducibility (different developers
  building the same commit produce binaries with different bytes).
  Added `-trimpath` to every `go build` invocation in
  `Taskfile.yml` and `scripts/build-all.sh` — release artifacts now
  contain only repo-relative paths and are bit-reproducible
  modulo Go's BuildID.
- **npm shim's missing-binary fallback echoed back `--` separator.**
  After the plugin started passing `bin -- "$LBUFFER"`, a fall-through
  invocation (no platform binary present) ran
  `argv.slice(2).join(' ')` which produced `-- $LBUFFER` instead of
  just `$LBUFFER`. `BUFFER=$(...)` then captured `-- typed text`
  instead of just `typed text`. The shim now strips a leading `--`
  before echoing. Smoke test 3/4 in `task release:smoke` covers
  both the direct-invocation form (no `--`) and the widget-mode
  form (with `--`) so a regression is caught at release time.
- **Multi-line paste corrupted the picker layout.** The bracketed-
  paste handler appended the payload verbatim to `m.Input`, and the
  renderer wrote `m.Input` straight to the TTY at the captured
  prompt column. A `\r` in the payload would carriage-return into
  the prompt prefix; a `\n` would push the picker rendering down by
  one row, leaving stale cells; a `\t` would jump to the next
  tabstop. None are useful as filter input. Added
  `sanitizeInputRune` / `sanitizeInputString` that map `\n` / `\r` /
  `\t` to space — the token separator so multi-word AND-search
  still treats `git\nlog` as `git log`. Five regression cases pin
  the paste path; one pins the per-keystroke path.
- **`Backspace` corrupted multi-byte UTF-8 input.** Backspace was
  doing `m.Input = m.Input[:len(m.Input)-1]` — slicing one BYTE off
  the end. For ASCII this works because every char is 1 byte. For
  CJK / emoji / accented Latin (all ≥2 bytes in UTF-8), it left a
  trailing continuation byte and corrupted the input into invalid
  UTF-8. Switched to `utf8.DecodeLastRuneInString` so Backspace
  deletes one *rune*. Six regression cases pin the bug.

- **`$LBUFFER="--version"` (or `--help`, `-h`) silently destroyed
  user input.** When the user typed a flag-shaped string at the
  prompt and pressed `^R`, the widget shelled out as
  `zsh-history-enquirer "$LBUFFER"`, the binary saw `args=["bin",
  "--version"]` matching `isVersionFlag`, fast-pathed to the version
  print, and `BUFFER=$(...)` resolved to the version string instead
  of opening the picker with `--version` as the filter. The plugin
  now passes a `--` separator (`bin -- "$LBUFFER"`) so widget-mode
  invocations are always 3-arg, never trigger the fast-path, and
  Go's `flag.Parse` correctly treats anything after `--` as
  positional. Two new isVersionFlag / isHelpFlag test cases pin
  the widget-mode shape so a future plugin edit that drops the `--`
  can't re-introduce the bug silently.
- **`--help` printed help, then a confusing "startup failed" error.**
  `flag.Parse` returns `flag.ErrHelp` on `-h` / `--help`, which
  bubbled up through fx as a provider error. The user got the help
  text (good) followed by a stack-trace-shaped error (bad). Added
  an `isHelpFlag` fast-path in `main.go` that calls a new
  `app.PrintHelp(os.Stdout)` helper before fx wires up — clean help
  text, exit 0, no spurious error. A drift-detection test
  (`TestPrintHelp_MatchesNewConfigFlags`) keeps PrintHelp's listed
  flags in sync with NewConfig's runtime parser.
- **Highlighter produced invalid UTF-8 on Unicode case-fold mismatches.**
  `highlight()` did `lc := strings.ToLower(s)` and then sliced `s`
  with byte indices computed against `lc`. For most input this is
  fine — ASCII A-Z folds 1:1. But for runes whose case-fold changes
  byte length (Turkish capital `İ` → `i` shrinks 2→1; some
  expansions grow), `lc[idx:]` and `s[idx:]` no longer point to the
  same character boundary, and slicing `s` with `lc`'s byte indices
  emits broken `\xb0...` mojibake to the terminal. Added a length
  check: when `len(lc) != len(s)` the highlighter falls back to
  returning the original string unhighlighted (search-match behavior
  is unaffected because `search.AndFilter` only checks
  `Contains(lc, t)`). Two regression tests pin the Turkish-İ case.
- **External kill mid-render left the terminal in raw mode.**
  `Run()` was passed `context.Background()`, which never canceled
  on SIGTERM / SIGHUP. If the user (or another process) sent
  `kill -TERM <pid>` while the picker was open, the process was
  torn down before fx ran the TTY OnStop hook — leaving the
  terminal stuck without echo / canonical input until the user ran
  `stty sane`. `invokeRun` now wires SIGINT / SIGTERM / SIGHUP
  through `signal.NotifyContext` so the event loop's `<-ctx.Done()`
  case fires, the cancel path runs, the TTY hook restores termios,
  and the user's input is preserved. (SIGINT from Ctrl-C inside the
  picker still arrives as the byte `0x03` because raw mode disables
  ISIG; the new wiring only matters for external kills.)
- **`Alt+Backspace` cancelled the picker.** macOS Terminal, iTerm2,
  GNOME Terminal — every xterm-style terminal — sends the chord as
  `\e\x7f`. The parser saw the lone Esc, dispatched `KeyEsc` (which
  cancels the picker), and the trailing `\x7f` was orphaned. Shell
  users who reach for word-delete muscle memory mid-search were
  dropped out of the picker every time. Fixed in `feedEsc`: when
  the byte after `\e` is `\x7f` or `\x08`, route directly to
  `KeyCtrlW` (the existing word-delete path). Plain Esc → ...later...
  → Backspace still cancels because the reader's 50ms `flushTimer`
  separates them into distinct Feed calls. Pinned by a parser unit
  test, a slow-path-still-cancels guard, and e2e scenario 20 on
  both glibc / musl.
- **Filter rotations scrambled the immutable `Choices` slice.**
  `search.AndFilter` aliases `Choices` when the input has no tokens
  (a documented zero-copy fast path). `recomputeFilter` then
  assigned that aliased slice to `m.Filter`, which is later rotated
  in place by `rotateUp` / `rotateDown`. So every Up arrow on the
  empty-input view scribbled the rotation through into the
  user-supplied `Choices`, and a subsequent `recomputeFilter` (e.g.
  after Ctrl-U) returned a permuted history rather than reverse-
  chronological order. Fixed by cloning the filter slice in
  `recomputeFilter` whenever tokens is empty.
- **UTF-8 resync dropped valid bytes alongside invalid ones.** When
  decode failed and the internal buffer reached `utf8.UTFMax`, the
  parser dropped the *entire* 4-byte buffer. So feeding
  `[0xbd, 'a', 'b', 'c']` (a stray continuation byte then ASCII)
  silently swallowed the user's `abc` along with the bad byte.
  Real-world triggers: terminal locale mismatch, SSH bit corruption,
  paste payload that begins with a partial UTF-8 sequence. Fixed
  with one-byte-at-a-time resync: drop only the invalid lead byte
  (using a strict `isValidUTF8Lead` helper that's stricter than
  `utf8.RuneStart` — rejects 0xc0/0xc1 and 0xf5-0xff which Go's
  stdlib accepts) and re-attempt decode on the rest.
- **Reader's flush timer didn't arm on `stateSS3`.** The parser's
  `FlushEsc` already handled both `stateEsc` and `stateSS3`, but
  the reader only armed its 50ms timer for `stateEsc`. Net effect:
  a terminal that sent `\eO` and then nothing (rare on flaky links
  / embedded firmware) froze the picker — no events surfaced until
  some unrelated byte unstuck the SS3 sequence.
- **Non-ASCII LBUFFER mis-aligned the picker against the prompt.**
  The init-column arithmetic and `m.Cursor` updates were using
  `len()` (bytes) where the underlying CSI sequences expect cells.
  For accented Latin / Greek / Cyrillic / Hebrew / Arabic / CJK /
  emoji, byte-count over-counted display width by 1.5–3×, so the
  picker drew at the wrong column whenever LBUFFER had any
  non-ASCII content — visibly mis-aligning against the prompt and
  parking the caret in empty space after the input row. Now goes
  through `ui.CellWidth` (mattn/go-runewidth, the same library every
  Charm / bubbletea TUI uses), East Asian Width-aware so CJK
  ideographs and emoji each contribute their actual 2 cells.
- **`splitNonEmptyLines` was a misnomer, letting empty entries
  through.** A corrupt $HISTFILE (with `echo "" >> ~/.zsh_history`,
  partial-write blank lines, or CRLF-only blank lines that
  collapsed to "" after the trailing-CR strip) produced empty
  entries the picker rendered as blank rows. The user could
  navigate to one and accidentally hit Enter, blanking $BUFFER.
  The function now actually drops empty lines.
- **CRLF in $HISTFILE scrambled the picker frame.** A history file
  saved on Windows (or by a misconfigured editor) left a `\r` at
  the end of every entry. When rendered, the `\r` carriage-returned
  the cursor back to col 1 mid-frame, and the next entry's pointer
  overwrote the previous entry's last byte — entries appearing to
  mash together. `splitNonEmptyLines` now strips trailing `\r` from
  each line before emitting it.
- **`$LBUFFER="--print-install-hint"` blanked user input.** The npm
  shim used `process.argv.includes('--print-install-hint')` to
  trigger the postinstall hint. Same widget-contract footgun as
  the `--version` / `--help` cases: a $LBUFFER literally equal to
  the flag name tripped the fast-path, wrote the hint to stderr,
  and exited 0 with empty stdout — silently destroying the user's
  typed text. Narrowed to `argv.length === 3 && argv[2] === ...`,
  matching the discipline used on the Go side. Three node:test
  scenarios pin it.
- **Raw control bytes in history entries could disrupt the picker
  frame.** A corrupt or maliciously-appended `$HISTFILE` entry
  containing e.g. `\x1b[2J` (clear-screen) would let the embedded
  ESC reach the terminal during render. Added
  `sanitizeChoiceForRender` in the renderer that replaces 0x00–0x1f
  (except `\t` / `\n`) and 0x7f with caret notation. Sanitization
  is render-only — `m.Filter[i]` keeps the original bytes so
  `SubmitResult` returns the literal command for re-execution.
- **`sanitizeInputRune` only filtered `\n` / `\r` / `\t`.**
  Bracketed-paste payloads deliberately preserve embedded control
  bytes (so `\x03` doesn't fire CtrlC inside a paste), so a
  clipboard with `\x1b[2J` would land in `m.Input` verbatim and
  the renderer would let the ESC clear the screen mid-frame.
  Extended the sanitizer to map every C0 byte (0x00–0x1f) and
  0x7f to space — matching the long-standing `\n` / `\r` / `\t`
  behaviour.
- **`NewModel(input, ...)` stored argv directly into `m.Input`.**
  Third entry-point sealed: a power user who pressed Ctrl-V Ctrl-[
  to insert a literal ESC into LBUFFER before Ctrl-R, or a hostile
  clipboard auto-pasted before invocation, would hand the picker
  a `cfg.Input` carrying raw control bytes. NewModel now runs
  input through `sanitizeInputString` before storing it.
- **Extended-history entries with empty commands surfaced as
  blank picker rows.** `: 1700000001:0;` (no command after the
  semicolon) survived `splitNonEmptyLines` (the line itself is
  non-empty), then `stripExtendedHistoryPrefix` reduced it to "".
  `fixtureLoader.Load` now drops post-strip empties symmetrically.
- **DSR cursor probe silently swallowed user input typed during
  the probe window.** Fast typists pressing `^R git` lost the first
  1–4 keystrokes. The probe loop's `parseDSRResponse` anchored on
  the first `[`, throwing away any prefix bytes. Fix: anchor on
  `\x1b[`, tighten the loop break condition to `\x1b[<...>R`, and
  extend `Probe.Cursor`'s signature to return leftover bytes on
  the success path (the timeout path already used
  `TimeoutError.Leftover`).
- **Reader's main event loop exited on EINTR from `unix.Read`.**
  The poll path was already EINTR-resilient, but the read syscall
  itself can return EINTR when a signal arrives between poll
  returning POLLIN and the read syscall completing — most commonly
  SIGWINCH from a mid-Ctrl-R terminal resize. `if rerr != nil ||
  n == 0 { return }` then closed the events channel and tore down
  the picker on every resize. Branched the rerr check so EINTR
  becomes `continue`. The same fix applied symmetrically to the
  cursor probe.
- **npm shim silently blanked BUFFER when `spawnSync` failed.**
  When `result.error` is set (the child could not be spawned at
  all — ENOENT, EACCES, ETXTBSY, stale symlink, lost +x bit), the
  shim exited 0 with no stdout. `BUFFER=$(cli.js -- "$LBUFFER")`
  then landed as empty, destroying the user's typed text on every
  Ctrl-R. Refactored the BUFFER-preservation echo into an
  `echoArgvAndExit()` helper and wired the `result.error` branch
  to call it after a stderr diagnostic.
- **A panic in the keys reader goroutine still crashed the process.**
  The top-level `defer recoverPanic` in `main()` only catches panics
  on the main goroutine; a panic inside `Reader.Events`'s read loop
  would terminate the process before reaching the recover. Added a
  dedicated `recoverGoroutinePanic` deferred at the top of the reader
  goroutine; on panic it logs to `PanicWriter` (stderr by default)
  and returns normally — the deferred `close(out)` then signals the
  main loop to exit cleanly with `Canceled=true`, which echoes
  `m.Input` so BUFFER survives. 1 regression test pins the recovery.
- **A goroutine panic during the picker session would blank `BUFFER`.**
  The existing `recoverStartFailure` only fires when fx.App.Start
  errors before invokeRun runs — a runtime panic later in the picker
  (a bug in update.go / render.go, or a third-party panic from
  x/ansi / uniseg) would crash the process with no stdout output, so
  `BUFFER=$(...)` resolved to empty and the user's typed `$LBUFFER`
  was lost. Added a top-level `defer recoverPanic(...)` in `main()`
  that prints the panic + stack to stderr (invisible to `$(...)`)
  and echoes argv back to stdout so BUFFER survives even on the
  crash path. 3 tests pin the recovery flow + the stack helper.
- **Mid-pick SIGWINCH left stale wrap rows visible until the next
  keystroke.** Most terminals reflow wrapped content on resize, so the
  previous frame's `PrevSize` / `PrevCursorRow` no longer matched the
  physical row positions and the row-by-row erase missed reflowed
  leftovers. Added `Model.NeedsFullErase` flag (set by the resize
  handler in `update.go`); on the next render, `Pre` walks back to
  row N and emits `\x1b[J` (EraseScreenBelow) to wipe everything
  below at once, skipping the per-row walk-down (the broader erase
  already cleared the picker's area). Three bytes per WINCH burst.
  Two regression tests cover the model side and the renderer side.
- **CJK glyph at a wrap boundary positioned the caret 1 col off.**
  The closed-form cursor formula assumed contiguous cell packing, but
  every common terminal soft-wraps a 2-cell glyph that meets a 1-cell
  remainder at the right margin. `InputCursorPosition` now walks the
  input rune-by-rune, simulating the wrap exactly. Same change fixes
  a latent bug where `Render` sliced `m.Input[:m.Cursor]` to compute
  cells-before-cursor — but `m.Cursor` is a cell count, not a byte
  offset, so the slice mis-cut multi-byte runes.
- **`Frame.Size` ignored input row wraps; long inputs misplaced the
  caret and leaked stale wrap rows.** When the picker's input row
  exceeded the terminal width (a long argv prefilled by the zsh widget,
  or a long filter typed live) the renderer's bookkeeping only counted
  choice rows. `renderPost` then emitted `CursorToCol(initCol+m.Cursor)`
  which terminals clamped at the right margin, so the caret landed on
  the wrong row, and the next `renderPre` walked down from the wrong
  starting row and erased too few lines. Fixed by introducing
  `InputCursorPosition` / `InputExtraRows` helpers
  (`internal/ui/wrap.go`), tracking `inputExtra` + a new
  `Frame.CursorRow` / `RenderOptions.PrevCursorRow` round-trip in
  `internal/ui/render.go` and `internal/app/loop.go`, and aligning
  `scrollToEnd` (`KeyEnd`) with the same wrap-aware budget. New
  `choiceHeightLimit` shared between `renderBody` and `scrollToEnd`
  prevents future drift. 23 new test cases + e2e scenario
  21-input-wrap-edit (debian + alpine, 21/21 pass).
- **Parser's `stateCSI` could get stuck.** A flaky terminal
  sending `\e[` and stopping (a kill mid-sequence, programmatic
  input that paused, or a very real network glitch over ssh)
  left the parser in stateCSI forever. Worse, every subsequent
  typed byte was silently consumed by the CSI accumulator: any
  byte in 0x40..0x7e terminates the sequence as unrecognized
  and is dropped. `FlushEsc` now resolves stateCSI by emitting
  `Esc + '[' + each accumulated parameter byte as a Rune`; the
  reader arms its 50ms flush timer for stateCSI alongside
  stateEsc / stateSS3.

### Internal — replace hand-rolled libs with community standards

- **`internal/ansi` deleted; consumers now use `charmbracelet/x/ansi`
  v0.11.7.** The 100-line bespoke escape-string emitter has been
  replaced with the same library bubbletea / lipgloss use. Byte
  output is identical (or equivalent — `\x1b[K` vs `\x1b[0K` mean the
  same thing) so e2e expectations are unchanged. The `n=1` form of
  `CursorPreviousLine` shifted from `\x1b[1F` to `\x1b[F` (terminals
  treat both as "one line up") — three test assertions updated to
  match.
- **`mattn/go-runewidth` swapped for `rivo/uniseg` v0.4.7** in the
  cell-width helpers. uniseg iterates Unicode grapheme clusters so a
  decomposed `é` (`e + combining-acute`) and emoji ZWJ families like
  the family pictograph are measured as their rendered cell footprint
  rather than as the sum of their constituent runes. The `CellWidth`
  helper, `rowCellWidth` in `WrappedRowCount`, and the rune-walking
  `InputCursorPosition` all drive the cursor off cluster widths now.
  Common cases (ASCII, CJK, single-char emoji) produce identical
  widths to runewidth, so no test expectations needed updating.

### Distribution / release process

- **`scripts/release/build-npm.sh --publish` is now idempotent.**
  A transient npm registry hiccup mid-publish previously left the
  registry in a half-released state (some platform packages
  published, umbrella missing) — the retry then tripped on
  EPUBLISHCONFLICT for the already-published versions. Added a
  `publish_if_new` helper that calls `npm view <pkg>@<ver> version`
  first and skips if the exact-version is already on the registry.

### Added (ergonomics + correctness)

- **`NO_COLOR` env-var support** ([no-color.org](https://no-color.org)
  convention). Setting `NO_COLOR` to any non-empty value disables
  the bold-cyan token highlighting; filtering / matching are
  unaffected.
- **`Alt+Backspace` → word-delete.** Mirrors zsh's emacs-keymap
  default. Listed in the keybindings table.
- **CJK / emoji-aware rendering** via `mattn/go-runewidth` —
  initial-column math, wrap math, cursor placement all use real
  terminal cell counts.
- **Env-var documentation in `--help`.** The `Environment:` section
  lists `HISTFILE`, `NO_COLOR`, `ZHE_DEBUG` so users discover
  configuration knobs without spelunking the README.

### Distribution

- `npm install -g zsh-history-enquirer` — esbuild-style with four
  `@zsh-history-enquirer/<os>-<arch>` `optionalDependencies`.
- `brew install zthxxx/tap/zsh-history-enquirer`.
- Raw GitHub Release binaries with `checksums.txt`.

## [1.3.1] - 2022-01-23

The last Node.js release. See git history on the `master` branch
for prior changes.
