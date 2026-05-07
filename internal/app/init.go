package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/zthxxx/zsh-history-enquirer/internal/history"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
	"github.com/zthxxx/zsh-history-enquirer/internal/ui"
)

// cursorResult is the inner channel payload from the parallel DSR
// probe goroutine. Defined as a package-level type so init.go and
// run.go can share it without one importing the other.
//
// The leftover field carries any non-DSR bytes the probe consumed
// alongside the response — typically the user's first keystrokes
// after Ctrl-R that arrived before the picker had finished probing.
// On the fast path (probe succeeded with a clean buffer) this is
// empty; otherwise the caller re-feeds these bytes to the keystream
// parser via Reader.Prefeed so no input is lost.
type cursorResult struct {
	row, col int
	leftover string
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
		row, col, leftover, err := probe.Cursor(ctx, CursorTimeout)
		cursorCh <- cursorResult{row: row, col: col, leftover: leftover, err: err}
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
// caller can replay them through the keystream parser. Both code
// paths produce leftover:
//
//   - Success: the user typed something before / after the DSR
//     response and the probe parsed the response cleanly. Leftover
//     comes from cur.leftover, populated by Probe.Cursor's success
//     return.
//   - Failure: the probe timed out or hit a malformed response.
//     Leftover comes from TimeoutError.Leftover.
//
// The fallback (when err != nil) assumes the prompt starts at
// column 1 with the input filling the first inputCells cells; the
// cursor sits one cell past the last input cell. ui.CellWidth gives
// the precise cell count (East Asian Width-aware) so non-ASCII
// LBUFFER no longer mis-aligns the picker against the actual cursor
// position.
func handleProbeFallback(cur *cursorResult, cfg *Config, stderr io.Writer) string {
	if cur.err == nil {
		return cur.leftover
	}
	var leftover string
	var te *tty.TimeoutError
	if errors.As(cur.err, &te) {
		leftover = te.Leftover
	}
	_, _ = fmt.Fprintf(stderr, "warning: DSR cursor probe failed: %v (using col=1 fallback)\n", cur.err)
	cur.row = 1
	cur.col = ui.CellWidth(cfg.Input) + 1
	return leftover
}

// clampCursor enforces the cur.{row,col} into the terminal bounds.
// Defends against bytes that happened to match the DSR shape but
// were never a real response. Uses ui.CellWidth for the same
// reason as handleProbeFallback — exact cell math, not byte or
// rune approximation.
func clampCursor(cur *cursorResult, cfg *Config, rows, cols int) {
	if cur.col < 1 || cur.col > cols {
		cur.col = ui.CellWidth(cfg.Input) + 1
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
