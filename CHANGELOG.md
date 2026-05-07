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

### Distribution

- `npm install -g zsh-history-enquirer` — esbuild-style with four
  `@zsh-history-enquirer/<os>-<arch>` `optionalDependencies`.
- `brew install zthxxx/tap/zsh-history-enquirer`.
- Raw GitHub Release binaries with `checksums.txt`.

## [1.3.1] - 2022-01-23

The last Node.js release. See git history on the `master` branch
for prior changes.
