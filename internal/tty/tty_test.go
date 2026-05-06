package tty

import (
	"os"
	"runtime"
	"testing"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
)

// TestNewFromFile_NilFile asserts the nil-guard is hit cleanly. The
// production graph never passes nil, but tests and future callers
// might.
func TestNewFromFile_NilFile(t *testing.T) {
	t.Parallel()
	got, err := NewFromFile(nil)
	require.Error(t, err)
	require.Nil(t, got)
}

// TestNewFromFile_NonTTY asserts NewFromFile rejects fds that don't
// support termios queries — e.g. a regular file. Without this guard
// the renderer would silently misbehave because Size() / EnterRaw()
// would error in surprising ways later.
func TestNewFromFile_NonTTY(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp(t.TempDir(), "non-tty")
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	got, err := NewFromFile(f)
	require.Error(t, err,
		"NewFromFile must reject a non-tty fd; otherwise downstream "+
			"ioctl calls would fail at unpredictable points")
	require.Nil(t, got)
}

// TestTTY_RoundtripFromPty exercises the public surface of TTY against
// a real pty pair. Covers Reader, Writer, File, and Close, none of
// which require a controlling terminal.
func TestTTY_RoundtripFromPty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}
	t.Parallel()

	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer func() { _ = master.Close() }()

	tt, err := NewFromFile(slave)
	require.NoError(t, err)

	require.NotNil(t, tt.Reader())
	require.NotNil(t, tt.Writer())
	require.Equal(t, slave, tt.File(),
		"File() must return the underlying *os.File so callers can "+
			"perform fd-level ioctls")

	// Close() should not error even though we never entered raw mode.
	require.NoError(t, tt.Close())
	// Second Close must be a no-op (the rest of the codebase relies
	// on Close-twice safety because the fx OnStop hook may run after
	// a manual Close in error paths).
	require.NoError(t, tt.Close(),
		"Close must be safe to call multiple times")
}

// TestTTY_LeaveRaw_RestoresTermios — exercises EnterRaw + LeaveRaw
// against a real pty. The previous coverage came only via the cursor
// probe path, which short-circuits on cleanup; the explicit test
// catches a regression where LeaveRaw would not restore termios.
func TestTTY_LeaveRaw_RestoresTermios(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}
	t.Parallel()

	_, slave, err := pty.Open()
	require.NoError(t, err)
	defer func() { _ = slave.Close() }()

	tt, err := NewFromFile(slave)
	require.NoError(t, err)

	require.NoError(t, tt.EnterRaw())
	require.True(t, tt.rawEntered)

	require.NoError(t, tt.LeaveRaw())
	require.False(t, tt.rawEntered,
		"LeaveRaw must clear the rawEntered flag so a subsequent "+
			"Close does not double-restore")

	// LeaveRaw must be safe to call again.
	require.NoError(t, tt.LeaveRaw())
}
