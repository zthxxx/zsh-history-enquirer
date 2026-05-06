# plan/10-tasks — atomic checklist

Single source of truth for refactor progress. Each item is one
idempotent action.

## P1. Foundation

- [x] Create orphan branch `refactor/golang/dev` in worktree
- [x] Write `docs/spec/{00-overview,10-widget-contract,20-history-loading,30-search-and-filter,40-rendering,50-keybindings,60-bracketed-paste,70-distribution}.md`
- [x] Write `docs/design/{00-architecture,10-fx-graph,20-history,30-tty,40-keys,50-ui,60-distribution,70-testing}.md`
- [x] Write `docs/plan/{00-roadmap,10-tasks}.md`
- [x] Add `.editorconfig`, `.gitattributes`, `.gitignore`
- [x] Add `.golangci.yml`
- [x] Add `Taskfile.yml`
- [x] Add `lefthook.yml`
- [x] Add `.markdownlint.yaml`, `.markdownlint-cli2.yaml`
- [x] First commit: `chore: scaffold project foundation (specs, design, plan, config)`

## P2. Core

- [x] `go mod init github.com/zthxxx/zsh-history-enquirer`
- [x] `internal/ansi`: cursor/erase primitives + tests
- [x] `internal/history/transform.go`: ReverseDedupeUnescape (pure)
  - [x] property tests via rapid
  - [x] table tests covering literal `\n` and empty entries
- [x] `internal/history/loader.go`: `Loader` interface + `ZshLoader` + `FixtureLoader`
  - [x] FixtureLoader tests against checked-in fixture
  - [x] ZshLoader tests using `t.TempDir()` HISTFILE (gated by `zsh` availability)
- [x] `internal/search`: Tokenize + AndFilter
  - [x] property tests via rapid

## P3. UI

- [x] `internal/tty`: TTY handle, raw mode RAII guard, DSR probe (unix.Poll-based)
  - [x] tests via creack/pty (skipped if no pty)
- [x] `internal/keys`: byte-stream parser, Event types, bracketed-paste state machine
  - [x] tests for marker-split-across-reads, ESC alone timeout, multi-byte rune
- [x] `internal/ui/wrap.go`: wrapped_row_count
  - [x] property tests
- [x] `internal/ui/model.go` + `update.go`: state struct + Update fn
  - [x] table tests for every keybinding
- [x] `internal/ui/render.go`: Frame builder
  - [x] golden tests
- [x] `internal/ui/throttle.go`: leading-edge throttle + trailing-edge flush in run.go

## P4. App

- [x] `internal/app/module.go`: fx providers + invokers (Stdout/StderrWriter named types)
- [x] `cmd/zsh-history-enquirer/main.go`: app bootstrap, exit-0 always
- [x] `plugin/zsh-history-enquirer.plugin.zsh`: widget file (no installer)
- [x] Smoke run: built, ran inside docker zsh, output `BUFFER` correctly

## P5. Distribution

- [x] `npm-workspace/`: pnpm-workspace.yaml, umbrella package, install shim
- [x] `npm-workspace/templates/platform/`: render template
- [x] `scripts/release/build-npm.sh`: render + publish flow (dry-run mode tested)

## P6. CI

- [x] `.github/workflows/ci.yml`: lint + unit + build matrix + e2e (debian + alpine)
- [x] `.github/workflows/release.yml`: tag-triggered build + release + npm publish + homebrew bump
- [x] `scripts/ci/bump-homebrew-tap.sh`: adapted from hams

## P7. E2E

- [x] `e2e/{debian,alpine}/Dockerfile`: zsh + expect
- [x] `e2e/scenarios/*.exp`: 8 scenarios — basic-pick, multi-line-scroll,
      cancel-preserves-input, multi-word-search, paste-bracketed,
      pageup-pagedown, home-end, prefilter-from-lbuffer
- [x] `e2e/run.sh`: scenario runner with fresh per-test state
- [x] `task ci:e2e:run`: one-liner shared with CI
- [x] Both targets pass: `summary: 8 passed, 0 failed`

## P8. Polish

- [x] `README.md`: rewrite for Go, manual install, no .zshrc edits
- [x] `README.zh-CN.md`: Chinese translation (the only non-English doc)
- [x] `AGENTS.md` + `CLAUDE.md` symlink: agent-facing instructions
- [x] `pkg/version/version.go`: version stamping, --version flag
- [x] `task check` runs clean: lint, test, e2e on both libcs

## Discovered & deferred items
- See [plan/20-followups.md](./20-followups.md).

See [plan/20-followups.md](./20-followups.md) for items found during
execution but deferred.
