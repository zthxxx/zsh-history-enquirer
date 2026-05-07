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
