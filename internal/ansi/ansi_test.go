package ansi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCursorTo(t *testing.T) {
	t.Parallel()
	require.Equal(t, "\x1b[3;7H", CursorTo(3, 7))
}

func TestCursorToCol(t *testing.T) {
	t.Parallel()
	require.Equal(t, "\x1b[12G", CursorToCol(12))
}

func TestCursorUp(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", CursorUp(0))
	require.Equal(t, "", CursorUp(-3))
	require.Equal(t, "\x1b[2A", CursorUp(2))
}

func TestCursorDown(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", CursorDown(0))
	require.Equal(t, "\x1b[5B", CursorDown(5))
}

func TestEraseLines(t *testing.T) {
	t.Parallel()
	got := EraseLines(2)
	// down 2 + (erase line + prev line) × 2.
	require.Contains(t, got, CursorDown(2))
	require.Contains(t, got, EraseLine)
}

func TestEraseLinesZero(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", EraseLines(0))
	require.Equal(t, "", EraseLines(-1))
}

// TestCursorPrevLine — used by the renderer when rewinding past
// soft-wrapped rows. n=3 → "\e[3F" by spec.
func TestCursorPrevLine(t *testing.T) {
	t.Parallel()
	require.Equal(t, "", CursorPrevLine(0), "n=0 must be a no-op")
	require.Equal(t, "", CursorPrevLine(-2), "n<0 must be a no-op")
	require.Equal(t, "\x1b[3F", CursorPrevLine(3))
}

// TestPublicConstants pins the wire bytes for the CSI constants. If
// any of these change the renderer breaks silently — locking them
// here surfaces the change at review time, not at runtime.
func TestPublicConstants(t *testing.T) {
	t.Parallel()
	require.Equal(t, "\x1b[", CSI)
	require.Equal(t, "\x1b[2K", EraseLine)
	require.Equal(t, "\x1b[0K", EraseLineEnd)
	require.Equal(t, "\x1b[0J", EraseDisplayDown)
	require.Equal(t, "\x1b[?25l", HideCursor)
	require.Equal(t, "\x1b[?25h", ShowCursor)
	require.Equal(t, "\x1b[?2004h", BracketedPasteOn)
	require.Equal(t, "\x1b[?2004l", BracketedPasteOff)
	require.Equal(t, "\x1b[6n", DSRCursor)
}
