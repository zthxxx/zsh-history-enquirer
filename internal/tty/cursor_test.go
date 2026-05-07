package tty

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDSRResponse_OK(t *testing.T) {
	t.Parallel()

	row, col, leftover, err := parseDSRResponse("\x1b[12;34R")
	require.NoError(t, err)
	require.Equal(t, 12, row)
	require.Equal(t, 34, col)
	require.Empty(t, leftover, "clean response → no leftover")
}

func TestParseDSRResponse_LeadingNoise(t *testing.T) {
	t.Parallel()

	// A poorly-behaved terminal — or, more commonly, a fast-typing
	// user pressing Ctrl-R then typing characters that arrive at
	// the TTY before the DSR response — drops bytes ahead of the
	// CSI introducer. The parser must anchor on `\x1b[` and return
	// the prefix + suffix bytes via the leftover channel so the
	// caller can replay them through the regular keystream parser.
	row, col, leftover, err := parseDSRResponse("garbage\x1b[1;1Rmore")
	require.NoError(t, err)
	require.Equal(t, 1, row)
	require.Equal(t, 1, col)
	require.Equal(t, "garbagemore", leftover,
		"non-DSR bytes (pre + post) must round-trip via leftover")
}

// TestParseDSRResponse_PreservesUserTypedPrefix pins the canonical
// fast-typing case: user presses Ctrl-R, then types `git ` before
// the picker has finished its DSR probe. The bytes arrive at the
// TTY ahead of the response. Without leftover preservation the
// `git ` would be silently consumed and never reach the picker.
func TestParseDSRResponse_PreservesUserTypedPrefix(t *testing.T) {
	t.Parallel()

	row, col, leftover, err := parseDSRResponse("git \x1b[7;42R")
	require.NoError(t, err)
	require.Equal(t, 7, row)
	require.Equal(t, 42, col)
	require.Equal(t, "git ", leftover)
}

// TestParseDSRResponse_PreservesPostResponseBytes — symmetric to
// the prefix case: the read may pull in bytes typed AFTER the DSR
// response in the same chunk. They must also survive.
func TestParseDSRResponse_PreservesPostResponseBytes(t *testing.T) {
	t.Parallel()

	_, _, leftover, err := parseDSRResponse("\x1b[7;42Rls -la")
	require.NoError(t, err)
	require.Equal(t, "ls -la", leftover)
}

// TestParseDSRResponse_TypedBracketIsLeftover — a user who typed
// `[` before the response would, under the old `strings.Index(s,"[")`
// anchor, mis-position the parser onto their typed bracket. Anchoring
// on `\x1b[` keeps the parse correct.
func TestParseDSRResponse_TypedBracketIsLeftover(t *testing.T) {
	t.Parallel()

	row, col, leftover, err := parseDSRResponse("[\x1b[12;34R")
	require.NoError(t, err)
	require.Equal(t, 12, row)
	require.Equal(t, 34, col)
	require.Equal(t, "[", leftover)
}

func TestParseDSRResponse_Malformed(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"R",
		"\x1b[12R",    // no semicolon
		"\x1b[abc;1R", // non-numeric
		"\x1b[1;abcR", // non-numeric
		"prefix without R or CSI",
	}
	for _, c := range cases {
		_, _, _, err := parseDSRResponse(c)
		require.Error(t, err, "input %q should fail", c)
	}
}

// TestParseDSRResponse_MalformedReturnsFullInputAsLeftover pins the
// contract that the malformed-parse error paths populate leftover
// with the full input string. handleProbeFallback (in internal/app)
// reads cur.leftover as the default fallback for non-Timeout errors
// so the user's typed bytes round-trip through reader.Prefeed even
// when the probe could not extract a (row, col) — the bug we fixed
// in the fast-typing-arrow scenario. The non-numeric body case is
// the realistic trigger: user presses Ctrl-R then types an arrow
// before the probe completes; the buffer reads
// `\x1b[A\x1b[12;5R`, parseDSR anchors on the FIRST `\x1b[`, and the
// body `A\x1b[12;5` fails the numeric parse.
func TestParseDSRResponse_MalformedReturnsFullInputAsLeftover(t *testing.T) {
	t.Parallel()
	cases := []string{
		"\x1b[12R",            // no semicolon
		"\x1b[abc;1R",         // non-numeric row
		"\x1b[1;abcR",         // non-numeric col
		"\x1b[A\x1b[12;5R",    // user typed Up before DSR; first CSI body fails the parse
		"\x1b[\x1b[12;5R",     // bare CSI prefix wedged in front of the response
	}
	for _, c := range cases {
		row, col, leftover, err := parseDSRResponse(c)
		require.Errorf(t, err, "input %q should fail", c)
		require.Equalf(t, 0, row, "row should be 0 on malformed parse for %q", c)
		require.Equalf(t, 0, col, "col should be 0 on malformed parse for %q", c)
		require.Equalf(t, c, leftover,
			"leftover must round-trip the full input on malformed parse "+
				"so handleProbeFallback can replay it via reader.Prefeed; "+
				"input=%q got leftover=%q", c, leftover)
	}
}
