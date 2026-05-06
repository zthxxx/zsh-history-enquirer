package search

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestTokenize_Empty(t *testing.T) {
	t.Parallel()
	require.Nil(t, Tokenize(""))
	require.Nil(t, Tokenize("   "))
}

func TestTokenize_BasicSplit(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"git", "log"}, Tokenize("git log"))
}

func TestTokenize_LowercasesAndDedupes(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"git"}, Tokenize("GIT git"))
	require.Equal(t, []string{"log", "iso"}, Tokenize("LOG iso  log  "))
}

func TestAndFilter_Empty(t *testing.T) {
	t.Parallel()
	in := []string{"git status", "echo ok"}
	require.Equal(t, in, AndFilter(in, nil))
	require.Equal(t, in, AndFilter(in, []string{}))
}

func TestAndFilter_AndSemantics(t *testing.T) {
	t.Parallel()
	choices := []string{
		"git log --pretty=fuller --date=iso -n 1",
		"git log",
		"git status",
		"echo iso",
	}
	got := AndFilter(choices, Tokenize("log iso"))
	require.Equal(t, []string{"git log --pretty=fuller --date=iso -n 1"}, got)
}

func TestAndFilter_CaseInsensitive(t *testing.T) {
	t.Parallel()
	choices := []string{"Git Status", "Echo OK"}
	require.Equal(t, []string{"Git Status"}, AndFilter(choices, Tokenize("git")))
	require.Equal(t, []string{"Echo OK"}, AndFilter(choices, Tokenize("OK")))
}

func TestAndFilter_PreservesOrder(t *testing.T) {
	t.Parallel()
	choices := []string{"a-x", "b-x", "c-x"}
	got := AndFilter(choices, Tokenize("x"))
	require.Equal(t, choices, got)
}

func TestProperty_AndFilter_MonotonicInTokens(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		choices := rapid.SliceOf(rapid.StringMatching(`[a-z ]{1,16}`)).Draw(rt, "choices")
		base := rapid.SliceOf(rapid.StringMatching(`[a-z]{1,4}`)).Draw(rt, "tokens")

		// Adding a token can only narrow the result set.
		extra := rapid.StringMatching(`[a-z]{1,4}`).Draw(rt, "extra")
		biggerSet := AndFilter(choices, base)
		smallerSet := AndFilter(choices, append(slices.Clone(base), extra))

		require.LessOrEqual(rt, len(smallerSet), len(biggerSet))
		// Every element of smaller is in bigger.
		for _, e := range smallerSet {
			require.True(rt, slices.Contains(biggerSet, e))
		}
	})
}

func TestProperty_AndFilter_EveryMatchContainsAllTokens(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		choices := rapid.SliceOf(rapid.StringMatching(`[a-zA-Z ]{1,20}`)).Draw(rt, "choices")
		input := rapid.StringMatching(`[a-z ]{0,12}`).Draw(rt, "input")
		tokens := Tokenize(input)

		got := AndFilter(choices, tokens)
		for _, m := range got {
			lc := strings.ToLower(m)
			for _, tok := range tokens {
				require.Contains(rt, lc, tok)
			}
		}
	})
}
