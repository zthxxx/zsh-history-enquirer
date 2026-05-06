package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/zthxxx/zsh-history-enquirer/internal/history"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

// cursorResult is the inner channel payload from the parallel DSR
// probe goroutine. Defined as a package-level type so init.go and
// run.go can share it without one importing the other.
type cursorResult struct {
	row, col int
	err      error
}

type historyResult struct {
	lines []string
	err   error
}

// fetchInitialState fires the cursor-probe and history-load
// goroutines in parallel and joins on both. Pulling this out of
// Run() keeps the orchestration step linear and lets each side
// fail independently without bringing the picker down.
func fetchInitialState(
	ctx context.Context,
	t *tty.TTY,
	loader history.Loader,
	_ io.Writer,
) (cursorResult, historyResult) {
	cursorCh := make(chan cursorResult, 1)
	historyCh := make(chan historyResult, 1)

	go func() {
		probe := tty.NewProbe(t)
		row, col, err := probe.Cursor(ctx, CursorTimeout)
		cursorCh <- cursorResult{row, col, err}
	}()
	go func() {
		lines, err := loader.Load(ctx)
		historyCh <- historyResult{lines, err}
	}()

	cur := <-cursorCh
	hist := <-historyCh
	return cur, hist
}

// readGeometry queries the TTY for its rows/cols, falling back to
// 24x80 when TIOCGWINSZ reports zero (docker pty without SIGWINCH).
func readGeometry(t *tty.TTY) (rows, cols int, err error) {
	rows, cols, err = t.Size()
	if err != nil {
		return 0, 0, fmt.Errorf("query size: %w", err)
	}
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}
	return rows, cols, nil
}

// handleProbeFallback applies the cursor-probe error fallback in
// place. Returns any leftover bytes the probe consumed so the
// caller can replay them through the keystream parser.
func handleProbeFallback(cur *cursorResult, cfg *Config, stderr io.Writer) string {
	if cur.err == nil {
		return ""
	}
	var leftover string
	var te *tty.TimeoutError
	if errors.As(cur.err, &te) {
		leftover = te.Leftover
	}
	_, _ = fmt.Fprintf(stderr, "warning: DSR cursor probe failed: %v (using col=1 fallback)\n", cur.err)
	cur.row = 1
	cur.col = len(cfg.Input) + 1
	return leftover
}

// clampCursor enforces the cur.{row,col} into the terminal bounds.
// Defends against bytes that happened to match the DSR shape but
// were never a real response.
func clampCursor(cur *cursorResult, cfg *Config, rows, cols int) {
	if cur.col < 1 || cur.col > cols {
		cur.col = len(cfg.Input) + 1
	}
	if cur.row < 1 || cur.row > rows {
		cur.row = 1
	}
}

// computeInitCol returns the column at which the picker's input row
// starts — the captured cursor minus the initial input length, with
// a one-column floor so prefix arithmetic never goes negative.
func computeInitCol(curCol, inputLen, cols int) int {
	initCol := curCol - inputLen
	if initCol < 1 || initCol > cols {
		initCol = 1
	}
	return initCol
}
