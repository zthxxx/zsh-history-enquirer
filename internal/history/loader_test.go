package history

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixtureLoader_StripsExtendedPrefix(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	content := `: 1568797100:0;command-0
: 1568797100:0;command-1
plain-line
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)

	// Reverse-dedupe order: plain-line, command-1, command-0.
	require.Equal(t, []string{"plain-line", "command-1", "command-0"}, out)
}

func TestFixtureLoader_Empty(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	require.NoError(t, os.WriteFile(path, []byte(""), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestFixtureLoader_MultilineEscape(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	content := `: 1568797123:0;echo a\nb`
	require.NoError(t, os.WriteFile(path, []byte(content+"\n"), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)

	require.Equal(t, []string{"echo a\nb"}, out)
	require.NotContains(t, out[0], `\n`)
}

func TestZshLoader_AgainstTempHistfile(t *testing.T) {
	t.Parallel()

	if _, err := exec.LookPath("zsh"); err != nil {
		t.Skip("zsh not installed; skipping ZshLoader integration test")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, ".zsh_history")
	content := `: 1568797100:0;cmd-a
: 1568797101:0;cmd-b
: 1568797102:0;cmd-a
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	loader := NewZshLoader(Options{HistFile: path, HistSize: 100})
	out, err := loader.Load(context.Background())
	require.NoError(t, err)

	require.Equal(t, []string{"cmd-a", "cmd-b"}, out)
}
