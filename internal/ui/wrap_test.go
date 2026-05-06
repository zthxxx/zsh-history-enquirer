package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestWrappedRowCount_SingleLineNoWrap(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, WrappedRowCount("hello", 80))
}

func TestWrappedRowCount_SingleLineWraps(t *testing.T) {
	t.Parallel()
	// 80 chars + 2-char pointer = 82 → 2 rows on an 80-col terminal.
	require.Equal(t, 2, WrappedRowCount(strings.Repeat("x", 80), 80))
}

func TestWrappedRowCount_MultiLine(t *testing.T) {
	t.Parallel()
	// 3 logical lines, each fitting within cols.
	require.Equal(t, 3, WrappedRowCount("a\nb\nc", 80))
}

func TestWrappedRowCount_EmptyLineCountsAsOne(t *testing.T) {
	t.Parallel()
	require.Equal(t, 3, WrappedRowCount("\n\n", 80))
}

func TestWrappedRowCount_ZeroCols(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, WrappedRowCount("anything", 0))
}

func TestProperty_WrappedRowCount_Monotonic(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringMatching(`[a-z\n]{0,40}`).Draw(rt, "s")
		cols := rapid.IntRange(1, 200).Draw(rt, "cols")

		// Adding a character can only ever increase or keep the row
		// count (assuming we don't introduce a newline that would
		// have happened anyway; we restrict the alphabet so newline
		// is the only way to add a row out of order — which still
		// only adds rows).
		base := WrappedRowCount(s, cols)
		bigger := WrappedRowCount(s+"x", cols)
		require.GreaterOrEqual(rt, bigger, base)
	})
}
