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

// TestInputCursorPosition pins the wrap arithmetic the renderer uses
// to land the caret on the right (row, col) when input overflows the
// terminal width. The function maps (initCol, cellsBefore, cols) to
// (rowOffset, col1Indexed) — each case below is hand-traced against
// xterm/iTerm wrap behaviour.
func TestInputCursorPosition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		initCol     int
		cellsBefore int
		cols        int
		wantRow     int
		wantCol     int
	}{
		{"empty-input-on-row-N", 5, 0, 80, 0, 5},
		{"single-cell-no-wrap", 5, 1, 80, 0, 6},
		{"end-of-row-no-wrap", 5, 15, 20, 0, 20},
		// Deferred wrap: 16 x's from col 5 land cells at cols 5-20.
		// Cursor would be at col 21 (one past last); clamp to col 20.
		{"deferred-wrap-clamps-to-last-col", 5, 16, 20, 0, 20},
		// 17th x triggers actual wrap to row N+1 col 1; cursor at col 2.
		{"one-cell-into-wrap-row", 5, 17, 20, 1, 2},
		{"thirty-cells-from-col-5", 5, 30, 20, 1, 15},
		// 36 cells: row N has 16, row N+1 has 20 (deferred again at end).
		{"row-2-deferred-wrap", 5, 36, 20, 1, 20},
		// 37 cells: actual wrap to row N+2 col 1.
		{"row-2-with-overflow", 5, 37, 20, 2, 2},
		{"zero-cols-degrades-gracefully", 5, 10, 0, 0, 5},
		{"col-1-prompt-edge", 1, 20, 20, 0, 20},
		// 21 cells from col 1: 20 fill row N (deferred), 21st on row N+1
		// col 1, cursor right after at col 2.
		{"col-1-just-past-edge", 1, 21, 20, 1, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row, col := InputCursorPosition(tc.initCol, tc.cellsBefore, tc.cols)
			require.Equalf(t, tc.wantRow, row,
				"row mismatch for initCol=%d cells=%d cols=%d", tc.initCol, tc.cellsBefore, tc.cols)
			require.Equalf(t, tc.wantCol, col,
				"col mismatch for initCol=%d cells=%d cols=%d", tc.initCol, tc.cellsBefore, tc.cols)
		})
	}
}

// TestInputExtraRows pins the helper that decides how many wrap rows
// the input occupies below the input start row. The renderer subtracts
// this from heightLimit and adds it into Frame.Size, so any drift here
// directly causes either choice over-draws or stale wrap rows in Pre.
func TestInputExtraRows(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		initCol    int
		cellsTotal int
		cols       int
		want       int
	}{
		{"empty-input", 5, 0, 80, 0},
		{"fits-on-row", 5, 15, 20, 0},
		{"exactly-fills-row-no-wrap", 5, 16, 20, 0},
		{"one-past-fill", 5, 17, 20, 1},
		{"thirty-cells-wraps-once", 5, 30, 20, 1},
		{"thirty-six-cells-wraps-twice", 5, 36, 20, 1},
		{"thirty-seven-cells-wraps-twice", 5, 37, 20, 2},
		{"col-1-twenty-cells", 1, 20, 20, 0},
		{"col-1-twenty-one-cells", 1, 21, 20, 1},
		{"zero-cols-degrades", 5, 30, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := InputExtraRows(tc.initCol, tc.cellsTotal, tc.cols)
			require.Equalf(t, tc.want, got,
				"initCol=%d cells=%d cols=%d", tc.initCol, tc.cellsTotal, tc.cols)
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
