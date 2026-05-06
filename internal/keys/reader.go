package keys

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

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

// Events returns a channel that yields Event values. The channel is
// closed when ctx is cancelled or the TTY signals EOF. The returned
// goroutine cleans up its signal subscription on exit.
//
// Implementation notes:
//   - Bytes are read in chunks of up to 64; that is more than enough
//     for any single key + bracketed-paste burst per syscall.
//   - The "ESC alone" timeout is 50 ms — long enough to coalesce a
//     real CSI sequence, short enough that pressing Esc still feels
//     instant.
//   - SIGWINCH is captured here, not in the UI layer, so the resize
//     event arrives on the same channel as keypresses and update
//     ordering stays linear.
func (r *Reader) Events(ctx context.Context) <-chan Event {
	out := make(chan Event, 32)

	winch := make(chan os.Signal, 1)
	signal.Notify(winch, unix.SIGWINCH)

	go func() {
		defer close(out)
		defer signal.Stop(winch)

		var mu sync.Mutex // protects the parser when used from two goroutines (read + winch)

		// Spawn a reader that pulls bytes and decodes them. We use a
		// goroutine so the SIGWINCH handler can interleave events.
		bytesCh := make(chan []byte, 16)
		readerErr := make(chan error, 1)
		go func() {
			buf := make([]byte, 64)
			for {
				n, err := r.tty.Reader().Read(buf)
				if n > 0 {
					b := make([]byte, n)
					copy(b, buf[:n])
					select {
					case bytesCh <- b:
					case <-ctx.Done():
						return
					}
				}
				if err != nil {
					readerErr <- err
					return
				}
			}
		}()

		// "ESC alone" flush timer.
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
				if _, ok := ev.(KeyEvent); ok {
					// Lone Esc was already flushed via FlushEsc; nothing more to do.
				}
			}
			return true
		}

		for {
			select {
			case <-ctx.Done():
				return

			case b := <-bytesCh:
				mu.Lock()
				events := r.parser.Feed(b)
				if r.parser.state == stateEsc {
					armFlush()
				}
				mu.Unlock()
				if !emit(events) {
					return
				}

			case <-flushTimer.C:
				mu.Lock()
				events := r.parser.FlushEsc()
				mu.Unlock()
				if !emit(events) {
					return
				}

			case <-winch:
				rows, cols, err := r.tty.Size()
				if err != nil {
					continue
				}
				if !emit([]Event{ResizeEvent{Rows: rows, Cols: cols}}) {
					return
				}

			case err := <-readerErr:
				if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, os.ErrClosed) {
					// Surface as a synthetic event? For now just exit
					// — the caller observes the channel close.
				}
				return
			}
		}
	}()

	return out
}
