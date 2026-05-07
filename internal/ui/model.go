package ui

import (
	"slices"

	"github.com/zthxxx/zsh-history-enquirer/internal/search"
)

// DefaultMaxLimit matches the legacy `limit: 15`.
const DefaultMaxLimit = 15

// Model carries every piece of state the picker needs to render a
// frame. Every field here is exported so tests can construct a Model
// directly — there is no hidden mutable state (the throttle, the
// renderer, and the keystreamer all live outside the Model).
type Model struct {
	// Geometry — captured at startup and updated on resize.
	InitCol int // 1-indexed column of the prompt when the picker started
	InitRow int // 1-indexed row of the prompt when the picker started
	Width   int // terminal width  (cols)
	Height  int // terminal height (rows)

	// Data.
	Choices []string // immutable post-loader output
	Filter  []string // current filtered-and-rotated view; rotated in place by Up/Down
	Idx     int      // selection within Filter
	Limit   int      // dynamic, capped at MaxLimit

	// Input.
	Input string // typed input
	// Cursor is the display-width offset of the caret from the start
	// of Input, expressed in terminal cells. It is what the renderer
	// adds to InitCol to position the cursor on the input row.
	//
	// Cell-width via CellWidth (mattn/go-runewidth): exact for
	// every script the Unicode East Asian Width tables cover —
	// ASCII, Latin-extended, Greek, Cyrillic, Hebrew, Arabic, CJK,
	// emoji, fullwidth punctuation. Bytes were wrong for all
	// non-ASCII input (over-counting by 2-3×); rune-count was off
	// for CJK and emoji (under-counting by 1 cell each). CellWidth
	// is the fix.
	Cursor int

	// Configuration.
	MaxLimit int // typically DefaultMaxLimit

	// Status flags.
	Submitted bool
	Canceled  bool
	Result    string
}

// NewModel constructs a Model with sensible defaults given an initial
// input, captured geometry, and the post-loader choices list.
func NewModel(input string, choices []string, rows, cols, initRow, initCol, maxLimit int) *Model {
	if maxLimit <= 0 {
		maxLimit = DefaultMaxLimit
	}
	m := &Model{
		InitCol:  initCol,
		InitRow:  initRow,
		Width:    cols,
		Height:   rows,
		Choices:  choices,
		Input:    input,
		Cursor:   CellWidth(input),
		MaxLimit: maxLimit,
	}
	m.recomputeFilter()
	return m
}

// recomputeFilter rebuilds Filter from Choices using the current
// Input. The Visible window points at the start of the filtered list.
//
// search.AndFilter aliases Choices when the input has no tokens (a
// documented zero-copy fast path). Filter is mutated in place by
// rotateUp / rotateDown, so we clone in that case to keep the
// "Choices is immutable post-loader" invariant intact. Without the
// clone, scrolling the empty-input view scrambles Choices, and a
// subsequent recomputeFilter — e.g. after Ctrl-U — returns a
// permuted history that no longer matches reverse-chronological
// order.
func (m *Model) recomputeFilter() {
	tokens := search.Tokenize(m.Input)
	filter := search.AndFilter(m.Choices, tokens)
	if len(tokens) == 0 {
		filter = slices.Clone(filter)
	}
	m.Filter = filter
	m.Idx = 0
}

// rotateUp rotates the filtered list in-place by 1 (last element
// becomes first). Used by the up-arrow scroll.
func (m *Model) rotateUp(n int) {
	if len(m.Filter) == 0 || n <= 0 {
		return
	}
	n %= len(m.Filter)
	if n == 0 {
		return
	}
	// move the last n elements to the front
	tail := slices.Clone(m.Filter[len(m.Filter)-n:])
	copy(m.Filter[n:], m.Filter[:len(m.Filter)-n])
	copy(m.Filter, tail)
}

// rotateDown rotates the filtered list in-place by 1 (first element
// becomes last).
func (m *Model) rotateDown(n int) {
	if len(m.Filter) == 0 || n <= 0 {
		return
	}
	n %= len(m.Filter)
	if n == 0 {
		return
	}
	head := slices.Clone(m.Filter[:n])
	copy(m.Filter, m.Filter[n:])
	copy(m.Filter[len(m.Filter)-n:], head)
}

// Focused returns the currently highlighted entry, or "" if Filter
// is empty.
func (m *Model) Focused() string {
	if len(m.Filter) == 0 || m.Idx >= len(m.Filter) {
		return ""
	}
	return m.Filter[m.Idx]
}

// SubmitResult is the value the picker should write to stdout at the
// end of the session. Cancel preserves the typed input; submit prefers
// the focused entry, falling back to Input if there are no matches.
func (m *Model) SubmitResult() string {
	if m.Canceled {
		return m.Input
	}
	if f := m.Focused(); f != "" {
		return f
	}
	return m.Input
}
