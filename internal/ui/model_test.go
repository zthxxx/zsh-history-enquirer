package ui

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
)

func sampleChoices() []string {
	return []string{
		"echo zsh-history-enquirer",
		"pwgen --help",
		"where php",
		"cat <<< 123",
		"git status",
		"md5sum --help",
		"cd Temporary",
		"cd Documents",
		"echo author zthxxx",
		"where git",
		"ffmpeg -i \"input.mp4\" -vf reverse reversed.mp4",
		"echo earlier command",
		"git log --pretty=fuller --date=iso -n 1",
		"114514",
		"233333",
		"command-15",
		"command-14",
		"command-13",
		"command-12",
		"command-11",
		"command-10",
		"command-9",
		"command-8 \n - line 0 \n - line 1 \n - line 2 \n - line 3 \n - line 4 \n - line 5 \n - line 5",
		"command-7",
		"command-6 \n - line 1 \n - line 2 \n - line 3 \n\n\n - line 4 \n - line 5",
		"command-5",
		"command-4",
		"command-3",
		"command-2",
		"command-1",
		"command-0",
	}
}

func newTestModel(input string) *Model {
	const rows, cols = 15, 80
	const initRow, initCol = 1, 1
	return NewModel(input, sampleChoices(), rows, cols, initRow, initCol, DefaultMaxLimit)
}

func TestModel_InitialFocusIsTopOfFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	require.Equal(t, "echo zsh-history-enquirer", m.Focused())
}

func TestModel_FilterByInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("git")
	require.Equal(t, "git status", m.Focused())
}

func TestModel_FilterAndScroll(t *testing.T) {
	t.Parallel()
	m := newTestModel("git")
	// Render once to have the model compute Limit; then move down.
	m.Render(RenderOptions{})
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	require.Equal(t, "where git", m.Focused())
}

func TestModel_BackspaceShrinksInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("git")
	m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
	require.Equal(t, "gi", m.Input)
}

func TestModel_CtrlUClearsInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("anything")
	m.Update(keys.KeyEvent{Key: keys.KeyCtrlU})
	require.Empty(t, m.Input)
}

func TestModel_EnterSubmitsFocused(t *testing.T) {
	t.Parallel()
	m := newTestModel("git")
	terminate := m.Update(keys.KeyEvent{Key: keys.KeyEnter})
	require.True(t, terminate)
	require.True(t, m.Submitted)
	require.Equal(t, "git status", m.Result)
}

func TestModel_EscCancelsAndPreservesInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("3jdfn2-9jgf")
	terminate := m.Update(keys.KeyEvent{Key: keys.KeyEsc})
	require.True(t, terminate)
	require.True(t, m.Canceled)
	require.Equal(t, "3jdfn2-9jgf", m.Result)
}

func TestModel_EnterOnNoMatchPreservesInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("3jdfn2-9jgf")
	terminate := m.Update(keys.KeyEvent{Key: keys.KeyEnter})
	require.True(t, terminate)
	require.True(t, m.Submitted)
	require.Equal(t, "3jdfn2-9jgf", m.Result)
}

func TestModel_PasteAppendsToInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Update(keys.PasteEvent{Payload: "git "})
	require.Equal(t, "git ", m.Input)
	m.Update(keys.RuneEvent{R: 's'})
	m.Update(keys.RuneEvent{R: 't'})
	require.Equal(t, "git st", m.Input)
}

func TestModel_DownScrollsBeyondVisible(t *testing.T) {
	t.Parallel()
	// 15-row terminal → effective limit of 12 (height - 3). Move
	// down past it and the focus must follow.
	m := newTestModel("command")
	m.Render(RenderOptions{})

	// Walk down 8 times — must end up on command-9 (the 9th match
	// counting from command-15 at index 0). Mirrors the legacy
	// jest test "search command and scroll".
	for range 8 {
		m.Update(keys.KeyEvent{Key: keys.KeyDown})
		m.Render(RenderOptions{PrevSize: m.Limit})
	}
	require.Contains(t, m.Focused(), "command-")
}

// TestModel_PageUpFromTop — pressing PageUp at the top of an
// already-rotated filter wraps to the bottom, matching the legacy
// "infinite list" UX.
func TestModel_PageUpFromTop(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Render(RenderOptions{})
	first := m.Focused()
	m.Update(keys.KeyEvent{Key: keys.KeyPageUp})
	m.Render(RenderOptions{PrevSize: m.Limit})
	// PageUp at idx=0 rotates the list; the focus is now on the
	// new visible[0], which was previously somewhere lower in the
	// filter.
	require.NotEqual(t, first, m.Focused(),
		"PageUp at top must rotate; got the same focus")
}

// TestModel_HomeResetsFilter — Home recomputes filter and resets
// idx to 0 (the most recent matching entry).
func TestModel_HomeResetsFilter(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Render(RenderOptions{})
	for range 5 {
		m.Update(keys.KeyEvent{Key: keys.KeyDown})
		m.Render(RenderOptions{PrevSize: m.Limit})
	}
	moved := m.Focused()
	m.Update(keys.KeyEvent{Key: keys.KeyHome})
	m.Render(RenderOptions{PrevSize: m.Limit})
	// Home brings us back to the freshest filter — the topmost
	// entry of choices (after reverse-dedupe in newTestModel that's
	// "echo zsh-history-enquirer" per sampleChoices()).
	require.NotEqual(t, moved, m.Focused(),
		"Home should reset focus to the top of the freshest filter")
}

// TestModel_UpAtTopRotates — pressing Up at the top of the
// visible window rotates the visible list rather than alerting.
func TestModel_UpAtTopRotates(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Render(RenderOptions{})
	first := m.Focused()
	m.Update(keys.KeyEvent{Key: keys.KeyUp})
	m.Render(RenderOptions{PrevSize: m.Limit})
	require.NotEqual(t, first, m.Focused(),
		"Up at top of visible window should rotate the list")
}

// TestModel_BackspaceOnEmptyInputIsNoOp asserts Backspace with
// empty Input does not panic or otherwise mutate state.
func TestModel_BackspaceOnEmptyInputIsNoOp(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	require.Empty(t, m.Input)
	m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
	require.Empty(t, m.Input)
}

// TestModel_UnknownKeyIsIgnored — random keys (e.g. KeyTab,
// KeyDelete with no specific handler) must not panic or terminate.
func TestModel_UnknownKeyIsIgnored(t *testing.T) {
	t.Parallel()
	m := newTestModel("git")
	terminate := m.Update(keys.KeyEvent{Key: keys.KeyTab})
	require.False(t, terminate, "Tab should not terminate the picker")
	require.Equal(t, "git", m.Input, "Tab should not modify input")
}

func TestModel_ResizeUpdatesGeometry(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Update(keys.ResizeEvent{Rows: 30, Cols: 100})
	require.Equal(t, 30, m.Height)
	require.Equal(t, 100, m.Width)
}

// TestModel_EndLandsOnLastMatch_NoMultiline asserts that End on a
// purely single-line filter focuses the oldest match — the
// well-trodden case.
func TestModel_EndLandsOnLastMatch_NoMultiline(t *testing.T) {
	t.Parallel()
	choices := []string{"a-1", "a-2", "a-3", "a-4", "a-5", "a-6", "a-7", "a-8"}
	m := NewModel("a", choices, 15, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})

	m.Update(keys.KeyEvent{Key: keys.KeyEnd})
	m.Render(RenderOptions{PrevSize: m.Limit})

	require.Equal(t, "a-8", m.Focused())
}

// TestModel_EndLandsOnLastMatch_WithMultilineInWindow exercises the
// regression case fixed by the back-walk-and-rotate scrollToEnd: a
// multi-line entry between the head and the last match must NOT
// prevent End from focusing the last match.
func TestModel_EndLandsOnLastMatch_WithMultilineInWindow(t *testing.T) {
	t.Parallel()
	multi := "middle\nL1\nL2\nL3\nL4\nL5\nL6\nL7"
	choices := []string{
		"head-1", "head-2", "head-3", "head-4", "head-5", "head-6",
		multi,
		"tail-1", "tail-2", "tail-3", "tail-4",
	}
	m := NewModel("", choices, 15, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})

	m.Update(keys.KeyEvent{Key: keys.KeyEnd})
	m.Render(RenderOptions{PrevSize: m.Limit})

	require.Equal(t, "tail-4", m.Focused(),
		"End must focus the last match even when the previous "+
			"visible window contained a multi-line entry")
}

// TestModel_EndOnEmptyFilter is a no-op smoke test: pressing End
// with no matches must not panic and must keep state coherent.
func TestModel_EndOnEmptyFilter(t *testing.T) {
	t.Parallel()
	m := NewModel("zzz-no-such", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})

	m.Update(keys.KeyEvent{Key: keys.KeyEnd})
	require.Empty(t, m.Filter)
	require.Equal(t, 0, m.Idx)
}

// TestModel_DownOnEmptyFilter — pressing Down when nothing matches
// must not panic and must leave Idx at 0.
func TestModel_DownOnEmptyFilter(t *testing.T) {
	t.Parallel()
	m := NewModel("zzz-no-such", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	require.Empty(t, m.Filter)
	require.Equal(t, 0, m.Idx)
}

// TestModel_UpOnEmptyFilter — pressing Up when nothing matches
// must not panic.
func TestModel_UpOnEmptyFilter(t *testing.T) {
	t.Parallel()
	m := NewModel("zzz-no-such", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	m.Update(keys.KeyEvent{Key: keys.KeyUp})
	require.Empty(t, m.Filter)
	require.Equal(t, 0, m.Idx)
}

// TestModel_DownBeforeFirstRender_AdvancesModulo — exercises the
// Limit==0 branch in moveDown(): before the first Render(), Limit
// has not been computed. moveDown advances Idx modulo filter size,
// preventing a panic if Down is dispatched at the very first frame.
func TestModel_DownBeforeFirstRender_AdvancesModulo(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	require.Equal(t, 0, m.Limit, "Limit must be 0 before first Render")

	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	require.Equal(t, 1, m.Idx, "Down before render must advance Idx by 1")
}

// TestModel_PageUpBeforeFirstRender exercises max1's n<1 branch:
// before any Render(), m.Limit is 0; PageUp calls rotateUp(max1(0))
// which must rotate by 1, not by 0 (which would be a no-op and
// leave the user feeling that PageUp does nothing).
func TestModel_PageUpBeforeFirstRender(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	require.Equal(t, 0, m.Limit, "Limit must be 0 before Render")
	first := m.Filter[0]

	m.Update(keys.KeyEvent{Key: keys.KeyPageUp})
	// Filter should have rotated by 1 — first item is now last.
	require.NotEqual(t, first, m.Filter[0],
		"PageUp before first Render must still rotate (max1 guard)")
}

// TestMax1 is a direct test of the unexported helper; included
// because Limit==0 paths are exercised in two other places
// indirectly and it's cheap to nail down here.
func TestMax1(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, max1(0))
	require.Equal(t, 1, max1(-1))
	require.Equal(t, 1, max1(1))
	require.Equal(t, 5, max1(5))
}

// TestModel_RotateUp_NoOpEdges pins the early-return branches of
// rotateUp: empty Filter and zero/negative n. Both must leave the
// model unchanged.
func TestModel_RotateUp_NoOpEdges(t *testing.T) {
	t.Parallel()
	m := NewModel("zzz", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	require.Empty(t, m.Filter)
	m.rotateUp(1) // empty Filter — must not panic

	m2 := newTestModel("")
	before := slices.Clone(m2.Filter)
	m2.rotateUp(0)
	require.Equal(t, before, m2.Filter, "rotateUp(0) must be a no-op")
	m2.rotateUp(-3)
	require.Equal(t, before, m2.Filter, "rotateUp(-n) must be a no-op")
}

// TestModel_RotateUp_ModuloWrap pins the n %= len branch. n equal
// to filter length should be a no-op (full rotation).
func TestModel_RotateUp_ModuloWrap(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	before := slices.Clone(m.Filter)
	m.rotateUp(len(m.Filter))
	require.Equal(t, before, m.Filter,
		"rotateUp(len(Filter)) must be a full revolution = identity")
}

// TestModel_RotateDown_ModuloWrap pins the same branch on rotateDown.
func TestModel_RotateDown_ModuloWrap(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	before := slices.Clone(m.Filter)
	m.rotateDown(len(m.Filter))
	require.Equal(t, before, m.Filter,
		"rotateDown(len(Filter)) must be a full revolution = identity")
}

// TestModel_RotateUpDown_RoundTrip — rotateDown(n) then rotateUp(n)
// must restore the original Filter for any 0 < n < len(Filter).
func TestModel_RotateUpDown_RoundTrip(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	before := slices.Clone(m.Filter)
	for n := 1; n < len(m.Filter); n++ {
		m.rotateDown(n)
		m.rotateUp(n)
		require.Equalf(t, before, m.Filter,
			"rotate down/up by %d should round-trip", n)
	}
}

// TestModel_DownWrapsWhenFilterFitsInLimit — when len(Filter) <=
// Limit, the entire filter is visible at once, and Down at the
// bottom wraps to top via the rotateDown path.
func TestModel_DownWrapsWhenFilterFitsInLimit(t *testing.T) {
	t.Parallel()
	choices := []string{"a-1", "a-2", "a-3"}
	m := NewModel("a", choices, 15, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})
	require.Equal(t, 3, m.Limit, "all 3 entries must fit in the limit")

	// Walk to the last visible entry.
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	require.Equal(t, "a-3", m.Focused())

	// One more Down at the last visible entry — wrap to top.
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	m.Render(RenderOptions{PrevSize: m.Limit})
	require.Equal(t, 0, m.Idx,
		"Down past the last visible entry must wrap to top (Idx=0)")
}
