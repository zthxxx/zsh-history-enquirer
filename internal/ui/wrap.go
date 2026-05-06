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
	"unicode/utf8"
)

// PointerWidth is the number of cells reserved for the selection
// pointer in front of every visible choice (matches the legacy
// implementation: a 2-cell glyph).
const PointerWidth = 2

// WrappedRowCount returns the number of terminal rows that the given
// text occupies when printed at column 0 of a `cols`-wide terminal,
// after prefixing each visual line with the pointer.
//
// Rules (mirroring the legacy `calcTextTakeRows`):
//   - text is split on `\n` into logical lines
//   - each logical line takes ceil(len(line) / cols) rows, with empty
//     lines counting as 1
//   - the pointer is conceptually prefixed only to the first logical
//     line, but the row math treats every line as wrapping
//     independently — that matches what the legacy implementation
//     does and produces the right result for the *first* line, and
//     a slight over-estimate for continuation lines (which is safer
//     than under-estimating: we draw one fewer match instead of
//     overflowing).
//
// We count *runes* (utf8.RuneCountInString) rather than bytes. For
// ASCII this equals cells exactly. For CJK it slightly under-counts
// (a CJK glyph takes 2 cells but is 1 rune); for mixed text the
// estimate is between cell-true and rune-true. This matches the
// legacy Node.js implementation's behaviour (JS `String.length`
// counts UTF-16 code units, which approximates runes for the BMP)
// — both ports occasionally show one extra match on lines that
// would just barely overflow if cells were counted exactly.
func WrappedRowCount(text string, cols int) int {
	if cols <= 0 {
		return 1
	}
	rows := 0
	first := true
	for _, line := range strings.Split(text, "\n") {
		// Pointer prefix only on the first logical line. Subsequent
		// continuation lines don't get one (they are wraps, not new
		// pointer-eligible items), but we still factor the prefix
		// into the *first* line's wrap math.
		width := utf8.RuneCountInString(line)
		if first {
			width += PointerWidth
			first = false
		}
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
