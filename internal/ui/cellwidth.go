package ui

import "github.com/rivo/uniseg"

// CellWidth returns the number of terminal cells the string s occupies
// when rendered in a typical UTF-8-aware terminal. East Asian wide
// glyphs (CJK ideographs, fullwidth punctuation, emoji) consume 2
// cells per glyph; everything else consumes 1. Combining marks and
// zero-width joiners contribute 0 — they merge into the preceding
// grapheme cluster rather than reserving their own cell.
//
// Callers that need a column-arithmetic value (cursor positioning,
// wrap math, picker init column) should use this helper rather than
// utf8.RuneCountInString or len() — the former under-counts CJK by
// one cell per glyph, the latter over-counts every multi-byte rune by
// the byte-count factor (2-4×). Both produce visible mis-alignments
// against the prompt and the wrapped picker body.
//
// We delegate to rivo/uniseg's grapheme-cluster-aware string width.
// The earlier mattn/go-runewidth measured per-rune width which
// over-counted decomposed glyphs (e + combining acute = "é" rendered
// as one cell) and emoji ZWJ sequences (man+ZWJ+woman+ZWJ+girl, the
// family pictograph, rendered as one wide cell). uniseg builds on
// the same Unicode tables but groups runes into grapheme clusters
// first — matching what every modern terminal actually paints.
func CellWidth(s string) int {
	return uniseg.StringWidth(s)
}
