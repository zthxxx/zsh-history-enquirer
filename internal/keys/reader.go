package keys

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"golang.org/x/sys/unix"

	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

// PanicWriter is the destination for panic-recovery diagnostics in the
// reader goroutine. Defaults to os.Stderr; tests can redirect.
//
//nolint:gochecknoglobals // intentional swap point.
var PanicWriter io.Writer = os.Stderr

// recoverGoroutinePanic is the deferred recover for the reader's hot
// loop. A parser bug or third-party crash would otherwise terminate
// the whole process and destroy $LBUFFER (BUFFER=$(...) captures
// stdout — no stdout means an empty buffer for the user). Closing the
// events channel from the deferred close above signals the main loop
// to exit cleanly with Canceled=true; the loop then echoes m.Input
// back to stdout, preserving the widget contract. The panic itself is
// reported to PanicWriter (stderr by default — invisible to the
// command substitution).
//
// Output uses "\r\n" rather than "\n" because the panic happens while
// the terminal is still in raw mode (OPOST disabled) — a bare "\n"
// drops the cursor down a row without resetting to col 0, so any
// subsequent stderr write (from main's recover or the shell prompt
// after we exit) would land at a stale column.
func recoverGoroutinePanic() {
	if rec := recover(); rec != nil {
		_, _ = fmt.Fprintf(PanicWriter,
			"zsh-history-enquirer: keys reader panic recovered: %v\r\n", rec)
	}
}

// Reader streams events from a TTY plus terminal-resize signals.
type Reader struct {
	tty    *tty.TTY
	parser *Parser
}

// NewReader wraps a TTY into an event source.
func NewReader(t *tty.TTY) *Reader {
	return &Reader{tty: t, parser: NewParser()}
}

// Prefeed pushes a string of bytes through the parser before the
// reader goroutine starts. Used to replay user-typed bytes that the
// cursor probe consumed while waiting for a DSR response that never
// arrived.
func (r *Reader) Prefeed(s string) []Event {
	if s == "" {
		return nil
	}
	return r.parser.Feed([]byte(s))
}

// pollInterval is the maximum time we ever block in unix.Poll inside
// the reader loop. Each iteration, we either wake on data or on the
// poll timeout; the timeout is what lets us see ctx cancellation
// without a separate goroutine that would leak on cancel.
const pollInterval = 100 * time.Millisecond

// Events returns a channel that yields Event values. The channel is
// closed when ctx is canceled or the TTY signals EOF.
//
// Implementation:
//   - One goroutine, one fd. We unix.Poll with a short interval so
//     ctx.Done() is checked at least every pollInterval; this avoids
//     the goroutine-leak bug where a separate read goroutine would
//     stay blocked in a syscall after the parent select returned.
//   - SIGWINCH is captured on the same goroutine so events stay
//     linearly ordered with keypresses.
//   - The "ESC alone" timeout is 50ms — long enough to coalesce a
//     real CSI sequence, short enough that pressing Esc still feels
//     instant.
func (r *Reader) Events(ctx context.Context) <-chan Event {
	out := make(chan Event, 32)

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, unix.SIGWINCH)

	go func() {
		defer close(out)
		defer signal.Stop(winch)
		// Panic recovery: a parser bug or third-party crash inside
		// this hot loop would otherwise terminate the whole process,
		// destroying $LBUFFER (BUFFER=$(...) captures only stdout —
		// no stdout means an empty buffer for the user). Closing
		// `out` from the deferred close above signals the main loop
		// to exit cleanly with Canceled=true; the loop then echoes
		// m.Input back to stdout, preserving the widget contract.
		// The panic itself is reported to stderr (invisible to the
		// command substitution).
		defer recoverGoroutinePanic()

		fd := int(r.tty.File().Fd())

		flushTimer := time.NewTimer(time.Hour)
		flushTimer.Stop()
		defer flushTimer.Stop()
		armFlush := func() {
			if !flushTimer.Stop() {
				select {
				case <-flushTimer.C:
				default:
				}
			}
			flushTimer.Reset(50 * time.Millisecond)
		}

		emit := func(events []Event) bool {
			for _, ev := range events {
				select {
				case out <- ev:
				case <-ctx.Done():
					return false
				}
			}
			return true
		}

		buf := make([]byte, 64)
		for {
			// 1. Check ctx cancellation.
			select {
			case <-ctx.Done():
				return
			default:
			}

			// 2. Drain SIGWINCH non-blockingly.
			select {
			case <-winch:
				rows, cols, werr := r.tty.Size()
				if werr == nil {
					if !emit([]Event{ResizeEvent{Rows: rows, Cols: cols}}) {
						return
					}
				}
			default:
			}

			// 3. Drain the ESC-alone flush timer.
			select {
			case <-flushTimer.C:
				if !emit(r.parser.FlushEsc()) {
					return
				}
			default:
			}

			// 4. Poll the fd. The pollInterval keeps each iteration
			//    bounded so ctx.Done() / SIGWINCH / flushTimer all
			//    get checked at least every pollInterval ms.
			pfd := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
			nready, perr := unix.Poll(pfd, int(pollInterval/time.Millisecond))
			if perr != nil {
				if perr == unix.EINTR {
					continue
				}
				return
			}
			if nready == 0 || pfd[0].Revents&unix.POLLIN == 0 {
				continue
			}

			n, rerr := unix.Read(fd, buf)
			if n > 0 {
				events := r.parser.Feed(buf[:n])
				// Arm the flush timer whenever the parser is left in a
				// state FlushEsc can resolve. Otherwise an aborted ESC
				// prelude (terminal sent `\eO` or `\e[` then nothing —
				// rare but possible on flaky links and unusual
				// emulators) would leave the picker frozen indefinitely
				// with no events emitted: the user would see no key
				// feedback until any other byte arrived to break the
				// sequence (and even then, the buffered bytes would be
				// silently discarded by the unrecognized-sequence
				// default branch). stateCSI is in the list because a
				// stuck `\e[` is the same hazard as a stuck `\eO`.
				switch r.parser.state {
				case stateEsc, stateSS3, stateCSI:
					armFlush()
				}
				if !emit(events) {
					return
				}
			}
			if rerr != nil {
				// EINTR is recoverable: a signal (typically SIGWINCH from
				// the user resizing the terminal) interrupted the read
				// between poll returning POLLIN and the Read syscall
				// completing. The next iteration drains SIGWINCH and
				// re-polls; killing the picker on every resize would be
				// a hostile UX. Other errors are genuinely unrecoverable.
				if rerr == unix.EINTR {
					continue
				}
				return
			}
			if n == 0 {
				// EOF / fd closed; exit so the caller observes the
				// channel close.
				return
			}
		}
	}()

	return out
}
