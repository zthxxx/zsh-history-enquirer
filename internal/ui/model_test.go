package ui

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

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

// TestModel_BackspaceDeletesRuneNotByte pins the regression: a
// byte-level Backspace would leave a trailing UTF-8 continuation
// byte, corrupting the input into invalid UTF-8 and rendering as
// `\xe4\xbd` mojibake. CJK / emoji / accented Latin chars are all
// ≥2 bytes in UTF-8 so this matters in practice.
func TestModel_BackspaceDeletesRuneNotByte(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// `in`'s last rune must be removed; remaining bytes must be
		// valid UTF-8 (and equal to `want`).
		in   string
		want string
	}{
		{"chinese", "你", ""},
		{"prefixed-chinese", "git 你", "git "},
		{"emoji", "🚀", ""},
		{"prefixed-emoji", "ship 🚀", "ship "},
		{"accented-latin", "café", "caf"},
		{"ascii-still-works", "git", "gi"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newTestModel(tc.in)
			m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
			require.Equal(t, tc.want, m.Input)
			require.Truef(t, utf8.ValidString(m.Input),
				"Backspace produced invalid UTF-8: % x", m.Input)
		})
	}
}

func TestModel_CtrlUClearsInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("anything")
	m.Update(keys.KeyEvent{Key: keys.KeyCtrlU})
	require.Empty(t, m.Input)
}

// TestModel_PasteSanitizesNewlinesToSpaces pins the regression
// where a multi-line paste payload would scribble across the
// terminal. Pasted control bytes are useless as filter input, so
// we map \n / \r / \t to space (preserving the token boundary)
// before they ever land in m.Input.
func TestModel_PasteSanitizesNewlinesToSpaces(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"newline-to-space", "git\nlog", "git log"},
		{"crlf-each-to-space", "git\r\nlog", "git  log"},
		{"tab-to-space", "git\tlog", "git log"},
		{"plain-passthrough", "git log", "git log"},
		{"multiline-block", "line1\nline2\nline3", "line1 line2 line3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newTestModel("")
			m.Update(keys.PasteEvent{Payload: tc.in})
			require.Equal(t, tc.want, m.Input)
		})
	}
}

// TestModel_TypingNewlineGetsSanitized — same protection on the
// per-rune typing path. Hard to type \n directly (Enter is bound
// to Submit), but a custom keymap or a misconfigured terminal
// could deliver one.
func TestModel_TypingNewlineGetsSanitized(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Update(keys.RuneEvent{R: '\n'})
	require.Equal(t, " ", m.Input)
	m.Update(keys.RuneEvent{R: '\t'})
	require.Equal(t, "  ", m.Input)
	m.Update(keys.RuneEvent{R: 'g'})
	require.Equal(t, "  g", m.Input)
}

// TestModel_CtrlPCtrlNAreUpDownAliases pins the Ctrl-P / Ctrl-N
// alias behaviour: zsh's emacs keymap binds these to
// up-line-or-history / down-line-or-history. Power users press
// these reflexively. Without an alias the picker would silently
// ignore them.
func TestModel_CtrlPCtrlNAreUpDownAliases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		alias     keys.Key
		canonical keys.Key
	}{
		{"ctrl-p == up", keys.KeyCtrlP, keys.KeyUp},
		{"ctrl-n == down", keys.KeyCtrlN, keys.KeyDown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a := newTestModel("")
			b := newTestModel("")
			a.Update(keys.KeyEvent{Key: tc.alias})
			b.Update(keys.KeyEvent{Key: tc.canonical})
			require.Equal(t, b.Idx, a.Idx,
				"%v should leave Idx where %v does", tc.alias, tc.canonical)
		})
	}
}

// TestModel_CtrlWDeletesLastWord pins shell-muscle-memory behaviour:
// Ctrl-W strips trailing whitespace and the preceding word in one
// keystroke. Tested across ASCII, multi-rune CJK / emoji, multiple
// trailing spaces, and the empty-input no-op.
func TestModel_CtrlWDeletesLastWord(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"two-words", "git status", "git "},
		{"three-words-keeps-prefix", "git log -p", "git log "},
		{"trailing-space-stripped", "git log ", "git "},
		{"single-word", "git", ""},
		{"empty-noop", "", ""},
		{"chinese-word", "命令 你好", "命令 "},
		{"emoji-word", "ship 🚀", "ship "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := newTestModel(tc.in)
			m.Update(keys.KeyEvent{Key: keys.KeyCtrlW})
			require.Equal(t, tc.want, m.Input)
		})
	}
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

// TestModel_EndOnEntryLargerThanTerminal exercises the
// visibleCount==0 fallback in scrollToEnd: when the LAST filtered
// entry's wrapped row count alone exceeds heightLimit, the
// back-walk loop never increments visibleCount. The fallback
// bumps it to 1 so End still lands on a focused entry rather
// than nothing.
func TestModel_EndOnEntryLargerThanTerminal(t *testing.T) {
	t.Parallel()
	huge := "huge-tail\n" + strings.Repeat("L\n", 30)
	choices := []string{"a-1", "a-2", huge}
	// Tiny terminal — heightLimit = 5-3 = 2, but `huge` alone wraps
	// to ~32 rows.
	m := NewModel("", choices, 5, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})

	m.Update(keys.KeyEvent{Key: keys.KeyEnd})
	// Idx must be 0 (visibleCount-1 with visibleCount=1 from the
	// fallback). The Filter has been rotated by 1 — the huge entry
	// (last in original Filter) is now at position 0.
	require.Equal(t, 0, m.Idx,
		"End on overflow-only entry must still focus index 0")
}

// TestModel_NewModel_ZeroMaxLimitDefaults pins the maxLimit<=0
// guard in NewModel: passing 0 (or negative) must default to
// DefaultMaxLimit. The fx graph routes cfg.MaxLimit through this,
// and cfg.MaxLimit defaults to 0.
func TestModel_NewModel_ZeroMaxLimitDefaults(t *testing.T) {
	t.Parallel()
	m := NewModel("", []string{"a"}, 24, 80, 1, 1, 0)
	require.Equal(t, DefaultMaxLimit, m.MaxLimit)
	m2 := NewModel("", []string{"a"}, 24, 80, 1, 1, -5)
	require.Equal(t, DefaultMaxLimit, m2.MaxLimit,
		"negative maxLimit must also default")
}

// TestModel_UpDecrementsFromMidWindow exercises the moveUp branch
// where Idx > 0 — the simple decrement case. Previous tests
// covered the at-top rotate path and the empty-filter early-return
// path; the most-common case (mid-window) was not directly pinned.
func TestModel_UpDecrementsFromMidWindow(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Render(RenderOptions{})
	// Move down twice.
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	m.Update(keys.KeyEvent{Key: keys.KeyDown})
	require.Equal(t, 2, m.Idx)
	// Now Up should decrement to 1, no rotation.
	m.Update(keys.KeyEvent{Key: keys.KeyUp})
	require.Equal(t, 1, m.Idx)
}

// TestModel_CtrlCCancelsAndPreservesInput — pins the Ctrl-C cancel
// path. Was missing from explicit tests; the only Esc/^C variant
// asserted before was Esc.
func TestModel_CtrlCCancelsAndPreservesInput(t *testing.T) {
	t.Parallel()
	m := newTestModel("xyz-no-such-input")
	terminate := m.Update(keys.KeyEvent{Key: keys.KeyCtrlC})
	require.True(t, terminate)
	require.True(t, m.Canceled)
	require.False(t, m.Submitted, "Ctrl-C must not also set Submitted")
	require.Equal(t, "xyz-no-such-input", m.Result,
		"Ctrl-C must preserve typed input verbatim — widget contract")
}

// TestSubmitResult_Branches exercises all three branches of
// SubmitResult directly:
//   - Canceled → Input
//   - Submitted with focused → focused
//   - Submitted with no match → Input
func TestSubmitResult_Branches(t *testing.T) {
	t.Parallel()

	// Canceled.
	m1 := newTestModel("typed")
	m1.Canceled = true
	require.Equal(t, "typed", m1.SubmitResult())

	// Submitted with focused match.
	m2 := newTestModel("git")
	require.NotEmpty(t, m2.Filter, "git should have matches in fixture")
	m2.Submitted = true
	require.Equal(t, m2.Filter[0], m2.SubmitResult())

	// Submitted with no match.
	m3 := newTestModel("zzz-no-match-zzz")
	require.Empty(t, m3.Filter)
	m3.Submitted = true
	require.Equal(t, "zzz-no-match-zzz", m3.SubmitResult(),
		"Enter on no-match must return typed input verbatim")
}

// TestModel_PageDown rotates the visible window down by one page.
// The model has no direct getter for the rotation amount, so we
// assert by checking that the focused entry CHANGED — which it
// must after a downward rotation.
func TestModel_PageDown(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Render(RenderOptions{})
	first := m.Focused()

	m.Update(keys.KeyEvent{Key: keys.KeyPageDown})
	m.Render(RenderOptions{PrevSize: m.Limit})
	require.NotEqual(t, first, m.Focused(),
		"PageDown must rotate the visible window")
}

// TestModel_PageDownThenPageUpRoundTrip — on a single-line filter
// (constant Limit across renders), PageDown followed by PageUp
// must restore the original focus. Multi-line entries break the
// round-trip because the dynamic limit recomputes between renders;
// that's a known property, not a regression — hence the
// single-line fixture.
func TestModel_PageDownThenPageUpRoundTrip(t *testing.T) {
	t.Parallel()
	choices := make([]string, 30)
	for i := range choices {
		choices[i] = "single-line-entry-" + string(rune('a'+i%26))
	}
	m := NewModel("", choices, 15, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})
	first := m.Focused()

	m.Update(keys.KeyEvent{Key: keys.KeyPageDown})
	m.Render(RenderOptions{PrevSize: m.Limit})
	m.Update(keys.KeyEvent{Key: keys.KeyPageUp})
	m.Render(RenderOptions{PrevSize: m.Limit})

	require.Equal(t, first, m.Focused(),
		"PageDown + PageUp on single-line filter must round-trip")
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

// TestModel_RotateDown_NoOpEdges pins the early-return guards on
// rotateDown — empty Filter and zero/negative n. Both must leave
// the model unchanged. Symmetric to TestModel_RotateUp_NoOpEdges.
func TestModel_RotateDown_NoOpEdges(t *testing.T) {
	t.Parallel()
	m := NewModel("zzz", []string{"git", "echo"}, 15, 80, 1, 1, DefaultMaxLimit)
	require.Empty(t, m.Filter)
	m.rotateDown(1) // empty Filter — must not panic

	m2 := newTestModel("")
	before := slices.Clone(m2.Filter)
	m2.rotateDown(0)
	require.Equal(t, before, m2.Filter, "rotateDown(0) must be a no-op")
	m2.rotateDown(-3)
	require.Equal(t, before, m2.Filter, "rotateDown(-n) must be a no-op")
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

// TestModel_RotateDoesNotMutateChoices pins a stability invariant:
// search.AndFilter aliases m.Choices when the input has no tokens,
// so without an explicit clone in recomputeFilter, rotateUp /
// rotateDown would scribble the rotation through into the immutable
// Choices slice. The user-visible failure is that scrolling the
// empty-input view, then typing-and-clearing, returns a permuted
// history instead of the chronological order.
func TestModel_RotateDoesNotMutateChoices(t *testing.T) {
	t.Parallel()
	choices := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	snapshot := slices.Clone(choices)

	m := NewModel("", choices, 24, 80, 1, 1, DefaultMaxLimit)
	m.rotateUp(2)
	m.rotateDown(3)

	require.Equal(t, snapshot, choices,
		"Choices was mutated by Filter rotations — alias leak")
}

// TestModel_ScrollThenClear_RestoresChronologicalOrder is the
// user-facing pin of the same invariant. Scroll via Up, type a
// character, then Ctrl-U; the resulting Filter must equal the
// original Choices order, not a rotated permutation.
func TestModel_ScrollThenClear_RestoresChronologicalOrder(t *testing.T) {
	t.Parallel()
	choices := []string{"newest", "second", "third", "fourth", "fifth"}
	snapshot := slices.Clone(choices)

	m := NewModel("", choices, 24, 80, 1, 1, DefaultMaxLimit)
	m.Render(RenderOptions{})

	// Rotate the empty-input view a couple of times via Up.
	m.Update(keys.KeyEvent{Key: keys.KeyUp})
	m.Update(keys.KeyEvent{Key: keys.KeyUp})

	// Type then clear. recomputeFilter runs both at the rune-append
	// and at the Ctrl-U step.
	m.Update(keys.RuneEvent{R: 'x'})
	m.Update(keys.KeyEvent{Key: keys.KeyCtrlU})

	require.Equal(t, snapshot, m.Filter,
		"after scroll-and-clear, Filter must equal original Choices order")
	require.Equal(t, snapshot, choices,
		"underlying Choices must remain in chronological order")
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
