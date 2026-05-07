package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// stripHighlight removes the bold-cyan SGR pair so tests can match
// against the plain payload that the user effectively sees.
func stripHighlight(s string) string {
	out := strings.ReplaceAll(s, highlightOn, "")
	out = strings.ReplaceAll(out, highlightOff, "")
	return out
}

func TestRender_FirstFrameWritesPrompt(t *testing.T) {
	t.Parallel()

	m := NewModel("git", []string{"git status", "git log"}, 15, 80, 1, 5, DefaultMaxLimit)
	frame := m.Render(RenderOptions{PrevSize: 0})

	require.NotEmpty(t, frame.Body)
	// Body must echo the input and at least one entry. Strip the
	// highlight escapes inserted by the matcher so we compare
	// against the user-visible characters.
	plain := stripHighlight(frame.Body)
	require.Contains(t, plain, "git")
	require.Contains(t, plain, "git status")
	require.Greater(t, frame.Size, 0)
	require.Greater(t, frame.Limit, 0)
}

func TestRender_DynamicLimitRespectsHeight(t *testing.T) {
	t.Parallel()

	// 5-row terminal → heightLimit = 2. With single-line entries, we
	// must not draw more than 2 of them.
	choices := []string{"a", "b", "c", "d", "e"}
	m := NewModel("", choices, 5, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})
	require.Equal(t, 2, frame.Limit)
}

func TestRender_PointerOnFocused(t *testing.T) {
	t.Parallel()

	m := NewModel("", []string{"a", "b"}, 15, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})
	// Focus is on index 0.
	require.Contains(t, frame.Body, pointerSelected+"a")
	require.Contains(t, frame.Body, pointerUnselected+"b")
}

func TestRender_NoMatchesShowsHint(t *testing.T) {
	t.Parallel()

	m := NewModel("zzzzz-no-such", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})
	require.Contains(t, frame.Body, "no matches")
}

func TestRender_PreClearsPrevious(t *testing.T) {
	t.Parallel()

	m := NewModel("", []string{"a"}, 15, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{PrevSize: 3})
	// Should contain three "erase line" sequences for a 3-row body.
	count := strings.Count(frame.Pre, "\x1b[2K")
	require.Equal(t, 3, count)
}

func TestRender_PostMovesCursorToInput(t *testing.T) {
	t.Parallel()

	m := NewModel("hi", []string{"hello", "hint"}, 15, 80, 1, 5, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})
	// Post must position the cursor at initCol + len(input) = 5 + 2 = 7.
	require.Contains(t, frame.Post, "\x1b[7G")
}

// TestRender_PreAtScrollBoundary exercises renderPre when prevSize is
// larger than (terminal rows - cursor row) — the case where the
// previous frame caused the terminal to scroll. The renderer cannot
// see scrolling, so it must still emit a deterministic Pre sequence
// (erase + cursor restore) that does not assume the input row stayed
// in place. This locks down the contract that renderPre always emits
// `prevSize` erase-line sequences regardless of geometry.
func TestRender_PreAtScrollBoundary(t *testing.T) {
	t.Parallel()

	// 5-row terminal, input on row 1, prevSize=12 (way more than
	// terminalRows-currentRow=4). The renderer should still emit
	// 12 erase sequences and a cursor-prev-line for 12.
	m := NewModel("", []string{"a"}, 5, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{PrevSize: 12})

	require.Equal(t, 12, strings.Count(frame.Pre, "\x1b[2K"),
		"Pre must erase exactly prevSize lines")
	// CursorPrevLine(12) emits "\x1b[12F".
	require.Contains(t, frame.Pre, "\x1b[12F",
		"Pre must walk back prevSize rows even past the visible window")
}

// TestRender_PreFirstFrame confirms that PrevSize=0 (the very first
// render in a session) produces a minimal Pre that only resets the
// input column — no erase-line sequences below.
func TestRender_PreFirstFrame(t *testing.T) {
	t.Parallel()

	m := NewModel("", []string{"a", "b", "c"}, 15, 80, 1, 4, DefaultMaxLimit)
	frame := m.Render(RenderOptions{PrevSize: 0})

	require.NotContains(t, frame.Pre, "\x1b[2K",
		"first frame's Pre should not contain erase-line escapes")
	require.Contains(t, frame.Pre, "\x1b[4G",
		"first frame's Pre must place the cursor at initCol=4")
}

// TestRender_LimitMin1WithGiantEntry covers the
// `limit==0 && len(Filter)>0` fallback in renderBody. When the
// only filter entry wraps to more rows than heightLimit, the
// dynamic-limit walk produces limit=0; the fallback bumps it to 1
// so the user sees at least the first row of that giant entry
// rather than an empty picker.
func TestRender_LimitMin1WithGiantEntry(t *testing.T) {
	t.Parallel()

	huge := "huge\n" + strings.Repeat("L\n", 30) + "tail"
	// 5-row terminal — heightLimit = 2; huge wraps to ~32 rows.
	m := NewModel("", []string{huge}, 5, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})

	require.Equal(t, 1, frame.Limit,
		"giant entry alone must still produce limit=1 (drawing at least the head)")
	require.Contains(t, stripHighlight(frame.Body), "huge",
		"the head of the giant entry must be in the rendered body")
}

// TestRender_NegativeIdxClampedToZero exercises the `m.Idx < 0`
// clamp in renderBody. A model that's been mutated to a negative
// Idx (defensive guard rail) must still render with focus on row 0.
func TestRender_NegativeIdxClampedToZero(t *testing.T) {
	t.Parallel()

	m := NewModel("", []string{"a", "b"}, 15, 80, 1, 1, DefaultMaxLimit)
	m.Idx = -7 // pathological state — guard must clamp it
	frame := m.Render(RenderOptions{})

	require.Equal(t, 0, m.Idx, "negative Idx must clamp to 0")
	require.Contains(t, frame.Body, pointerSelected+"a",
		"row 0 must be the focused entry after clamp")
}

// TestRender_LongInputWraps locks down the wrap-aware Frame contract
// for inputs that overflow the terminal width.
//
// Setup: 30 x's typed at initCol=5 on a 20-col terminal.
//   - Row N has cols 5–20 filled (16 x's).
//   - Row N+1 has cols 1–14 filled (14 more x's), cursor at col 15.
//
// The previous renderer math counted only choice rows in Frame.Size and
// emitted CursorPrevLine(1) + CursorToCol(35). Col 35 > 20 → terminal
// clamps to col 20 of the wrap row, so the caret landed at the end of
// the visible x's instead of where m.Cursor pointed. The fix below
// computes inputExtra (1 wrap row) and folds it into both Size and the
// Post arithmetic.
func TestRender_LongInputWraps(t *testing.T) {
	t.Parallel()

	m := NewModel(strings.Repeat("x", 30), nil, 10, 20, 1, 5, DefaultMaxLimit)
	frame := m.Render(RenderOptions{PrevSize: 0, PrevCursorRow: 0})

	require.Equal(t, 2, frame.Size,
		"Size must include the input wrap row (1) plus the (no matches) row (1)")
	require.Equal(t, 1, frame.CursorRow,
		"CursorRow must point at the wrap row where the caret actually rests")

	// Post must walk up exactly (Size - CursorRow) = 1 row, then place
	// the caret at the col-15 wrap position. Pre-fix it emitted "[35G".
	require.Contains(t, frame.Post, "\x1b[F",
		"Post must walk up 1 row from the bottom of the body to the cursor wrap row")
	require.Contains(t, frame.Post, "\x1b[15G",
		"Post must position the caret at col 15 (where the 30th char's caret rests on the wrap row)")
	require.NotContains(t, frame.Post, "\x1b[35G",
		"Post must not emit a clamped col 35 — terminals would land at col 20 instead")
}

// TestRender_PreWalksUpFromInputWrapCursor pins that the second-frame
// Pre walks up the previous CursorRow before erasing — without this
// step, the input wrap rows above the caret bleed into the next frame.
func TestRender_PreWalksUpFromInputWrapCursor(t *testing.T) {
	t.Parallel()

	m := NewModel(strings.Repeat("x", 30), nil, 10, 20, 1, 5, DefaultMaxLimit)
	// Simulate a second render pass: the previous frame's Post left
	// the cursor at (row N+1, col 15). Pre must walk back up to row N
	// before it can address the input row to redraw.
	frame := m.Render(RenderOptions{PrevSize: 2, PrevCursorRow: 1})

	require.Contains(t, frame.Pre, "\x1b[F",
		"Pre must walk up the previous CursorRow (1) before erasing")
	// Two CSI 2K erase-line sequences for the two prev body rows.
	require.Equal(t, 2, strings.Count(frame.Pre, "\x1b[2K"),
		"Pre must erase exactly PrevSize lines below the input row")
	require.Contains(t, frame.Pre, "\x1b[2F",
		"Pre must walk back up PrevSize lines after erasing to land on row N")
}

// TestRender_WrapInvariantAcrossPasses simulates a typing burst that
// pushes the input across the wrap boundary and back, asserting that
// the (PrevSize, PrevCursorRow) round-trip is self-consistent across
// passes. Concretely: we capture each frame's (Size, CursorRow), feed
// them to the next pass as opts, and assert the renderer never asks
// to walk further up than it can. This catches off-by-one drift in
// either the formula or the bookkeeping in the event loop.
func TestRender_WrapInvariantAcrossPasses(t *testing.T) {
	t.Parallel()

	// Sequence: empty → "x"*15 (fits row) → "x"*30 (wraps once) →
	// "x"*36 (deferred at end of row N+1) → "x"*37 (wraps twice).
	steps := []string{
		"",
		strings.Repeat("x", 15),
		strings.Repeat("x", 30),
		strings.Repeat("x", 36),
		strings.Repeat("x", 37),
		strings.Repeat("x", 30), // Shrinking back down.
		strings.Repeat("x", 1),
	}

	prevSize := 0
	prevCursorRow := 0
	for i, input := range steps {
		m := NewModel(input, nil, 10, 20, 1, 5, DefaultMaxLimit)
		opts := RenderOptions{PrevSize: prevSize, PrevCursorRow: prevCursorRow}
		frame := m.Render(opts)

		// CursorRow must never exceed Size — Post would otherwise emit
		// a negative walk-up which CursorPrevLine cannot represent.
		require.LessOrEqualf(t, frame.CursorRow, frame.Size,
			"step %d (%q): CursorRow (%d) must not exceed Size (%d)",
			i, input, frame.CursorRow, frame.Size)

		// CursorRow >= 0 always.
		require.GreaterOrEqualf(t, frame.CursorRow, 0,
			"step %d (%q): CursorRow must be non-negative", i, input)

		prevSize = frame.Size
		prevCursorRow = frame.CursorRow
	}
}

// TestRender_HeightLimitReservesInputWrapSpace pins that the dynamic
// limit walk subtracts input wrap rows from the available height.
// Without this, a wrapped input + multi-line choices would draw past
// the terminal bottom and the next Pre would erase too few rows.
func TestRender_HeightLimitReservesInputWrapSpace(t *testing.T) {
	t.Parallel()

	// Construct an input that wraps but still passes the AND filter
	// against single-line choices. Tokenize splits on whitespace and
	// dedupes; "x x x x ..." becomes a single token "x" that matches
	// any choice containing the letter x. Total cells of the input row
	// itself drive inputExtra independently of token semantics.
	//
	// 99 cells from initCol=1 on 60-col → lastCellCol=99 → inputExtra=1.
	// heightLimit base = m.Height - 3 = 12 - 3 = 9. With inputExtra=1
	// the dynamic walk caps at 8. Provide 9 single-row "x"-bearing
	// choices and assert only 8 land in the frame.
	input := strings.TrimRight(strings.Repeat("x ", 50), " ")
	choices := []string{"xa", "xb", "xc", "xd", "xe", "xf", "xg", "xh", "xi"}
	m := NewModel(input, choices, 12, 60, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})

	require.Equal(t, 8, frame.Limit,
		"heightLimit must shrink by inputExtra so wrapped input does not push choices off-screen")
	require.Equal(t, 9, frame.Size,
		"Size must include both input wrap rows (1) and choice rows (8)")
	require.Equal(t, 1, frame.CursorRow,
		"cursor sits on wrap row N+1 because m.Cursor lands at the end of a 99-cell input")
}

// TestRender_DynamicLimitMatchesSanitizedRender pins that the
// dynamic-limit walk and the actual render arithmetic agree on
// row counts even when entries contain control bytes that will
// be sanitized to caret notation. The earlier WrappedRowCount(raw)
// path silently undercounted entries with `\x1b` / `\x07` / `\x7f`
// (runewidth treats them as 0 cells), so an entry that actually
// rendered as 10 cells could slip into a window the picker thought
// would only need 6 cells. The renderer now caches the sanitized
// version of each visible entry and uses it for BOTH the row-count
// math and the body write — so they agree byte-for-byte.
func TestRender_DynamicLimitMatchesSanitizedRender(t *testing.T) {
	t.Parallel()
	// `cmd \x1b[2J` sanitizes to `cmd ^[[2J` (10 cells incl. pointer
	// = 12 cells). Plus a one-line `git status` entry (12 cells).
	// In a 12-col terminal: each entry is exactly 1 wrap row.
	// In an 8-col terminal: each entry is 2 wrap rows.
	choices := []string{"cmd \x1b[2J", "git status"}
	cases := []struct {
		name      string
		cols      int
		height    int
		wantLimit int
	}{
		// 12-col, height=8 → heightLimit=5. Each entry 2 rows
		// (12/12=1 row + carry... actually 12 cells / 12 cols = 1
		// row). 2 entries × 1 = 2 rows. Both fit.
		{"comfortable", 80, 8, 2},
		// 8-col, height=8 → heightLimit=5. First entry sanitized
		// is "cmd ^[[2J" = 9 cells, +pointer 2 = 11 cells. ceil(11/8)
		// = 2 rows. Second entry "git status" = 10 cells +pointer
		// = 12 cells. ceil(12/8) = 2 rows. Total 4 ≤ 5. Both fit.
		{"narrow-both-fit", 8, 8, 2},
		// 8-col, height=5 → heightLimit=2. First entry needs 2 rows
		// (sanitized). Second entry won't fit. Limit=1.
		// Pre-fix: raw "cmd \x1b[2J" treated as ~6 cells (+pointer 2
		// = 8 cells, ceil(8/8) = 1 row). Math thought first entry was
		// 1 row, second was 2 rows. Total under-estimate.
		{"narrow-only-first-fits", 8, 5, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewModel("", choices, tc.height, tc.cols, 1, 1, DefaultMaxLimit)
			frame := m.Render(RenderOptions{})
			require.Equalf(t, tc.wantLimit, frame.Limit,
				"got limit=%d, want %d (cols=%d height=%d)",
				frame.Limit, tc.wantLimit, tc.cols, tc.height)
			// Sanity: the sanitized form must be in the body, not
			// the raw ESC.
			require.NotContainsf(t, frame.Body, "\x1b[2J",
				"raw ESC sequence must not reach frame.Body: %q", frame.Body)
		})
	}
}
