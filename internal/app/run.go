package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
	"unicode/utf8"

	"github.com/zthxxx/zsh-history-enquirer/internal/ansi"
	"github.com/zthxxx/zsh-history-enquirer/internal/history"
	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
	"github.com/zthxxx/zsh-history-enquirer/internal/ui"
)

// CursorTimeout is how long we wait for the terminal to reply to the
// DSR cursor probe. 250 ms is generous; legitimate terminals reply
// within ~5 ms, but the wait is invisible to the user because it
// overlaps with the history fetch.
const CursorTimeout = 250 * time.Millisecond

// RenderInterval is the throttle window. Mirrors the legacy 72 ms.
const RenderInterval = 72 * time.Millisecond

// RunResult carries the picker's chosen output line. Run() writes
// this string to the supplied stdout and exits 0 on every termination
// path — see spec/10-widget-contract.md.
type RunResult struct {
	Output string
}

// Run is the application's main loop. It:
//
//  1. opens /dev/tty,
//  2. queries the cursor position in parallel with loading history,
//  3. enters raw mode + bracketed paste mode,
//  4. drives the model/event/render cycle until termination,
//  5. cleans up the terminal and returns the chosen output.
//
// All errors are wrapped; failure restores the terminal before
// returning so the caller's `fmt.Println(result.Output); os.Exit(0)`
// is the last visible side effect either way.
func Run(ctx context.Context, cfg *Config, t *tty.TTY, loader history.Loader, stderr io.Writer) (*RunResult, error) {
	if cfg.PrintVersion {
		// Defensive — main.go already short-circuits --version
		// before fx wires the TTY. If we still got here, route the
		// version line back through the RunResult so the umbrella
		// invokeRun prints it via PrintResult (i.e. to stdout).
		return &RunResult{Output: VersionLine()}, nil
	}

	if err := t.EnterRaw(); err != nil {
		return nil, fmt.Errorf("enter raw: %w", err)
	}
	defer func() { _ = t.LeaveRaw() }()

	// Hide cursor + bracketed paste on; flip back at exit.
	_, _ = io.WriteString(t.Writer(), ansi.HideCursor+ansi.BracketedPasteOn)
	defer func() {
		_, _ = io.WriteString(t.Writer(), ansi.BracketedPasteOff+ansi.ShowCursor)
	}()

	debugW := openDebugLog()
	if c, ok := debugW.(io.Closer); ok && debugW != io.Discard {
		defer func() {
			//nolint:errcheck // best-effort log close
			_ = c.Close()
		}()
	}

	cur, hist := fetchInitialState(ctx, t, loader, stderr)
	if hist.err != nil {
		_, _ = fmt.Fprintf(stderr, "warning: history load failed: %v\n", hist.err)
		hist.lines = nil
	}

	rows, cols, err := readGeometry(t)
	if err != nil {
		return nil, err
	}

	probeLeftover := handleProbeFallback(&cur, cfg, stderr)
	debugProbe(debugW, cur, probeLeftover, rows, cols)
	clampCursor(&cur, cfg, rows, cols)

	// Use rune-count (cell approximation) rather than byte-count so the
	// initCol arithmetic survives non-ASCII LBUFFER text. For CJK input
	// the rune-count under-counts by ~1 cell per glyph (East Asian Width
	// not yet wired in); accepting that residual is still much better
	// than the previous bytes-based path which over-counted by 2-3×.
	initCol := computeInitCol(cur.col, utf8.RuneCountInString(cfg.Input), cols)
	model := ui.NewModel(cfg.Input, hist.lines, rows, cols, cur.row, initCol, cfg.MaxLimit)

	reader := keys.NewReader(t)
	preEvents := reader.Prefeed(probeLeftover)

	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := reader.Events(loopCtx)

	return runEventLoop(ctx, t, model, events, preEvents, debugW)
}

// PrintResult writes the chosen line to stdout exactly once. Bytes
// written here become the value of `BUFFER=$(...)` in the zsh
// widget. The output never contains a trailing newline beyond the
// one fmt.Println adds, because `$(...)` already strips that for us.
func PrintResult(stdout io.Writer, r *RunResult) {
	if r == nil || r.Output == "" {
		return
	}
	fmt.Fprintln(stdout, r.Output)
}

// HandleError produces the exit-0-always behavior required by the
// widget contract. The error is logged to stderr (which is invisible
// to the widget's `$(...)` capture) and the function returns 0.
func HandleError(stderr io.Writer, err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, context.Canceled) {
		return 0
	}
	fmt.Fprintf(stderr, "zsh-history-enquirer: %v\n", err)
	// Even on hard errors we exit 0: a non-zero exit code aborts the
	// `BUFFER=$(...)` substitution and loses whatever the user typed.
	return 0
}

// Stderr is a default os.Stderr proxy so callers in tests can swap it
// out cleanly.
//
//nolint:gochecknoglobals // intentional swap point.
var Stderr io.Writer = os.Stderr
