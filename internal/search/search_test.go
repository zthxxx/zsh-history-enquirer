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

// TestAndFilter_SpecMandatoryCases pins the exact match table from
// spec/30-search-and-filter.md so a refactor that changes filter
// semantics is caught against the documented contract — not just the
// implementation's preferences.
func TestAndFilter_SpecMandatoryCases(t *testing.T) {
	t.Parallel()

	// One choices list shared across cases — represents the kind of
	// real-world history a user has.
	choices := []string{
		"git status",
		"cd git-repo",
		"Git Push",
		"where php",
		"git stash",
		"git log",
		"git log --pretty=fuller --date=iso -n 1",
	}

	cases := []struct {
		name         string
		input        string
		mustMatch    []string
		mustNotMatch []string
	}{
		{
			name:         "single token (case-insensitive)",
			input:        "git",
			mustMatch:    []string{"git status", "cd git-repo", "Git Push", "git stash", "git log", "git log --pretty=fuller --date=iso -n 1"},
			mustNotMatch: []string{"where php"},
		},
		{
			name:         "two tokens (AND)",
			input:        "git st",
			mustMatch:    []string{"git status", "git stash"},
			mustNotMatch: []string{"git log", "git log --pretty=fuller --date=iso -n 1"},
		},
		{
			name:         "two tokens, the long entry only",
			input:        "log iso",
			mustMatch:    []string{"git log --pretty=fuller --date=iso -n 1"},
			mustNotMatch: []string{"git log"},
		},
		{
			name:         "case-insensitive uppercase",
			input:        "LOG ISO",
			mustMatch:    []string{"git log --pretty=fuller --date=iso -n 1"},
			mustNotMatch: []string{"git log"},
		},
		{
			name:         "empty input is identity",
			input:        "",
			mustMatch:    choices, // every entry passes
			mustNotMatch: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := AndFilter(choices, Tokenize(tc.input))
			for _, want := range tc.mustMatch {
				require.Containsf(t, got, want,
					"input %q must match %q", tc.input, want)
			}
			for _, notWant := range tc.mustNotMatch {
				require.NotContainsf(t, got, notWant,
					"input %q must NOT match %q", tc.input, notWant)
			}
		})
	}
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
