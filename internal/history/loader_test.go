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

// TestFixtureLoader_CRLFStripsCarriageReturn pins the regression
// where a $HISTFILE with CRLF line endings (imported from Windows,
// edited by a misconfigured editor, etc.) left a trailing '\r' on
// each entry. When the picker rendered such an entry, the '\r'
// carriage-returned the cursor back to col 1, scrambling the frame
// — the next entry's pointer would overwrite the previous entry's
// last byte.
//
// We strip trailing '\r' inside splitNonEmptyLines so every loader
// path (zshLoader, FixtureLoader) gets the fix uniformly.
func TestFixtureLoader_CRLFStripsCarriageReturn(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	content := "git status\r\necho hello\r\nls -la\r\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	// Reverse-deduped: most-recent-first.
	require.Equal(t, []string{"ls -la", "echo hello", "git status"}, out)
	for _, line := range out {
		require.NotContainsf(t, line, "\r",
			"CRLF must be stripped — line %q still has CR", line)
	}
}

// TestFixtureLoader_LFOnlyUnchanged is the symmetric guard: a
// regular LF-only file must not have any bytes stripped that
// belong to the entry. (We strip trailing '\r' only; an embedded
// '\r' inside an entry stays put since zsh accepts that as a
// literal.)
func TestFixtureLoader_LFOnlyUnchanged(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	// Embedded \r in the middle of a line — must NOT be stripped.
	content := "first\nmiddle \r still\nlast\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"last", "middle \r still", "first"}, out)
}

// TestFixtureLoader_EmbeddedBlankLineDropped pins that empty lines
// inside the file (from a corrupt write or a `echo "" >> $HISTFILE`)
// do NOT become empty entries in the picker. Empty entries would
// render as blank rows; pressing Enter on one would set $BUFFER
// to "" and silently swallow the user's typed prefix.
func TestFixtureLoader_EmbeddedBlankLineDropped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	// "a\n\nb\n" — embedded blank line between "a" and "b".
	content := "a\n\nb\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"b", "a"}, out,
		"embedded blank line must not appear in output")
	for _, line := range out {
		require.NotEmptyf(t, line,
			"output must not contain empty entries; got %q in %v", line, out)
	}
}

// TestFixtureLoader_ExtendedHistoryEmptyCmdDropped — a corrupt
// extended-history line `: 1700000001:0;` records an empty command.
// `splitNonEmptyLines` lets it through (the line itself is
// non-empty), but `stripExtendedHistoryPrefix` reduces it to "".
// Without a post-strip empty-drop, that "" survives the pipeline
// and renders as a blank picker row — pressing Enter on which sets
// $BUFFER to "" and silently swallows the user's typed prefix.
// The fix is symmetric to the embedded-blank-line drop in
// splitNonEmptyLines, applied after the prefix strip.
func TestFixtureLoader_ExtendedHistoryEmptyCmdDropped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	content := ": 1700000000:0;ls\n: 1700000001:0;\n: 1700000002:0;git\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"git", "ls"}, out,
		"empty extended-history command must not survive as blank entry")
	for _, line := range out {
		require.NotEmptyf(t, line,
			"output must not contain empty entries; got %q in %v", line, out)
	}
}

// TestFixtureLoader_CRLFOnlyLineDropped — a `\r\n` (an empty CRLF
// line) is `\r` after the LF strip; the trailing-CR pass turns it
// into "", and the empty-line drop removes it entirely. So the
// picker never sees a `\r`-only entry.
func TestFixtureLoader_CRLFOnlyLineDropped(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "history.txt")
	content := "first\r\n\r\nsecond\r\n"
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	out, err := FixtureLoader(path).Load(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"second", "first"}, out)
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

// TestNewZshLoader_DefaultsNegativeHistSize covers the
// negative-value defaulting branch. flag.Int accepts negative
// numbers, so a `--histsize=-1` argv is syntactically valid; the
// loader must promote that to DefaultHistSize so zsh doesn't
// surface a no-history view (or a hard error, depending on
// version) for what is almost certainly a typo.
func TestNewZshLoader_DefaultsNegativeHistSize(t *testing.T) {
	t.Parallel()

	loader := NewZshLoader(Options{HistSize: -42})
	zl, ok := loader.(*zshLoader)
	require.True(t, ok, "NewZshLoader must return a *zshLoader")
	require.Equal(t, DefaultHistSize, zl.opts.HistSize,
		"negative HistSize must default to %d, got %d",
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
