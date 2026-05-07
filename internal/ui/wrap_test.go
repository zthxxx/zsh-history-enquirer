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

// TestWrappedRowCount_TabAdvancesToTabstop pins the regression
// where literal `\t` inside a history entry was counted as 0 cells
// (runewidth's default), silently undercounting wrap rows for
// entries that contain tabs (heredoc indentation, multi-line shell
// snippets pasted into the prompt). Tab now advances to the next
// 8-cell tabstop relative to the line's starting column, matching
// standard terminal behaviour.
func TestWrappedRowCount_TabAdvancesToTabstop(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		text string
		cols int
		want int
	}{
		// "  ab\tcd": pointer (2) → col 2; "ab" → col 4; "\t" → col 8;
		// "cd" → col 10. Fits in 80-col terminal as 1 row.
		{"tab-in-line-fits", "ab\tcd", 80, 1},
		// Same line at cols=10: 10 cells exactly. ceil(10/10) = 1.
		{"tab-in-line-tight-fit", "ab\tcd", 10, 1},
		// Same line at cols=8: 10 cells, ceil(10/8) = 2 rows.
		// (Previously: cell-width thought it was 6 cells, ceil(6/8)=1 →
		// undercount that the picker would clamp on.)
		{"tab-in-line-narrow-wrap", "ab\tcd", 8, 2},
		// Tab at the start of a continuation line (after \n) starts
		// at col 0, advances to col 8, then "x" → col 9. ceil(9/80) = 1.
		// With first-line "a" + pointer = 3 cells, second line as above.
		// Total = 1 + 1 = 2 rows.
		{"tab-on-continuation-line", "a\n\tx", 80, 2},
		// Multiple tabs: "\t\t" at col 0 → col 8 → col 16 = 16 cells.
		{"two-tabs-back-to-back", "\t\t", 80, 1},
		// Tab when already on a tabstop must advance, not stay put.
		// "12345678\tx" at col 0: 8 chars get to col 8; tab advances
		// to col 16; "x" → col 17. 17 cells.
		{"tab-on-tabstop-still-advances", "12345678\tx", 80, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := WrappedRowCount(tc.text, tc.cols)
			require.Equalf(t, tc.want, got,
				"text=%q cols=%d → got %d rows, want %d", tc.text, tc.cols, got, tc.want)
		})
	}
}

// TestRowCellWidth pins the helper directly so the tabstop math is
// testable without the wrap-rows arithmetic on top.
func TestRowCellWidth(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		line     string
		startCol int
		want     int
	}{
		{"empty", "", 0, 0},
		{"plain-ascii", "abc", 0, 3},
		{"tab-from-zero", "\t", 0, 8},
		{"tab-from-mid", "\t", 3, 8},
		{"tab-on-tabstop-advances", "\t", 8, 16},
		{"tab-then-text", "\tx", 0, 9},
		{"text-then-tab", "ab\tx", 0, 9},
		{"with-pointer-start", "ab\tx", 2, 9},
		{"two-tabs", "\t\t", 0, 16},
		// CJK glyph after tab: tab → 8, 你 → 10, 好 → 12.
		{"cjk-after-tab", "\t你好", 0, 12},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rowCellWidth(tc.line, tc.startCol)
			require.Equalf(t, tc.want, got,
				"line=%q start=%d → got %d, want %d", tc.line, tc.startCol, got, tc.want)
		})
	}
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
