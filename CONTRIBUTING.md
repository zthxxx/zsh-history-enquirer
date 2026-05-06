# Contributing to zsh-history-enquirer

Thanks for contributing. This is a small, focused project: a zsh
widget that replaces <kbd>Ctrl</kbd>+<kbd>R</kbd> with an inline,
multi-line, deduplicated, multi-word fuzzy history picker. Anything
that doesn't make `^R` better is out of scope.

## Repo layout

| Path | What it is |
| --- | --- |
| `cmd/zsh-history-enquirer/` | Binary entry; fx-bootstrapped main. |
| `internal/app/` | DI graph + Run() picker loop. |
| `internal/{history,search,tty,keys,ui,ansi}/` | Layered domain packages. See `.go-arch-lint.yml` for the dependency rules. |
| `pkg/version/` | `-ldflags`-injected build identification. |
| `plugin/zsh-history-enquirer.plugin.zsh` | The widget file users source from `~/.zshrc`. |
| `e2e/{debian,alpine}/` | Per-libc Docker images. |
| `e2e/scenarios/*.exp` | Expect-driven scenarios. |
| `npm/` | NPM umbrella package + per-platform sub-package templates. |
| `docs/spec/` | User-visible behaviour. |
| `docs/design/` | Spec → Go implementation map. |
| `docs/plan/` | Roadmap + atomic checklist + post-mortems. |

## Prerequisites

- Go ≥ 1.25 (the `go.uber.org/fx@v1.24` dependency forces this; the
  same version is pinned in `.github/workflows/ci.yml`).
- Docker (for e2e; tests under `internal/**` do NOT require Docker).
- [`task`](https://taskfile.dev) — run `brew install go-task` or
  `go install github.com/go-task/task/v3/cmd/task@latest`.
- Optional: [`act`](https://github.com/nektos/act) for local CI parity.

Run `task setup` to install the rest (`golangci-lint@v2.12.2` —
matches CI; older versions are built against Go 1.24 and refuse to
load against the project's `go 1.25.0` go.mod, plus `goimports`,
`go-arch-lint`, `lefthook`).

## Workflow

```bash
task check:fast       # fmt + lint (go + arch + md) + unit tests
task check            # the above + e2e in Docker
task test:e2e         # e2e on debian + alpine
task test:e2e:one TARGET=debian   # one target only
task lint:go:fix      # auto-fix the auto-fixable
```

`task check:fast` is the standard pre-PR loop. `task check` is what
CI runs.

## Branching + commits

- One feature per branch. Multi-segment branch names are fine
  (`feat/foo`, `fix/bar/baz`); the CI's `on: push: branches: '**'`
  matches them.
- [Conventional Commits](https://www.conventionalcommits.org/).
  `commitlint` runs in the `commit-msg` hook via `lefthook`, so a
  typo in the message fails the commit before it's recorded (no
  push needed). The valid types are
  `feat / fix / chore / docs / refactor / test / ci / build / perf`.
  The pre-commit hook (separate) runs `task fmt`, `task lint:go`,
  `markdownlint-cli2`, and `task test:unit`.

## Spec → Design → Plan

Non-trivial changes start in `docs/`. Workflow:

1. Add or update `docs/spec/` with the *user-visible* behaviour.
2. Add or update `docs/design/` with the Go *implementation map*.
3. Append atomic tasks to `docs/plan/10-tasks.md` with `- [ ]`
   checkboxes; flip them to `- [x]` as you go.
4. Record any non-obvious decision or post-mortem in
   `docs/plan/20-followups.md` with date + cite.

The PR review checks that spec / design changes accompany code
changes that affect user-visible behaviour or layer boundaries.

## Layering

Enforced by `.go-arch-lint.yml`. The TL;DR:

- `cmd/` may import `internal/app` and `pkg/`.
- `internal/app` may import every other internal package.
- `internal/ui` may import `internal/{search, ansi, keys}`.
- `internal/keys` may import `internal/tty`.
- `internal/tty` may import `internal/ansi`.
- `internal/{history, search, ansi, version}` import nothing else
  internal — they are leaf packages.

Reach for a layer-violating import and `task lint:arch` will tell
you. CI runs the same check.

## Tests

- **Unit tests live next to the code**, in `*_test.go`. Go's
  built-in test runner. Use property-based tests where the
  invariant is amenable (we use [`pgregory.net/rapid`](https://pkg.go.dev/pgregory.net/rapid)
  in `history`, `search`, `ui/wrap`, `ui/highlight`, `keys/parser`).
- **No unit test may write to the user's real `$HISTFILE` or
  `~/.zshrc`.** History tests use `t.TempDir()` fixtures.
- **E2E tests live in Docker.** `e2e/scenarios/*.exp` runs against a
  real zsh, real pty, real binary. They are isolated in containers
  with empty `$HOME`.
- **Race detector is mandatory in CI.** `go test -race -count=1` is
  what `task test:unit` runs.

When adding a feature, the spec in `docs/spec/` says what the user
sees, and a corresponding test in either `internal/**/*_test.go` or
`e2e/scenarios/*.exp` enforces it.

## Releases

Tag `vX.Y.Z` on `master`, push the tag — `release.yml` does the
rest:

1. Cross-compiles for darwin/linux × arm64/amd64.
2. Creates a GitHub Release with `checksums.txt`.
3. Renders the npm umbrella + four `@zsh-history-enquirer/<os>-<arch>`
   sub-packages from `npm/templates/platform/` and
   publishes them via `NPM_TOKEN`.
4. Opens a PR against `zthxxx/homebrew-tap` rewriting the formula
   with the new version + per-platform sha256s, via
   `HOMEBREW_TAP_TOKEN`.

`v*-rc.N` / `v*-alpha.N` / `v*-beta.N` are released as GitHub
pre-releases and skip the homebrew-tap bump (Homebrew users only
see stable versions).

## Reporting bugs

Include:

- `zsh-history-enquirer --version` output.
- `echo "$ZSH_VERSION $TERM $(uname -srm)"`.
- The first ~50 bytes of `~/.zsh_history` if the bug touches a
  particular kind of entry (multi-line, unicode, …).
- The exact key sequence that reproduces.

Logs: set `ZHE_DEBUG=/tmp/zhe.log` before triggering the picker;
attach `/tmp/zhe.log` to the issue.
