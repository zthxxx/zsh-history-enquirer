// Package ui implements the picker's model/update/render layer.
//
// Read-order for new contributors:
//
//  1. spec/40-rendering.md, spec/50-keybindings.md
//  2. design/50-ui.md
//  3. wrap.go (this file) — geometry helpers
//  4. model.go         — pure state struct
//  5. update.go        — pure transition function
//  6. render.go        — pure frame builder
//  7. throttle.go      — leading-edge timer
package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

// PointerWidth is the number of cells reserved for the selection
// pointer in front of every visible choice (matches the legacy
// implementation: a 2-cell glyph).
const PointerWidth = 2

// tabStop is the cell distance between hardware tabstops. Most
// terminals use 8 by default. We use the same constant because the
// pointer-aware Tab math has to predict where the terminal will
// land after a `\t`, and getting it wrong by even a few cells
// causes wrap-row miscounts on entries that contain literal tabs
// (heredoc indentation, multi-line shell snippets pasted into the
// prompt). Hardware tabstop overrides are unusual enough to be out
// of scope.
const tabStop = 8

// rowCellWidth returns the visible cell count of a single logical
// line, accounting for `\t` advancing to the next `tabStop`. Used
// only by WrappedRowCount because Tab is the only printable byte
// whose cell footprint depends on the cursor's starting column —
// runewidth (which we use everywhere else for non-line-level
// calculations) treats `\t` as width 0, which silently underflows
// the wrap math whenever a history entry contains a literal tab.
//
// startCol is the cell column the rendering starts at (0 for the
// pointer-less continuation lines, PointerWidth for the first
// line). The function returns how far the line extends.
func rowCellWidth(line string, startCol int) int {
	col := startCol
	for _, r := range line {
		if r == '\t' {
			// Advance to the next multiple of tabStop. If we're
			// exactly on a tabstop, advance to the next one (a Tab
			// always moves the cursor at least one cell).
			col = ((col / tabStop) + 1) * tabStop
			continue
		}
		col += runewidth.RuneWidth(r)
	}
	return col
}

// InputCursorPosition reports the (row, col) where the visual cursor
// sits after `cellsBefore` cells of input have been written, given an
// input row that started at 1-indexed column `initCol` of a `cols`-wide
// terminal. Row is the 0-indexed offset from the input row; col is the
// 1-indexed terminal column matching ANSI's CSI G escape.
//
// The convention is "cursor sits one cell to the right of the last
// typed char, on the row that last char actually landed on" — matching
// readline / fish / vim. When the last char exactly fills the row
// (cursor would be at col cols+1) we clamp to col cols, the deferred-
// wrap state every common terminal renders. Without the clamp the
// renderer would jump the visible caret to col 1 of the next row even
// though no character has wrapped yet, surprising the user mid-type.
func InputCursorPosition(initCol, cellsBefore, cols int) (row, col int) {
	if cols <= 0 {
		return 0, initCol
	}
	if cellsBefore <= 0 {
		return 0, initCol
	}
	lastCharFlat := initCol + cellsBefore - 1
	if lastCharFlat < 1 {
		return 0, 1
	}
	row = (lastCharFlat - 1) / cols
	colInRow := ((lastCharFlat - 1) % cols) + 1
	col = colInRow + 1
	if col > cols {
		col = cols
	}
	return row, col
}

// InputExtraRows reports how many wrap rows below the input start row
// the input occupies when `cellsTotal` cells are written starting at
// 1-indexed column `initCol` of a `cols`-wide terminal. Returns 0
// when the input fits on row N (or is empty). The renderer uses this
// to (a) reserve choice space (heightLimit -= inputExtra) and
// (b) compute the total body row count for renderPre/renderPost.
//
// We treat exactly-filling-the-row as 0 extra rows: 16 cells from
// initCol=5 on a 20-col terminal land the last cell at col 20 with the
// cursor in deferred wrap on row N — visually still one row.
func InputExtraRows(initCol, cellsTotal, cols int) int {
	if cellsTotal <= 0 || cols <= 0 {
		return 0
	}
	lastCellCol := initCol + cellsTotal - 1
	if lastCellCol < 1 {
		return 0
	}
	return (lastCellCol - 1) / cols
}

// WrappedRowCount returns the number of terminal rows that the given
// text occupies when printed at column 0 of a `cols`-wide terminal,
// after prefixing each visual line with the pointer.
//
// Rules (mirroring the legacy `calcTextTakeRows`):
//   - text is split on `\n` into logical lines
//   - each logical line takes ceil(width / cols) rows, with empty
//     lines counting as 1
//   - the pointer is conceptually prefixed only to the first logical
//     line, but the row math treats every line as wrapping
//     independently — that matches what the legacy implementation
//     does and produces the right result for the *first* line, and
//     a slight over-estimate for continuation lines (which is safer
//     than under-estimating: we draw one fewer match instead of
//     overflowing).
//
// We count *cells* via runewidth (mattn/go-runewidth). East Asian
// wide glyphs (CJK, fullwidth punctuation, emoji) consume 2 cells
// per rune; combining marks 0; everything else 1. `\t` advances to
// the next 8-cell tabstop relative to the line's starting column,
// matching standard terminal behaviour — the previous form
// (`CellWidth(line)`) treated `\t` as 0 cells and silently
// undercounted lines with literal tabs.
func WrappedRowCount(text string, cols int) int {
	if cols <= 0 {
		return 1
	}
	rows := 0
	first := true
	for _, line := range strings.Split(text, "\n") {
		// Pointer prefix only on the first logical line. Subsequent
		// continuation lines start at col 0; both feed the same
		// rowCellWidth helper so Tab-advance math is consistent.
		startCol := 0
		if first {
			startCol = PointerWidth
			first = false
		}
		width := rowCellWidth(line, startCol)
		if width <= 0 {
			rows++
			continue
		}
		rows += (width + cols - 1) / cols
	}
	if rows == 0 {
		rows = 1
	}
	return rows
}
