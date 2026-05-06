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

// TestNewZshLoader_DefaultsHistSize covers the zero-value defaulting
// branch: passing HistSize=0 must yield a loader that uses
// DefaultHistSize (100000), not 0 — otherwise zsh's `fc -ln 1` would
// return nothing.
func TestNewZshLoader_DefaultsHistSize(t *testing.T) {
	t.Parallel()

	loader := NewZshLoader(Options{})
	zl, ok := loader.(*zshLoader)
	require.True(t, ok, "NewZshLoader must return a *zshLoader")
	require.Equal(t, DefaultHistSize, zl.opts.HistSize,
		"zero HistSize must default to %d, got %d",
		DefaultHistSize, zl.opts.HistSize)
}

// TestZshLoader_BadZshBinaryReturnsError covers the error path when
// the configured zsh binary doesn't exist. The error message should
// mention "exec failed" so callers can pattern-match on it.
func TestZshLoader_BadZshBinaryReturnsError(t *testing.T) {
	t.Parallel()

	loader := NewZshLoader(Options{
		ZshBinary: "/nonexistent/path/to/zsh-not-here",
		HistFile:  "/tmp/nonexistent",
	})
	_, err := loader.Load(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "exec failed",
		"loader error must mention exec failure for upstream pattern matching")
}

// TestStripExtendedHistoryPrefix_EdgeCases pins the three branches
// of the helper: prefixed-with-semicolon (normal), prefixed-without-
// semicolon (malformed but harmless), and unprefixed.
func TestStripExtendedHistoryPrefix_EdgeCases(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{": 12345:0;echo ok", "echo ok"},
		{"echo ok", "echo ok"},                     // no prefix
		{": 12345:0 echo ok", ": 12345:0 echo ok"}, // prefix-but-no-semi
		{": ", ": "}, // bare prefix
		{"", ""},
	}
	for _, tc := range cases {
		got := stripExtendedHistoryPrefix(tc.in)
		require.Equalf(t, tc.want, got,
			"stripExtendedHistoryPrefix(%q) = %q, want %q", tc.in, got, tc.want)
	}
}
