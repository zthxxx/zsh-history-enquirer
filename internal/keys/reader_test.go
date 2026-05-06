package keys

import (
	"context"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

// TestReader_Events_BasicFlow drives a real pty pair through
// keys.Reader.Events to confirm:
//   - Bytes written to the master appear as Events on the channel.
//   - Multi-byte CSI sequences are coalesced into single KeyEvents.
//   - The architect-flagged goroutine-leak path (ctx.Done() while
//     the reader is mid-Read) actually closes the channel.
func TestReader_Events_BasicFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}

	master, slave, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})

	// Put the slave in raw mode so writes from master arrive as
	// raw bytes without canonical-mode line buffering.
	t1, err := tty.NewFromFile(slave)
	require.NoError(t, err)
	require.NoError(t, t1.EnterRaw())
	t.Cleanup(func() { _ = t1.LeaveRaw() })

	r := NewReader(t1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := r.Events(ctx)

	// Send three bytes: 'a' (rune), arrow-down (CSI sequence), 'b'.
	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(master, "a\x1b[Bb")
	}()

	got := drainEvents(events, 3, 2*time.Second)
	require.Len(t, got, 3)
	require.Equal(t, RuneEvent{R: 'a'}, got[0])
	require.Equal(t, KeyEvent{Key: KeyDown}, got[1])
	require.Equal(t, RuneEvent{R: 'b'}, got[2])
}

// TestReader_Events_CtxCancelClosesChannel asserts the
// architect-flagged guard: ctx cancellation closes the events
// channel even while the reader is mid-poll. Without the
// poll-based reader rewrite, the goroutine would leak waiting on
// a Read that never returns.
func TestReader_Events_CtxCancelClosesChannel(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}

	master, slave, err := pty.Open()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = master.Close()
		_ = slave.Close()
	})

	t1, err := tty.NewFromFile(slave)
	require.NoError(t, err)
	require.NoError(t, t1.EnterRaw())
	t.Cleanup(func() { _ = t1.LeaveRaw() })

	r := NewReader(t1)
	ctx, cancel := context.WithCancel(context.Background())
	events := r.Events(ctx)

	// Cancel — no bytes were ever sent. The reader's poll loop
	// must wake up at the next pollInterval (100ms) and exit.
	cancel()

	select {
	case _, ok := <-events:
		require.False(t, ok, "channel should close on ctx cancel, not deliver an event")
	case <-time.After(2 * time.Second):
		t.Fatal("ctx cancellation did not close the events channel within 2s")
	}
}

// TestReader_Prefeed_ReplaysProbeLeftover ensures the Prefeed
// helper produces the same events as a normal Feed — the picker
// uses this to replay bytes the cursor probe consumed during a
// timeout window.
func TestReader_Prefeed_ReplaysProbeLeftover(t *testing.T) {
	t.Parallel()

	r := NewReader(&tty.TTY{}) // tty isn't touched by Prefeed
	got := r.Prefeed("hi\x1b[A")
	require.Len(t, got, 3)
	require.Equal(t, RuneEvent{R: 'h'}, got[0])
	require.Equal(t, RuneEvent{R: 'i'}, got[1])
	require.Equal(t, KeyEvent{Key: KeyUp}, got[2])
}

// TestReader_Prefeed_EmptyStringIsNoOp covers the early-return
// branch.
func TestReader_Prefeed_EmptyStringIsNoOp(t *testing.T) {
	t.Parallel()

	r := NewReader(&tty.TTY{})
	require.Nil(t, r.Prefeed(""))
}

// drainEvents reads up to `n` events from the channel within
// `timeout`. Returns whatever it collected so partial counts can be
// asserted explicitly by the test.
func drainEvents(ch <-chan Event, n int, timeout time.Duration) []Event {
	got := make([]Event, 0, n)
	deadline := time.After(timeout)
	for len(got) < n {
		select {
		case ev, ok := <-ch:
			if !ok {
				return got
			}
			got = append(got, ev)
		case <-deadline:
			return got
		}
	}
	return got
}

// Defensive compile-time check: the unix package must export the
// poll constants we depend on. A future Go-x-sys reshuffle that
// removes these would otherwise break us at runtime.
var _ = unix.POLLIN
