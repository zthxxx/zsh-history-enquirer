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
// Run() we exercise as a Go-level integration test; the deeper
// paths (DSR probe, raw mode, event loop, render, submit/cancel)
// are exhaustively covered by the docker-driven e2e scenarios in
// `e2e/scenarios/*.exp` against a real zsh + pty.
//
// We tried writing a Go-level pty integration of Run() against a
// creack/pty pair, but the resulting harness was fragile (the
// master-side drain goroutine, raw-mode timing, and submit/cancel
// cleanup interact in non-obvious ways). The e2e docker tests run
// against the actual binary so they catch the same bugs more
// reliably; we don't duplicate them here.
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
