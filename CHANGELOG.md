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

### Distribution

- `npm install -g zsh-history-enquirer` — esbuild-style with four
  `@zsh-history-enquirer/<os>-<arch>` `optionalDependencies`.
- `brew install zthxxx/tap/zsh-history-enquirer`.
- Raw GitHub Release binaries with `checksums.txt`.

## [1.3.1] - 2022-01-23

The last Node.js release. See git history on the `master` branch
for prior changes.
