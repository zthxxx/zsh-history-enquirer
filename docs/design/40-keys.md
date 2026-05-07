# design/40-keys — byte-stream → Event parser

> Spec: [spec/50](../spec/50-keybindings.md), [spec/60](../spec/60-bracketed-paste.md)

## Public API

```go
package keys

type Event interface{ event() }

type RuneEvent   struct{ R rune }              // a printable character
type KeyEvent    struct{ Key Key }             // arrow, enter, esc, ctrl-x, etc.
type PasteEvent  struct{ Payload string }      // bracketed paste payload, decoded
type ResizeEvent struct{ Rows, Cols int }      // SIGWINCH

type Reader struct{ … }
func NewReader(t *tty.TTY) *Reader

// Events returns a channel that produces events until ctx is done
// or the underlying reader yields EOF / unrecoverable poll error.
func (r *Reader) Events(ctx context.Context) <-chan Event

// Prefeed pushes already-consumed bytes through the parser before
// the reader goroutine starts. Used to replay bytes the cursor
// probe consumed during a timeout window.
func (r *Reader) Prefeed(s string) []Event
```

`KeyEvent` has only the `Key` field — no `Mod` modifier. zsh's
`^R` widget contract gives us bare key names (Up, Down, Enter,
Esc, Ctrl-* combos as named Keys), and modifiers like Shift+Tab
do not need to be distinguished by the picker. If a future
feature wants modifiers, it should be added behind a feature flag
so the simple `Key` shape stays the default.

`Reader` is a struct, not an interface — there's exactly one
production implementation. Tests use the same type, driving it
against a `creack/pty` slave (see `reader_test.go`).

## Why a custom parser, not bubbletea's

bubbletea has a key parser, but:

1. It does not natively expose bracketed-paste payloads as a single
   event — it emits each byte of the payload as a separate keypress
   inside the marker window. We want a single `PasteEvent`.
2. It throws away unrecognised escape sequences. The DSR response
   (`\e[<row>;<col>R`) we emit on startup can collide with a key-read
   loop if not handled carefully; owning the parser lets us reserve
   that sequence for the cursor probe and ignore it in the event stream.

## State machine

```
state = normal
buf   = []byte

on byte b:
  case state:
    normal:
      if b == 0x1b: state = esc; continue
      if b in printable: emit RuneEvent(b); continue
      if b == 0x03: emit KeyEvent{CtrlC}; continue
      if b == 0x0d || b == 0x0a: emit KeyEvent{Enter}; continue
      if b == 0x7f || b == 0x08: emit KeyEvent{Backspace}; continue
      ...
    esc:
      if b == '[': state = csi; continue
      if b == 0x1b || timeout: emit KeyEvent{Esc}; state = normal
      ...
    csi:
      buf = append(buf, b)
      if matches '[A'..'[D': emit Key{Up..Left}; flush
      if matches '[H'/'[F': emit Key{Home/End}
      if matches '[5~'/'[6~': emit Key{PageUp/Down}
      if matches '[200~': state = paste; flush
      ...
    paste:
      if buf endswith '\e[201~': emit PasteEvent(buf[:-len(marker)]); state = normal
      else: append b to buf
```

Boundary conditions to test:

- Marker split across reads (one byte per syscall).
- Paste payload containing a literal `0x03` — must not become CtrlC.
- ESC alone (no follow-up) within ~50 ms must produce an Esc event,
  not be swallowed waiting for a CSI completion.
- Multi-byte UTF-8 must be decoded into a single RuneEvent.

## Stuck-prelude flush (`Parser.FlushEsc`)

The 50 ms `flushTimer` in `keys.Reader` calls `Parser.FlushEsc`
whenever the parser is parked in any of `stateEsc` / `stateSS3` /
`stateCSI` and no follow-up bytes have arrived. Each prelude has a
distinct flush translation so the user's intent (typically Esc =
cancel) is preserved instead of the picker freezing forever:

| Parked state | Bytes consumed | Flush emits                                       |
| ------------ | -------------- | ------------------------------------------------- |
| `stateEsc`   | `\e`           | `KeyEsc`                                          |
| `stateSS3`   | `\eO`          | `KeyEsc`, `RuneEvent('O')`                        |
| `stateCSI`   | `\e[<params?>` | `KeyEsc`, `RuneEvent('[')`, params as runes       |

Without this, a flaky terminal or programmatic input that paused
mid-sequence (e.g. `\e[` and stop) would leave the parser stuck —
and worse for `stateCSI`, every subsequent typed byte in
`0x40..0x7e` would terminate the sequence as unrecognized and be
silently discarded by the default branch. The reader arms its
flush timer for ALL THREE states (not just `stateEsc`) so any
parked prelude resolves within the 50 ms window.

Tests: `TestParser_FlushEsc_*` family in `parser_test.go`.

## Resize

`SIGWINCH` is captured via `signal.Notify` and translated into a
`ResizeEvent{Rows, Cols}` by re-querying `unix.IoctlGetWinsize`. The
event is sent on the same channel so the UI update loop sees it.
