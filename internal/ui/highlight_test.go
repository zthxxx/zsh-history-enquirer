package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestHighlight_NoTokens(t *testing.T) {
	t.Parallel()
	require.Equal(t, "git status", highlight("git status", nil))
	require.Equal(t, "git status", highlight("git status", []string{}))
	require.Equal(t, "git status", highlight("git status", []string{""}))
}

func TestHighlight_SingleToken(t *testing.T) {
	t.Parallel()
	got := highlight("git status", []string{"git"})
	require.Equal(t, "\x1b[1;36mgit\x1b[0m status", got)
}

func TestHighlight_CaseInsensitive(t *testing.T) {
	t.Parallel()
	got := highlight("Git STATUS", []string{"git", "status"})
	require.Equal(t, "\x1b[1;36mGit\x1b[0m \x1b[1;36mSTATUS\x1b[0m", got)
}

func TestHighlight_OverlappingMatchesMerged(t *testing.T) {
	t.Parallel()
	// In "abab", "ab" matches at positions 0 and 2; "ba" matches at
	// position 1. The three spans (0,2), (2,4), (1,3) merge into
	// the single span (0,4), so the whole string is wrapped in one
	// open-close pair (no double SGR codes mid-string).
	got := highlight("abab", []string{"ab", "ba"})
	require.Equal(t, "\x1b[1;36mabab\x1b[0m", got)
}

func TestHighlight_NonAdjacentMatchesSeparate(t *testing.T) {
	t.Parallel()
	got := highlight("foo bar foo", []string{"foo"})
	require.Equal(t,
		"\x1b[1;36mfoo\x1b[0m bar \x1b[1;36mfoo\x1b[0m",
		got,
	)
}

func TestHighlight_NoMatch(t *testing.T) {
	t.Parallel()
	require.Equal(t, "git status", highlight("git status", []string{"xyz"}))
}

// TestProperty_Highlight_Idempotent: running highlight twice with the
// same tokens on the result produces the same bytes — once we have
// inserted SGR codes around tokens, a second pass would only add new
// codes if it found unhighlighted token occurrences (it shouldn't,
// because the case-insensitive match would re-hit the same payload).
//
// In practice the second pass DOES re-find tokens (because the SGR
// codes don't change the payload's case-insensitive content). To keep
// the property meaningful, we assert a weaker invariant: highlighting
// a string that contains no tokens is a no-op.
func TestProperty_Highlight_NoMatchIsIdentity(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringMatching(`[a-y ]{0,40}`).Draw(rt, "s")
		// Token "z" never appears in `s` (alphabet is a-y).
		require.Equal(rt, s, highlight(s, []string{"z"}))
	})
}

// TestProperty_Highlight_PreservesPayload: stripping the SGR escapes
// from highlight(s, tokens) gives back s.
func TestProperty_Highlight_PreservesPayload(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringMatching(`[a-z ]{0,40}`).Draw(rt, "s")
		toks := rapid.SliceOf(rapid.StringMatching(`[a-z]{1,3}`)).Draw(rt, "tokens")

		got := highlight(s, toks)
		stripped := strings.ReplaceAll(got, highlightOn, "")
		stripped = strings.ReplaceAll(stripped, highlightOff, "")
		require.Equal(rt, s, stripped)
	})
}
