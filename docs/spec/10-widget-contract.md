# spec/10-widget-contract — zsh ↔ binary boundary

The widget side of the project is a single file
(`plugin/zsh-history-enquirer.plugin.zsh`) that:

1. Defines a zle widget named `history_enquire`.
2. Binds it to <kbd>Ctrl</kbd>+<kbd>R</kbd> in **every** keymap a
   typical zsh user lands in: `emacs`, `viins`, `vicmd`. Vi-mode
   users would otherwise lose the picker the moment they hit Esc
   to enter `vicmd`.
3. On invocation:
   1. If the binary `zsh-history-enquirer` is on `$PATH`, runs
      `BUFFER=$(zsh-history-enquirer "$LBUFFER")` and snaps `CURSOR`
      to the new buffer length.
   2. If the binary is missing (mid-install, broken `$PATH`, …),
      falls back to `zsh`'s native widget via
      `zle .history-incremental-search-backward` (the leading `.`
      invokes the builtin without touching the current keymap), so
      the user is never left with a dead key. The previous
      bindkey-swap fallback was discarded — see
      [docs/plan/20-followups.md](../plan/20-followups.md).
4. Calls `zle -R -c` to repaint the prompt after `BUFFER` is set.

## Binary contract (mandatory)

The Go binary shall:

| concern | contract |
| --- | --- |
| **input** | `argv[1..]` joined with `' '` is the initial search input (i.e. the contents of `$LBUFFER` at invocation). |
| **stdout** | exactly one line: the chosen history entry, or — on cancel — the original input, or — on hard error — the original input. No trailing newline beyond what `BUFFER=$(...)` will strip. |
| **stderr** | reserved for diagnostics; never appears in `$BUFFER`. |
| **exit code** | `0` on submit, on cancel, on probe / load failure, and on every other code path. A non-zero exit code aborts `BUFFER=$(...)` and loses the user's typed input — see [legacy gotcha §2](../design/30-tty.md). |
| **kill signals** | SIGINT / SIGTERM / SIGHUP delivered externally must restore the terminal (leave raw mode, leave bracketed paste, show cursor) before the process exits, even if mid-render. The internal Ctrl-C arrives as the byte `0x03` because raw mode disables ISIG and is parsed as `KeyCtrlC` by `internal/keys`. |
| **interactive output** | written to `/dev/tty` directly, never to stdout, since stdout is a pipe under `$(…)` command substitution. |
| **interactive input** | read from `/dev/tty` directly when stdin is not a TTY. |

## Derived consequences

- `process.exit(0)` semantics from the legacy code translate to `os.Exit(0)`
  on every termination path in the Go binary — there is no useful
  exit-code signal because the shell uses none.
- The widget never has to know whether it spoke to the Node.js or Go
  implementation. This is what allows zero-downtime migration.
- A user who pastes into the picker, then hits <kbd>Esc</kbd>, gets back
  *exactly* what was on `LBUFFER` before they pressed <kbd>Ctrl</kbd>+<kbd>R</kbd>.
  This invariant is the cornerstone of the cancel-preserves-input UX.
- The "input is preserved" guarantee extends across **every** failure
  surface, including ones that have nothing to do with the picker UI:
  - missing platform binary (npm shim — see `npm/packages/zsh-history-enquirer/bin/cli.js`)
  - hard error inside `Run()` — raw-mode entry / geometry read
    (handled by `preserveOnError` in `internal/app/module.go`)
  - fx-provider startup failure — `/dev/tty` unopenable in a headless
    container (handled by `recoverStartFailure` in
    `cmd/zsh-history-enquirer/main.go`)
  - external `kill -TERM <pid>` mid-render (handled by the
    `signal.NotifyContext`-wrapped `runCtx` in `invokeRun`)
  
  All four paths funnel through the same conceptual rule: if the user
  typed something into `$LBUFFER`, that string is what `BUFFER=$(...)`
  must capture, no matter what went wrong inside the binary.
