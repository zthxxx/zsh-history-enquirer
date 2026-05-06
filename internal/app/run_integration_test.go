package app

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRun_VersionFastPath verifies that --version short-circuits in
// Run() before any TTY interaction. This is the only path through
// Run that doesn't need a real terminal, and it's worth locking
// down because it's the path users hit when debugging an install
// (`zsh-history-enquirer --version` from any context).
//
// The deeper paths through Run — DSR probe, raw mode, event loop,
// render — require a real pty plus access to private fields of
// tty.TTY. Those paths are exhaustively covered by the e2e docker
// scenarios (which exercise the actual binary against a real zsh +
// pty), so we don't try to duplicate that here as Go integration
// tests; doing so would either need to leak tty internals or
// reimplement the harness in Go for marginal extra coverage.
func TestRun_VersionFastPath(t *testing.T) {
	t.Parallel()

	cfg := &Config{PrintVersion: true}
	var stderr bytes.Buffer

	result, err := Run(context.Background(), cfg, nil, nil, &stderr)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, result.Output, "zsh-history-enquirer")
	// Stderr should be untouched on the version path — it's the
	// command-substitution-friendly fast path.
	require.Empty(t, stderr.String())
}

// Compile-time checks that the fx-graph named writer types remain
// io.Writer-compatible. Catches an accidental future refactor that
// would silently break the production graph wiring.
var _ io.Writer = (Stdout)(nil)
var _ io.Writer = (StderrWriter)(nil)
