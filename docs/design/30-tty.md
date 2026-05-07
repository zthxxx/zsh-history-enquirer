# design/30-tty — terminal handle, raw mode, DSR cursor query

> Spec: [spec/10 §interactive_output](../spec/10-widget-contract.md),
> [spec/40 §inline placement](../spec/40-rendering.md)

## Why we open `/dev/tty` directly

The widget invokes the binary inside `BUFFER=$(zsh-history-enquirer "$LBUFFER")`,
which makes:

- stdout: pipe back to zsh — **not a TTY**.
- stderr: still a TTY, but using it for UX is bad form (script
  wrappers often redirect it).

We need a **bidirectional handle on the controlling terminal**, so
we open `/dev/tty` ourselves. Both reads (key input) and writes
(escapes, frames) go through this fd.

```go
package tty

type TTY struct {
    file       *os.File       // wraps the /dev/tty fd
    savedTerm  *unix.Termios  // termios snapshot for restore
    rawEntered bool           // true while raw mode is active
}

func Open() (*TTY, error)
func NewFromFile(f *os.File) (*TTY, error)
func NewDevTTY(lc fx.Lifecycle) (*TTY, error)
```

`Open` is the bare opener used outside fx. `NewDevTTY` is the
fx-injected constructor that registers an OnStop. `NewFromFile`
wraps an already-open `*os.File` into a TTY — used by tests
driving a `creack/pty` slave.

The fx-bound constructor's hook:

1. If raw mode was entered, calls `LeaveRaw()` to restore the
   original termios (also disables bracketed paste).
2. Closes the `/dev/tty` fd.

`Close()` is idempotent: a second call is a no-op so error paths
that already cleaned up don't double-restore.

## Raw mode

`(*TTY).EnterRaw()` mutates the termios:

- `cflag |= CS8`
- `lflag &^= ICANON | ECHO | ISIG | IEXTEN` — no canonical mode,
  no local echo, no signal generation, no extended.
- `iflag &^= IXON | ICRNL | IGNCR | INLCR | ISTRIP | INPCK | BRKINT`
- `oflag &^= OPOST` — no post-processing; newline stays `\n`.
- `cc[VMIN] = 1; cc[VTIME] = 0`

Crucially we keep `ISIG` **off** in raw mode, because we want to
handle <kbd>Ctrl</kbd>+<kbd>C</kbd> ourselves — its byte (`0x03`)
becomes a key event the picker translates into "cancel" rather
than a signal that kills the process before we can write the
input back to stdout.

## DSR cursor query

```go
package tty

type Probe struct{ tty *TTY }

func NewProbe(t *TTY) *Probe
func (p *Probe) Cursor(ctx context.Context, timeout time.Duration) (row, col int, leftover string, err error)
```

Implementation:

1. Write `\e[6n` to the TTY.
2. Drive a read loop with `unix.Poll` honouring an absolute
   timeout. The poll-with-timeout path is required because
   `os.File.SetReadDeadline` is **unreliable on /dev/tty in
   docker's pty emulation** — some kernels block past the
   deadline. `unix.Poll` honours its timeout byte-for-byte
   regardless of the file's blocking mode.
3. The poll → read pair MUST be EINTR-resilient on BOTH halves —
   a SIGWINCH that lands while the read syscall is in flight
   would otherwise force a fallback every time the user resizes
   their terminal mid-Ctrl-R.
4. Anchor the response parse on `\x1b[`, not the first `[`, and
   require the loop break on `\x1b[<...>R` (not just any `R`).
   Without these anchors, a fast-typing user pressing `^R [` or
   `^R R` would short-circuit the parse / loop and lose
   keystrokes.
5. Parse `\e[<row>;<col>R`. 1-indexed in the protocol.
6. Return any bytes consumed alongside the response as
   `leftover` (a separate return value) — and on the timeout
   path, via `*tty.TimeoutError.Leftover`. Both routes feed the
   same `keys.Reader.Prefeed` so the user's pre-render
   keystrokes never get silently dropped.

The probe **must** run after raw mode is entered; otherwise the
terminal's line discipline echoes the response and corrupts the
next read.

## Bracketed paste

Enabled at startup by writing `\e[?2004h`, disabled by `LeaveRaw`
(`\e[?2004l`). The byte stream parser then sees `\e[200~` and
`\e[201~` markers around pastes — see [design/40-keys](./40-keys.md).

## Tests

- `internal/tty/tty_test.go`: unit-tests TTY against a `creack/pty`
  slave via `NewFromFile`. Catches Close-twice safety, raw-mode
  enter/leave, geometry queries.
- `internal/tty/raw_test.go` and `cursor_test.go`: drive the DSR
  probe end-to-end through a pty pair — write the query on the
  slave side, write a canned `\e[<row>;<col>R` from the master
  goroutine, assert the parsed result.

There is no in-memory `MemoryTTY` mock; tests use real pty pairs
because the value-add of the mock would mostly be in fields the
production code does not actually depend on. Tests run without a
controlling terminal because the pty slave is what we read from,
not /dev/tty.
