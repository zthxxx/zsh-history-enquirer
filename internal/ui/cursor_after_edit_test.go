package ui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
)

// TestModel_Cursor_AfterBackspaceCJK pins that Backspace correctly
// updates m.Cursor for CJK glyphs (2 cells per rune). Before the
// CellWidth migration, cursor would have been off by 1 cell after
// each CJK delete.
func TestModel_Cursor_AfterBackspaceCJK(t *testing.T) {
	t.Parallel()
	m := NewModel("你好世", []string{}, 24, 80, 1, 1, DefaultMaxLimit)
	require.Equal(t, 6, m.Cursor, "3 CJK runes = 6 cells")

	m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
	require.Equal(t, "你好", m.Input)
	require.Equal(t, 4, m.Cursor, "after BS: 2 CJK runes = 4 cells")

	m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
	require.Equal(t, "你", m.Input)
	require.Equal(t, 2, m.Cursor, "after BS: 1 CJK rune = 2 cells")

	m.Update(keys.KeyEvent{Key: keys.KeyBackspace})
	require.Equal(t, "", m.Input)
	require.Equal(t, 0, m.Cursor, "after BS: empty = 0 cells")
}

// TestModel_Cursor_AfterCtrlW pins that Ctrl-W correctly
// recomputes m.Cursor in cells.
func TestModel_Cursor_AfterCtrlW(t *testing.T) {
	t.Parallel()
	m := NewModel("git 你好", []string{}, 24, 80, 1, 1, DefaultMaxLimit)
	require.Equal(t, 8, m.Cursor, "'git ' (4) + '你好' (4) = 8 cells")

	m.Update(keys.KeyEvent{Key: keys.KeyCtrlW})
	require.Equal(t, "git ", m.Input)
	require.Equal(t, 4, m.Cursor, "after CtrlW: 'git ' = 4 cells")
}
