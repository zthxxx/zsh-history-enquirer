# plan/10-tasks — atomic checklist

Single source of truth for refactor progress. Each item is one
idempotent action.

## P1. Foundation

- [x] Create orphan branch `refactor/golang/dev` in worktree
- [x] Write `docs/spec/{00-overview,10-widget-contract,20-history-loading,30-search-and-filter,40-rendering,50-keybindings,60-bracketed-paste,70-distribution}.md`
- [x] Write `docs/design/{00-architecture,10-fx-graph,20-history,30-tty,40-keys,50-ui,60-distribution,70-testing}.md`
- [x] Write `docs/plan/{00-roadmap,10-tasks}.md`
- [ ] Add `.editorconfig`, `.gitattributes`, `.gitignore`
- [ ] Add `.golangci.yml`
- [ ] Add `Taskfile.yml`
- [ ] Add `lefthook.yml`
- [ ] Add `.markdownlint.yaml`, `.markdownlint-cli2.yaml`
- [ ] First commit: `chore: scaffold project structure`

## P2. Core

- [ ] `go mod init github.com/zthxxx/zsh-history-enquirer`
- [ ] `internal/ansi`: cursor/erase primitives + tests
- [ ] `internal/history/transform.go`: ReverseDedupeUnescape (pure)
  - [ ] property tests via rapid
  - [ ] table tests covering literal `\n` and empty entries
- [ ] `internal/history/loader.go`: `Loader` interface + `ZshLoader` + `FixtureLoader`
  - [ ] FixtureLoader tests against checked-in fixture
  - [ ] ZshLoader tests using `t.TempDir()` HISTFILE (gated by `zsh` availability)
- [ ] `internal/search`: Tokenize + AndFilter
  - [ ] property tests via rapid
- [ ] Commit: `feat(core): history loader, search filter, ansi primitives`

## P3. UI

- [ ] `internal/tty`: TTY handle, raw mode RAII guard, DSR probe
  - [ ] tests via creack/pty (skipped if no pty)
- [ ] `internal/keys`: byte-stream parser, Event types, bracketed-paste state machine
  - [ ] tests for marker-split-across-reads, ESC alone timeout, multi-byte rune
- [ ] `internal/ui/wrap.go`: wrapped_row_count
  - [ ] property tests
- [ ] `internal/ui/model.go` + `update.go`: state struct + Update fn
  - [ ] table tests for every keybinding
- [ ] `internal/ui/render.go`: Frame builder
  - [ ] golden tests
- [ ] `internal/ui/throttle.go`: leading-edge throttle
- [ ] Commit: `feat(ui): TUI driver — keys, model, render, throttle`

## P4. App

- [ ] `internal/app/module.go`: fx providers + invokers
- [ ] `cmd/zsh-history-enquirer/main.go`: app bootstrap, signal handling, exit-0 always
- [ ] `plugin/zsh-history-enquirer.plugin.zsh`: widget file (no installer)
- [ ] Smoke run: build, pipe an LBUFFER, assert correct stdout+exit
- [ ] Commit: `feat(app): cmd entry, fx graph, plugin file`

## P5. Distribution

- [ ] `npm-workspace/`: pnpm-workspace.yaml, top-level package, install shim
- [ ] `npm-workspace/templates/platform/`: render template
- [ ] `scripts/release/build-npm.sh`: render + publish flow (dry-run mode for CI tests)
- [ ] Commit: `feat(dist): npm workspace + esbuild-style platform packages`

## P6. CI

- [ ] `.github/workflows/ci.yml`: lint + unit + build matrix + (placeholder) e2e
- [ ] `.github/workflows/release.yml`: tag-triggered build matrix + release + npm publish + homebrew bump
- [ ] `scripts/ci/bump-homebrew-tap.sh`: copied + adapted from hams
- [ ] Commit: `ci: push verification + tag-triggered release pipeline`

## P7. E2E

- [ ] `e2e/zsh/Dockerfile`: debian-slim + zsh + expect
- [ ] `e2e/zsh/scenarios/`: scenarios listed in design/70
- [ ] `e2e/zsh/run.sh`: scenario runner with timeout + transcript dump
- [ ] `task ci:e2e:run`: one-liner shared with CI
- [ ] Commit: `test(e2e): docker zsh widget scenarios`

## P8. Polish

- [ ] `README.md`: rewrite for Go, manual install, no .zshrc edits
- [ ] `AGENTS.md` + `CLAUDE.md` symlink: agent-facing instructions
- [ ] `pkg/version/version.go`: version stamping, --version flag
- [ ] `task check` runs clean: lint, test, e2e
- [ ] Final commit + push to origin
- [ ] Verify workflow triggers locally via `act` for CI workflow
- [ ] `<promise>COMPLETE</promise>` only after all of the above

See [plan/20-followups.md](./20-followups.md) for items found during
execution but deferred.
