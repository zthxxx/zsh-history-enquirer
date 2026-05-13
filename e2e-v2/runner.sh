#!/usr/bin/env sh
#
# e2e-v2/runner.sh — container entrypoint for the v2 Go-native
# harness.
#
# The image is intentionally content-free (zsh + locales/ncurses). At
# `docker run` time the task target bind-mounts:
#
#   /usr/local/bin/zsh-history-enquirer       picker binary
#   /opt/zsh-history-enquirer/plugin.zsh      widget plugin
#   /seed/history/                            history fixtures (ro)
#   /seed/zshrc.template                      .zshrc template (ro)
#   /usr/local/bin/harness.test               precompiled test binary
#   /runner.sh                                this script
#
# The harness itself creates the scratch HOME via t.TempDir() per
# test; the only host data the container can see is what these
# read-only mounts expose, so the legacy `e2e/seed-home.sh`'s
# write-HOME logic is replaced entirely by the harness's session-
# level isolation.
#
# Why not source seed-home.sh? The harness writes the .zshrc and
# .zsh_history into a per-test scratch HOME at runtime (rendered from
# /seed/zshrc.template and /seed/history/*.history). Reusing the
# shell function would pull a heredoc into the container that the
# harness then has to re-render — duplicate logic with different
# substitution rules. Keeping the templates as files mounted at
# /seed/ is the single source of truth.
set -eu

# Hand off to the precompiled Go test binary. `-test.v` mirrors the
# legacy harness's verbose-by-default output so a CI log reader still
# sees per-scenario PASS/FAIL banners.
#
# We deliberately do NOT wrap this in a `for` loop or per-scenario
# script: each `Test*` function in the binary is its own scenario,
# Go's testing package handles invocation, parallelism, naming,
# pass/fail summarisation, and exit code propagation. The legacy
# `e2e/run.sh`'s manual per-script loop becomes a one-line exec.
exec /usr/local/bin/harness.test -test.v "$@"
