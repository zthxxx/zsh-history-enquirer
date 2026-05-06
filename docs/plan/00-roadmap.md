# plan/00-roadmap — macro plan

The refactor lands as a sequence of self-contained git commits, each one
of which leaves the worktree green (`task check`). The order below is
chosen so that earlier commits unblock later ones without requiring
forward references.

## Phases

```
P1. Foundation    docs (spec/design/plan), config files, Taskfile
P2. Core          history, search, ansi, tty packages with unit tests
P3. UI            keys parser, ui model + render, property tests
P4. App           cmd entrypoint, fx wiring, plugin file
P5. Distribution  npm skeleton + templates, postinstall hint shim
P6. CI            push CI (lint/test/build), release CI (npm + homebrew)
P7. E2E           docker image, scenarios, act parity
P8. Polish        README/AGENTS rewrite, version stamping, hardening
```

## Cadence

- Every phase ends with a `git commit` titled
  `<phase-id>: <one-line summary>`.
- Phases that introduce code are followed by a `task check` run; the
  commit is amended only if check fails on a fixable regression.
- Phases that introduce specs commit the docs first, then a separate
  commit for the corresponding code (so reviewers can read intent
  without code noise).

## Branch hygiene

- All work happens on `refactor/golang/dev`, an orphan branch with no
  shared history with `master` / `dev`. This is intentional: the
  refactor replaces the project, not amends it.
- The branch is rebased on itself (squash-fixup) only between phases,
  never inside a phase.
- The branch is pushed after every phase; force-with-lease is used if
  a rebase happened.

See [plan/10-tasks.md](./10-tasks.md) for the atomic task checklist.
