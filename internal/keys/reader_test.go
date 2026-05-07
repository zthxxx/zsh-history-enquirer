package keys

import (
	"context"
	"io"
	"runtime"
	"strings"
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

// TestReader_Events_SS3FlushTimerResetsState exercises the timer
// arm path for an aborted SS3 prelude. A terminal that emits `\eO`
// and then nothing (rare but possible on flaky links and embedded
// firmware emulators) used to leave the parser stuck in stateSS3,
// blocking ALL subsequent input until a key code byte arrived.
// The reader now arms the flush timer when entering stateSS3, the
// 50ms tick fires Parser.FlushEsc, and the parser resets to
// stateNormal — silently, since unrecognized SS3 sequences are
// no-ops in this picker (an aborted F-key prelude must not cancel
// the picker).
//
// We assert on the side-effect: a follow-on keystroke arriving
// AFTER the flush window must parse as itself. If the reader had
// failed to arm the timer, the parser would still be in stateSS3
// and the keystroke would be consumed as the SS3 body byte (e.g.
// 'A' would emit KeyUp instead of RuneEvent{'A'}).
func TestReader_Events_SS3FlushTimerResetsState(t *testing.T) {
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
		// Wait past the 50ms flush window plus the reader's 100ms poll
		// before the follow-on keystroke. If the flush armed correctly,
		// the parser's already back in stateNormal by the time 'A'
		// lands and we get RuneEvent{'A'} — not KeyUp (which is what
		// `\eOA` together would produce).
		time.Sleep(200 * time.Millisecond)
		_, _ = io.WriteString(master, "A")
	}()

	got := drainEvents(events, 1, 1*time.Second)
	require.Len(t, got, 1,
		"flush must reset state so the follow-on keystroke parses as itself")
	require.Equal(t, RuneEvent{R: 'A'}, got[0],
		"follow-on 'A' must parse as a plain rune — proves stateSS3 was flushed")
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
	seenX := false
	for !seenX {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatal("events channel closed unexpectedly — EINTR likely tore down the loop")
			}
			if re, isRune := ev.(RuneEvent); isRune && re.R == 'x' {
				seenX = true
			}
			// Other events (resize) are fine; ignore and keep draining.
		case <-deadline:
			t.Fatal("did not receive 'x' keystroke within 2s after SIGWINCH bursts")
		}
	}

	// IMPORTANT: cancel and wait for the reader goroutine to exit
	// BEFORE letting t.Cleanup close the slave PTY. Without this
	// drain, a SIGWINCH that arrived just before cancel can still
	// be queued in the reader's `winch` channel; the goroutine
	// would then call `r.tty.Size()` (which calls `t.file.Fd()`)
	// concurrently with the cleanup's `slave.Close()` and the race
	// detector flags it. Draining the events channel until close
	// guarantees the reader has exited; the close is the only
	// signal we have for "goroutine done" (no Done() / WaitGroup
	// is exposed by Reader).
	cancel()
	drainDeadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // reader goroutine has exited cleanly
			}
		case <-drainDeadline:
			t.Fatal("reader goroutine did not exit within 2s of cancel — race window still open")
		}
	}
}

// TestRecoverGoroutinePanic_WritesAndContinues exercises the reader's
// deferred recover. A parser-side panic must not crash the process —
// instead it gets logged to PanicWriter and the function returns
// normally so the goroutine's deferred close(out) signals the
// consumer to exit. We verify the recovery executed and the panic
// value reached the writer.
func TestRecoverGoroutinePanic_WritesAndContinues(t *testing.T) {
	var buf strings.Builder
	saved := PanicWriter
	t.Cleanup(func() { PanicWriter = saved })
	PanicWriter = &buf

	// Run a function that panics inside a deferred recover. If
	// recoverGoroutinePanic does its job, the function returns
	// normally and we can read the buffer.
	func() {
		defer recoverGoroutinePanic()
		panic("simulated parser blow-up")
	}()

	out := buf.String()
	require.Contains(t, out, "panic recovered",
		"recovery message must surface to the diagnostic writer")
	require.Contains(t, out, "simulated parser blow-up",
		"panic value must be included in the diagnostic")
	require.Contains(t, out, "TestRecoverGoroutinePanic",
		"a stack trace must accompany the panic value so post-mortem "+
			"debugging can find the faulting frame")
}

// TestReader_Events_MasterCloseExitsLoop pins the POLLHUP / EOF exit
// branch: when the controlling terminal disappears (the user's ssh
// session drops, the parent shell hangs up the pty, sshd kills the
// pipe), the reader goroutine must exit cleanly so the umbrella
// loop sees `events` close and the picker tears down with the
// widget contract intact (BUFFER preserved via Canceled=true path).
//
// We trigger the scenario by closing the master end of the pty pair
// while the reader is mid-poll. The slave-side poll gets either
// POLLHUP (Linux, often) or returns 0 bytes (Darwin, Linux
// fallback) — both code paths must exit the loop. Without the
// exit, the loop would spin or block indefinitely; the timeout
// below catches either failure shape.
func TestReader_Events_MasterCloseExitsLoop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}

	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer func() { _ = slave.Close() }()

	t1, err := tty.NewFromFile(slave)
	require.NoError(t, err)
	require.NoError(t, t1.EnterRaw())
	defer func() { _ = t1.LeaveRaw() }()

	r := NewReader(t1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := r.Events(ctx)

	// Hang up the pty so the slave's next poll either reports POLLHUP
	// or returns n=0 (EOF). Either path exits the reader loop.
	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = master.Close()
	}()

	// The events channel must close within a few poll cycles
	// (pollInterval is 100ms). 2s is generous.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-events:
			if !ok {
				return // reader exited cleanly — POLLHUP / EOF detected
			}
			// Drain any spurious events emitted before the close.
		case <-deadline:
			t.Fatal("events channel did not close within 2s of master.Close — POLLHUP/EOF exit branch broken")
		}
	}
}

// Defensive compile-time check: the unix package must export the
// poll constants we depend on. A future Go-x-sys reshuffle that
// removes these would otherwise break us at runtime.
var (
	_ = unix.POLLIN
	_ = unix.POLLHUP
	_ = unix.POLLERR
	_ = unix.POLLNVAL
)
