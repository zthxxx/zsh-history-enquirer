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

	"github.com/rivo/uniseg"
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
// uniseg (like every other width-aware library) reports `\t` as
// width 0, which silently underflows the wrap math whenever a
// history entry contains a literal tab.
//
// We iterate grapheme clusters (not runes) so an emoji ZWJ family
// or a decomposed accented Latin letter contributes its rendered
// cell footprint as a single unit — `e + combining-acute` reports
// as 1 cell, not 1 + 0.
//
// startCol is the cell column the rendering starts at (0 for the
// pointer-less continuation lines, PointerWidth for the first
// line). The function returns how far the line extends.
func rowCellWidth(line string, startCol int) int {
	col := startCol
	g := uniseg.NewGraphemes(line)
	for g.Next() {
		cluster := g.Str()
		if cluster == "\t" {
			// Advance to the next multiple of tabStop. If we're
			// exactly on a tabstop, advance to the next one (a Tab
			// always moves the cursor at least one cell).
			col = ((col / tabStop) + 1) * tabStop
			continue
		}
		col += uniseg.StringWidth(cluster)
	}
	return col
}

// InputCursorPosition walks `input` rune-by-rune (up to `cellsBefore`
// cells via runewidth) and returns the (row, col) where the visual
// cursor rests on a `cols`-wide terminal whose input row started at
// 1-indexed column `initCol`. Row is the 0-indexed offset from the
// input row; col is the 1-indexed terminal column matching ANSI's
// CSI G escape.
//
// We grapheme-walk rather than use a closed-form division because of
// two terminal quirks no cell-only formula handles correctly:
//
//  1. Wide-grapheme-over-wrap-boundary. A 2-cell CJK glyph (or an
//     emoji ZWJ family pictograph) at col `cols` has only 1 free
//     cell — every common terminal soft-wraps the entire grapheme
//     to the next row. A division-based formula assumes contiguous
//     cell packing and miscounts cursor col by 1 cell for such
//     inputs (a pasted CJK suffix on a long ASCII filter).
//  2. Deferred wrap. After writing exactly `cols` cells starting at
//     col 1, the cursor sits at "col cols+1" but renders at the last
//     visible cell. We clamp to col `cols` so subsequent CursorToCol
//     escapes don't get clipped silently by the terminal.
//
// `\t` and `\n` never appear in m.Input (sanitized at append time —
// see `sanitizeInputRune`), so we don't handle them. Iterating
// grapheme clusters via uniseg means a decomposed `e + combining
// acute` ("é") moves the cursor 1 cell, not 1 + 0; same for emoji
// ZWJ families.
func InputCursorPosition(initCol int, input string, cellsBefore, cols int) (row, col int) {
	if cols <= 0 {
		return 0, initCol
	}
	cur := initCol
	consumed := 0
	g := uniseg.NewGraphemes(input)
	for g.Next() {
		if consumed >= cellsBefore {
			break
		}
		w := uniseg.StringWidth(g.Str())
		if w == 0 {
			continue
		}
		if cur+w-1 > cols {
			row++
			cur = 1
		}
		cur += w
		consumed += w
	}
	if cur > cols {
		cur = cols
	}
	return row, cur
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
// We count *cells* via uniseg (rivo/uniseg, the same library
// CellWidth wraps). East Asian wide glyphs (CJK, fullwidth
// punctuation, emoji) consume 2 cells per cluster; combining marks
// merge into the preceding cluster; everything else 1. `\t` advances
// to the next 8-cell tabstop relative to the line's starting column,
// matching standard terminal behaviour — uniseg (like every other
// width library) treats `\t` as 0 cells, so the explicit tabstop
// math here is what keeps wrap rows accurate when a history entry
// contains a literal tab.
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
