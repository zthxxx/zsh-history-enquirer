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
