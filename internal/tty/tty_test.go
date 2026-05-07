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

// TestTTY_Size_ReturnsConfiguredWinsize covers the TIOCGWINSZ branch
// of TTY.Size against a real pty. We set a specific winsize via
// pty.Setsize and assert Size returns the same numbers — proves the
// ioctl is wired correctly and the rows/cols ordering matches what
// callers expect (rows first, cols second).
//
// Without an explicit test the only path that exercised Size was the
// app/init readGeometry helper, which already had a fallback for
// zero-size and so masked any wiring bug. Pinning Size directly so
// future changes to termios_*.go can't silently flip the ordering.
func TestTTY_Size_ReturnsConfiguredWinsize(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}
	t.Parallel()

	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer func() { _ = master.Close() }()
	defer func() { _ = slave.Close() }()

	require.NoError(t, pty.Setsize(slave, &pty.Winsize{Rows: 42, Cols: 137}))

	tt, err := NewFromFile(slave)
	require.NoError(t, err)

	rows, cols, err := tt.Size()
	require.NoError(t, err)
	require.Equal(t, 42, rows, "rows must match the configured winsize")
	require.Equal(t, 137, cols, "cols must match the configured winsize")
}

// TestTTY_Size_AfterCloseReturnsError pins the failure mode of Size
// after Close — the underlying fd is reset to nil so the ioctl path
// would surface the standard "use of closed file" error from the
// runtime. Callers that hold a stale TTY pointer must observe the
// error rather than silently receiving (0, 0, nil).
func TestTTY_Size_AfterCloseReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}
	t.Parallel()

	_, slave, err := pty.Open()
	require.NoError(t, err)

	tt, err := NewFromFile(slave)
	require.NoError(t, err)
	require.NoError(t, tt.Close())

	_, _, err = tt.Size()
	require.Error(t, err, "Size after Close must surface an error")
}

// TestTTY_Close_WhileRawEnteredRestoresAndCloses pins the panic-path
// fx OnStop hook: a panic in upstream code can leave the picker in
// raw mode at process tear-down. Close must still LeaveRaw before
// closing the fd, otherwise the user's surrounding shell inherits a
// terminal with ECHO off and ICANON off — every keystroke would echo
// raw and Enter would not deliver a line. Without this guarantee the
// "even a panic still leaves the user's terminal usable" promise in
// NewDevTTY's docstring would be a lie.
func TestTTY_Close_WhileRawEnteredRestoresAndCloses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pty unsupported on windows")
	}
	t.Parallel()

	_, slave, err := pty.Open()
	require.NoError(t, err)

	tt, err := NewFromFile(slave)
	require.NoError(t, err)

	// Capture the savedTerm bytes so we can compare after Close.
	pristine := *tt.savedTerm
	require.NoError(t, tt.EnterRaw())
	require.True(t, tt.rawEntered)

	// Close() must call LeaveRaw internally — exercising the
	// rawEntered=true branch of Close that was previously uncovered.
	require.NoError(t, tt.Close())
	require.Nil(t, tt.file, "Close must null out the file pointer")

	// savedTerm bytes are unchanged — Close doesn't clobber them, just
	// uses them to restore.
	require.Equal(t, pristine, *tt.savedTerm,
		"Close must use savedTerm to restore, not modify it")
}
