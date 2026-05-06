package tty

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseDSRResponse_OK(t *testing.T) {
	t.Parallel()

	row, col, err := parseDSRResponse("\x1b[12;34R")
	require.NoError(t, err)
	require.Equal(t, 12, row)
	require.Equal(t, 34, col)
}

func TestParseDSRResponse_LeadingNoise(t *testing.T) {
	t.Parallel()

	// A poorly-behaved terminal might leak a stray byte before the
	// CSI introducer. The parser must look for `[` and `R` markers
	// rather than insisting on a clean prefix.
	row, col, err := parseDSRResponse("garbage\x1b[1;1Rmore")
	require.NoError(t, err)
	require.Equal(t, 1, row)
	require.Equal(t, 1, col)
}

func TestParseDSRResponse_Malformed(t *testing.T) {
	t.Parallel()

	cases := []string{
		"",
		"R",
		"[12R",            // no semicolon
		"[abc;1R",         // non-numeric
		"[1;abcR",         // non-numeric
		"prefix without R",
	}
	for _, c := range cases {
		_, _, err := parseDSRResponse(c)
		require.Error(t, err, "input %q should fail", c)
	}
}
