package ui

import "github.com/mattn/go-runewidth"

// CellWidth returns the number of terminal cells the string s occupies
// when rendered in a typical UTF-8-aware terminal. East Asian wide
// glyphs (CJK ideographs, fullwidth punctuation, emoji) consume 2
// cells per rune; everything else consumes 1. Combining marks and
// zero-width joiners contribute 0.
//
// Callers that need a column-arithmetic value (cursor positioning,
// wrap math, picker init column) should use this helper rather than
// utf8.RuneCountInString or len() — the former under-counts CJK by
// one cell per glyph, the latter over-counts every multi-byte rune by
// the byte-count factor (2-4×). Both produce visible mis-alignments
// against the prompt and the wrapped picker body.
//
// We delegate to mattn/go-runewidth (used by every other Charm /
// bubbletea / cobra-style TUI in the Go ecosystem) so the East Asian
// Width and emoji presentation tables stay current with Unicode
// updates without us re-vendoring them.
func CellWidth(s string) int {
	return runewidth.StringWidth(s)
}
