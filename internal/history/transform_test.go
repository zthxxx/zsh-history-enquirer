package history

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func TestReverseDedupeUnescape_Empty(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{}, ReverseDedupeUnescape(nil))
	require.Equal(t, []string{}, ReverseDedupeUnescape([]string{}))
}

func TestReverseDedupeUnescape_Reverse(t *testing.T) {
	t.Parallel()
	in := []string{"a", "b", "c"}
	require.Equal(t, []string{"c", "b", "a"}, ReverseDedupeUnescape(in))
}

func TestReverseDedupeUnescape_Dedupe(t *testing.T) {
	t.Parallel()
	// "a" appears at index 0 and 3; keep the last one (index 3).
	in := []string{"a", "b", "c", "a", "d"}
	require.Equal(t, []string{"d", "a", "c", "b"}, ReverseDedupeUnescape(in))
}

func TestReverseDedupeUnescape_Unescape(t *testing.T) {
	t.Parallel()
	in := []string{`echo \n hello \n world`}
	require.Equal(t, []string{"echo \n hello \n world"}, ReverseDedupeUnescape(in))
}

func TestReverseDedupeUnescape_NoLiteralBackslashN(t *testing.T) {
	t.Parallel()
	out := ReverseDedupeUnescape([]string{`a\nb`, `c`, `d\ne`})
	for _, s := range out {
		require.NotContains(t, s, `\n`)
	}
}

// TestProperty_ReverseDedupeInvariants exercises the pipeline with
// arbitrary inputs and checks the documented invariants.
func TestProperty_ReverseDedupeInvariants(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		in := rapid.SliceOf(rapid.StringMatching(`[a-z\\n ]{0,12}`)).Draw(rt, "lines")
		out := ReverseDedupeUnescape(in)

		// 1. Output cannot grow the input.
		require.LessOrEqual(rt, len(out), len(in))

		// 2. Every output line must, after re-escaping, be present in
		//    the input. (Re-escape because Unescape has been applied to
		//    the output strings.)
		for _, e := range out {
			reescaped := strings.ReplaceAll(e, "\n", `\n`)
			require.True(rt,
				slices.Contains(in, reescaped),
				"output %q missing from input %v", e, in,
			)
		}

		// 3. No duplicates in the output.
		seen := make(map[string]bool, len(out))
		for _, e := range out {
			require.False(rt, seen[e], "duplicate %q", e)
			seen[e] = true
		}

		// 4. Order: every line in the output must appear no later in
		//    the *reversed* input than the next output line.
		reversed := slices.Clone(in)
		slices.Reverse(reversed)
		// Re-escape to compare to original-form input.
		var lastIdx = -1
		for _, e := range out {
			reescaped := strings.ReplaceAll(e, "\n", `\n`)
			idx := slices.Index(reversed, reescaped)
			require.Greater(rt, idx, lastIdx, "out-of-order")
			lastIdx = idx
		}
	})
}
