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

// TestReader_Events_PasteEvent drives a bracketed-paste through the
// reader to verify the Reader → Parser handoff preserves the paste
// payload as a single event (not split per-byte).
func TestReader_Events_PasteEvent(t *testing.T) {
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
	defer cancel()
	events := r.Events(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(master, "\x1b[200~git status\x1b[201~")
	}()

	got := drainEvents(events, 1, 2*time.Second)
	require.Len(t, got, 1, "paste must arrive as exactly ONE event")
	pe, ok := got[0].(PasteEvent)
	require.True(t, ok, "expected PasteEvent, got %T", got[0])
	require.Equal(t, "git status", pe.Payload)
}

// TestReader_Events_EscFlushTimerDelivers verifies the FlushEsc
// timer path: a lone ESC byte must produce a KeyEsc event after
// the 50ms flush window, not stay buffered indefinitely.
func TestReader_Events_EscFlushTimerDelivers(t *testing.T) {
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
	defer cancel()
	events := r.Events(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(master, "\x1b")
	}()

	// Allow the 50ms flush plus 100ms poll plus margin.
	got := drainEvents(events, 1, 1*time.Second)
	require.Len(t, got, 1, "lone ESC must be emitted within ~150ms")
	require.Equal(t, KeyEvent{Key: KeyEsc}, got[0])
}

// TestReader_Events_SS3FlushTimerDelivers exercises the same flush
// path for an aborted SS3 prelude. A terminal that emits `\eO` and
// then nothing (rare but possible on flaky links and embedded
// firmware emulators) used to leave the picker frozen with the
// parser stuck in stateSS3 — the reader did not arm the flush
// timer for that state, so FlushEsc was never called and the user
// saw no key feedback until some unrelated byte unstuck the
// sequence. The fix wires stateSS3 into the same arm-flush branch
// as stateEsc; this test pins it.
func TestReader_Events_SS3FlushTimerDelivers(t *testing.T) {
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
	defer cancel()
	events := r.Events(ctx)

	go func() {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(master, "\x1bO")
	}()

	// FlushEsc on a stale SS3 emits Esc + 'O' rune. Both must arrive
	// within the same window the bare-Esc path uses.
	got := drainEvents(events, 2, 1*time.Second)
	require.Len(t, got, 2, "stalled SS3 must flush within ~150ms")
	require.Equal(t, KeyEvent{Key: KeyEsc}, got[0])
	require.Equal(t, RuneEvent{R: 'O'}, got[1])
}

// TestReader_Events_SignalDoesNotKillLoop pins the EINTR-resilience
// of the read syscall. SIGWINCH (sent here directly to the test
// process) interrupts both poll() and read() syscalls. The reader
// already handled EINTR on poll, but read() also returns EINTR when
// a signal arrives between poll's POLLIN and the read completing.
// Without continue-on-EINTR-from-read, every terminal resize would
// close the events channel and tear the picker down. This test
// fires SIGWINCH repeatedly while typing, asserting both the
// resize event AND a subsequent keystroke arrive.
func TestReader_Events_SignalDoesNotKillLoop(t *testing.T) {
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
	defer cancel()
	events := r.Events(ctx)

	// Drive: fire SIGWINCH a few times while a stream of keystrokes
	// trickles in. We don't strictly assert on the resize events
	// (some kernels coalesce them); the assertion that matters is
	// that the keystrokes still arrive after the signals.
	go func() {
		time.Sleep(30 * time.Millisecond)
		for range 3 {
			_ = unix.Kill(unix.Getpid(), unix.SIGWINCH)
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = io.WriteString(master, "x")
	}()

	// Drain until we see the 'x' rune. If the loop died, we'll time
	// out on the channel close instead.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("events channel closed unexpectedly — EINTR likely tore down the loop")
			}
			if re, isRune := ev.(RuneEvent); isRune && re.R == 'x' {
				return // success: keystroke after SIGWINCH bursts
			}
			// Other events (resize) are fine; ignore and keep draining.
		case <-deadline:
			t.Fatal("did not receive 'x' keystroke within 2s after SIGWINCH bursts")
		}
	}
}

// Defensive compile-time check: the unix package must export the
// poll constants we depend on. A future Go-x-sys reshuffle that
// removes these would otherwise break us at runtime.
var _ = unix.POLLIN
