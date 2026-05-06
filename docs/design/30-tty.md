# design/30-tty — terminal handle, raw mode, DSR cursor query

> Spec: [spec/10 §interactive_output](../spec/10-widget-contract.md),
> [spec/40 §inline placement](../spec/40-rendering.md)

## Why we open `/dev/tty` directly

The widget invokes the binary inside `BUFFER=$(zsh-history-enquirer "$LBUFFER")`,
which makes:

- stdout: pipe back to zsh — **not a TTY**.
- stderr: still a TTY, but using it for UX is bad form (script wrappers
  often redirect it).

We need a **bidirectional handle on the controlling terminal**, so we
open `/dev/tty` ourselves with `O_RDWR | O_CLOEXEC`. Both reads (key
input) and writes (escapes, frames) go through this fd.

```go
package tty

type TTY struct {
    fd     int
    file   *os.File   // wraps fd for io.Reader/Writer
    saved  *unix.Termios
    closed bool
}

func NewDevTTY(lc fx.Lifecycle) (*TTY, error)
```

The constructor:

1. `unix.Open("/dev/tty", O_RDWR|O_CLOEXEC, 0)` — gets the fd.
2. `unix.IoctlGetTermios(fd, TCGETS)` — saves the original termios.
3. Registers an `lc.OnStop` hook that restores termios + close fd.

## Raw mode

Raw mode is entered explicitly by `(*TTY).EnterRaw()`:

- `cflag |= CS8`
- `lflag &^= ICANON | ECHO | ISIG | IEXTEN` (no canonical, no local
  echo, no signal generation, no extended)
- `iflag &^= IXON | ICRNL | IGNCR | INLCR | ISTRIP | INPCK | BRKINT`
- `oflag &^= OPOST` (no postprocessing → newline becomes `\n`, not `\r\n`)
- `cc[VMIN] = 1; cc[VTIME] = 0`

Crucially we keep `ISIG` *off* in raw mode, because we want to handle
<kbd>Ctrl</kbd>+<kbd>C</kbd> ourselves — its byte (0x03) becomes a
key event we translate into "cancel" rather than a signal that kills
the process.

## DSR cursor query

```go
package tty

// Probe queries the terminal for its current cursor position.
type Probe struct{ tty *TTY }

func (p *Probe) Cursor(ctx context.Context, timeout time.Duration) (row, col int, err error)
```

Implementation:

1. Write `[6n`.
2. Read until `R` is encountered, with the deadline set on the fd via
   `SetReadDeadline` (works because we wrap the fd in `os.File`).
3. Parse `[<row>;<col>R`. 1-indexed → subtract 1 internally.
4. Return.

The probe **must** run after raw mode is entered, otherwise the
terminal's line discipline echoes the response and corrupts the next
line.

## Bracketed paste

Enabled at startup by writing `\e[?2004h`, disabled at shutdown via
the lifecycle hook (`\e[?2004l`). The byte stream parser then sees
`\e[200~` and `\e[201~` markers around pastes — see `internal/keys`.

## Tests

- A `tty.MemoryTTY` is provided for unit tests of UI logic. It
  implements the same `Reader/Writer/Probe` surface but stores
  written bytes in a `bytes.Buffer` and serves reads from a scripted
  channel.
- A `creack/pty`-based test spawns a master/slave pair, sets the slave
  as the test's `/dev/tty`, and exercises real DSR + bracketed paste.

The unit tests must not touch a real terminal — CI runs without one.
