# design/70-testing — what runs where

## Test layers

| Layer | Where | What |
| --- | --- | --- |
| **unit** | `internal/**/*_test.go` | pure-Go, no zsh, no docker. Property-based via `pgregory.net/rapid` plus golden-frame tests in `internal/ui`. Runs with `task test:unit`. |
| **integration** | same package | components that need a pty (via `creack/pty`), still pure Go, no docker, but skipped on platforms without pty (Windows). |
| **e2e** | `e2e/zsh/` | real binary inside docker, real zsh, real pty, scripted via `expect`. **Never** runs against the user's machine; the entry script refuses to run outside the docker image. |

## Why e2e in docker

The legacy `tests/zsh-widget.test.zsh` runs an `expect` script against
`zsh -il` on the developer's box. It works, but:

- Mutates `~/.zsh_history` if `ZDOTDIR` isn't set right.
- Depends on whatever zsh + plugins are installed locally.
- Tickles the developer's terminal emulator with raw escapes — bad UX
  and flaky on certain emulators (warp, kitty's variants, …).

Putting it in docker:

- Pins zsh and OS versions (we test against latest stable + LTS).
- Runs against a known empty `$HOME`.
- Exits cleanly even on a CI-killed run because the container is torn
  down.

## Docker image

`e2e/zsh/Dockerfile` (debian-slim base):

```dockerfile
FROM debian:bookworm-slim AS base
RUN apt-get update && apt-get install -y --no-install-recommends \
    zsh expect ca-certificates \
    && rm -rf /var/lib/apt/lists/*
RUN useradd -m tester && chsh -s /bin/zsh tester
USER tester
WORKDIR /home/tester
COPY --chown=tester e2e/zsh/zshrc        /home/tester/.zshrc
COPY --chown=tester e2e/zsh/scenarios/   /home/tester/scenarios/
COPY --chown=tester e2e/zsh/run.sh       /home/tester/run.sh
ENTRYPOINT ["/home/tester/run.sh"]
```

The binary (`bin/zsh-history-enquirer-linux-<arch>`) is mounted in
read-only at `/usr/local/bin/zsh-history-enquirer`. The plugin file is
mounted under `/opt/zsh-history-enquirer/plugin.zsh`. The `~/.zshrc`
sources it.

## Scenario design

Each scenario is a `*.exp` file under `e2e/zsh/scenarios/`. They cover:

- **basic**: type `ech`, ^R, observe `echo ok`, hit Enter, run command,
  observe `ok` printed. (Regression from legacy.)
- **multi-line**: type nothing, ^R, scroll past a 5-line heredoc,
  observe the picker doesn't overflow.
- **multi-line wrapping**: terminal width = 30, navigate past a command
  that wraps + has embedded newlines.
- **paste-during-pick**: simulate bracketed paste with `\e[200~st\e[201~`
  + `at`, expect `stat`.
- **cancel-preserves-input**: type `xyz`, ^R, type `qwerty`, hit
  <kbd>Esc</kbd>, observe LBUFFER == `xyzqwerty`.

  > Note: the legacy spec says cancel returns the input typed inside
  > the picker. The widget then sets `BUFFER=$picker_output`, which
  > _replaces_ LBUFFER — so the surviving line is the in-picker input,
  > not LBUFFER + in-picker input. This corner is documented in
  > [spec/50](../spec/50-keybindings.md).

- **resize-during-pick**: send `SIGWINCH` after rendering, expect a
  re-render at new dimensions.
- **scroll-edges**: pageDown past the last match, expect rotation back
  to top; pageUp past the first, expect rotation back to bottom.

Each scenario is an idempotent expect script with a 20s outer timeout.
Failures dump the full transcript to stdout for triage.

## act compatibility

`task ci:e2e:run` is the **single recipe** invoked both by GitHub
Actions and by `act`. It builds the image (cache-keyed by Dockerfile
hash) and runs every scenario. CI passes the same `task ci:e2e:run`
command to act as it does on the runner — there is no
"works locally, fails on CI" gap.

## Property tests (unit-layer)

Used in:

- `internal/history`: reverse + dedupe + unescape invariants.
- `internal/search`: tokenize/AND-filter idempotence and monotonicity.
- `internal/ui/wrap`: row-count linearity and empty-line invariants.

Where used, the test name has the prefix `TestProperty_` so
`task test:property` runs them in isolation for benchmark-style
reproducibility.
