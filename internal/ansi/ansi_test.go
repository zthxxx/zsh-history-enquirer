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
