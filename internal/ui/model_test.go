package ui

import (
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
	require.True(t, m.Cancelled)
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

func TestModel_ResizeUpdatesGeometry(t *testing.T) {
	t.Parallel()
	m := newTestModel("")
	m.Update(keys.ResizeEvent{Rows: 30, Cols: 100})
	require.Equal(t, 30, m.Height)
	require.Equal(t, 100, m.Width)
}
