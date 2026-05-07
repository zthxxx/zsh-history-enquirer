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
// when the probe could not extract a (row, col).
//
// Inputs in this test are shapes that the scan-forward parser still
// cannot resolve to a `<row>;<col>` body — every `\x1b[…R` candidate
// they contain has a non-numeric or missing-semicolon body. Inputs
// where the scan-forward parser can find a valid DSR (e.g.
// `\x1b[A\x1b[12;5R`) are covered by
// TestParseDSRResponse_ScanForwardSkipsNonDSRCSI instead.
func TestParseDSRResponse_MalformedReturnsFullInputAsLeftover(t *testing.T) {
	t.Parallel()
	cases := []string{
		"\x1b[12R",    // no semicolon
		"\x1b[abc;1R", // non-numeric row
		"\x1b[1;abcR", // non-numeric col
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

// FuzzParseDSRResponse confirms parseDSRResponse never panics on
// arbitrary byte input — including pathological shapes a scan-forward
// loop could in principle iterate forever on (very long inputs,
// unbalanced introducers, etc). Standard `go test` runs each seed
// once; the fuzzer's mutate loop runs with `-fuzz=FuzzParseDSRResponse`.
func FuzzParseDSRResponse(f *testing.F) {
	seeds := []string{
		"",
		"\x1b[",
		"\x1b[12;34R",
		"\x1b[A\x1b[12;5R",
		"\x1b[\x1b[\x1b[\x1b[",
		"\x1b[" + string(make([]byte, 4096)),
		"\x1b[12;abcRR\x1b[1;1R",
	}
	for _, s := range seeds {
		f.Add(s)
	}
	f.Fuzz(func(_ *testing.T, s string) {
		_, _, _, _ = parseDSRResponse(s)
	})
}

// TestParseDSRResponse_ScanForwardSkipsNonDSRCSI pins the user-
// emphasized fast-typing scenario: user presses Ctrl-R then taps an
// arrow / Home / End before the picker finishes its DSR probe. The
// terminal-encoded keypress (`\x1b[A`, `\x1b[H`, …) lands in the
// probe buffer ahead of the real response, so the buffer ends up
// e.g. `\x1b[A\x1b[12;5R`. The scan-forward parser must:
//
//  1. Notice the first CSI body (`A`) is not a `<digits>;<digits>`
//     pair and skip it.
//  2. Find the second CSI body (`12;5`) and parse it cleanly.
//  3. Hand the user's pre-CSI bytes back via leftover so
//     reader.Prefeed can turn them into a real key event.
//
// Without this, the picker rendered at the col=1 fallback whenever
// the user raced the probe — the byte-preservation fix
// (commit a244f2f) closed the input-loss bug-class but left the
// rendering side on the fallback path.
func TestParseDSRResponse_ScanForwardSkipsNonDSRCSI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		input        string
		wantRow      int
		wantCol      int
		wantLeftover string
	}{
		{
			"user-up-arrow-then-dsr",
			"\x1b[A\x1b[12;5R",
			12, 5,
			"\x1b[A",
		},
		{
			"user-home-then-dsr",
			"\x1b[H\x1b[7;42R",
			7, 42,
			"\x1b[H",
		},
		{
			"user-typed-then-up-then-dsr",
			"git \x1b[A\x1b[3;9R",
			3, 9,
			"git \x1b[A",
		},
		{
			"non-dsr-csi-with-numeric-prefix-skipped",
			"\x1b[1@\x1b[5;5R",
			5, 5,
			"\x1b[1@",
		},
		{
			"two-arrows-before-dsr",
			"\x1b[A\x1b[B\x1b[7;7R",
			7, 7,
			"\x1b[A\x1b[B",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			row, col, leftover, err := parseDSRResponse(tc.input)
			require.NoErrorf(t, err, "input %q should parse via scan-forward", tc.input)
			require.Equal(t, tc.wantRow, row, "row mismatch for %q", tc.input)
			require.Equal(t, tc.wantCol, col, "col mismatch for %q", tc.input)
			require.Equal(t, tc.wantLeftover, leftover,
				"leftover should preserve every pre-DSR byte for replay; input=%q",
				tc.input)
		})
	}
}
