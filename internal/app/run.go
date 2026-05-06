package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

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
		// Printing version does not need the TTY at all.
		_, _ = io.WriteString(stderr, VersionLine()+"\n")
		return &RunResult{Output: ""}, nil
	}

	// Step 1: cursor probe + history load in parallel.
	type cursorResult struct {
		row, col int
		err      error
	}
	type historyResult struct {
		lines []string
		err   error
	}
	cursorCh := make(chan cursorResult, 1)
	historyCh := make(chan historyResult, 1)

	if err := t.EnterRaw(); err != nil {
		return nil, fmt.Errorf("enter raw: %w", err)
	}
	defer func() { _ = t.LeaveRaw() }()

	// Hide cursor + bracketed paste on; flip back at exit.
	_, _ = io.WriteString(t.Writer(), ansi.HideCursor+ansi.BracketedPasteOn)
	defer func() {
		_, _ = io.WriteString(t.Writer(), ansi.BracketedPasteOff+ansi.ShowCursor)
	}()

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

	if hist.err != nil {
		// Even with no history we should be able to run; show empty.
		_, _ = fmt.Fprintf(stderr, "warning: history load failed: %v\n", hist.err)
		hist.lines = nil
	}

	rows, cols, err := t.Size()
	if err != nil {
		return nil, fmt.Errorf("query size: %w", err)
	}

	// Cursor probe is best-effort. Some terminals (and most test pty
	// runners) do not reply to DSR within the timeout; fall back to
	// (row=1, col=len(input)+1) so the picker still draws — inline
	// alignment with the host shell prompt is lost on the fallback
	// path, but the picker remains fully usable.
	var probeLeftover string
	if cur.err != nil {
		var te *tty.TimeoutError
		if errors.As(cur.err, &te) {
			probeLeftover = te.Leftover
		}
		_, _ = fmt.Fprintf(stderr, "warning: DSR cursor probe failed: %v (using col=1 fallback)\n", cur.err)
		cur.row = 1
		cur.col = len(cfg.Input) + 1
	}
	if os.Getenv("ZHE_DEBUG") != "" {
		if f, derr := os.OpenFile(os.Getenv("ZHE_DEBUG"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); derr == nil {
			fmt.Fprintf(f, "[zhe] probe: row=%d col=%d err=%v leftover=%q\n", cur.row, cur.col, cur.err, probeLeftover)
			f.Close()
		}
	}

	// Defence-in-depth: a poorly-behaved emulator may reply with row
	// or col values outside the terminal — bytes that happened to
	// match the DSR shape but were never a real response. Clamp to
	// reasonable bounds before doing the initCol arithmetic so a
	// stray "row=99999" reply does not push the rendered body off-
	// screen and silently swallow it.
	if cur.col < 1 || cur.col > cols {
		cur.col = len(cfg.Input) + 1
	}
	if cur.row < 1 || cur.row > rows {
		cur.row = 1
	}

	initCol := cur.col - len(cfg.Input)
	if initCol < 1 {
		initCol = 1
	}
	if initCol > cols {
		initCol = 1
	}

	model := ui.NewModel(cfg.Input, hist.lines, rows, cols, cur.row, initCol, cfg.MaxLimit)

	// Step 2: drive event loop.
	reader := keys.NewReader(t)
	// Replay any bytes the cursor probe consumed while waiting for
	// the DSR response. Without this, the first 1-N keystrokes typed
	// by an impatient user (or by a test runner whose pty does not
	// reply to DSR) would be silently dropped.
	preEvents := reader.Prefeed(probeLeftover)
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := reader.Events(loopCtx)

	throttle := ui.NewThrottle(RenderInterval)
	prevSize := 0

	render := func(force bool) {
		if !force && !throttle.Fire(time.Now()) {
			return
		}
		frame := model.Render(ui.RenderOptions{PrevSize: prevSize})
		_, _ = io.WriteString(t.Writer(), frame.Pre+frame.Body+frame.Post)
		prevSize = frame.Size
	}

	render(true)

	var debugW io.Writer
	if path := os.Getenv("ZHE_DEBUG"); path != "" {
		if f, ferr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); ferr == nil {
			debugW = f
			defer f.Close()
		}
	}

	// Process any events that came from the probe-leftover bytes.
	for _, ev := range preEvents {
		if debugW != nil {
			fmt.Fprintf(debugW, "[zhe] preevent: %+v input=%q\n", ev, model.Input)
		}
		if model.Update(ev) {
			render(true)
			return &RunResult{Output: model.Result}, nil
		}
	}
	if len(preEvents) > 0 {
		render(false)
	}

	// trailingFlush fires shortly after the last event so that the
	// final state of a burst (a paste, a fast-typed word) reaches
	// the screen even when the leading-edge throttle blocked the
	// per-event renders.
	trailingFlush := time.NewTimer(time.Hour)
	trailingFlush.Stop()
	armTrailing := func() {
		if !trailingFlush.Stop() {
			select {
			case <-trailingFlush.C:
			default:
			}
		}
		trailingFlush.Reset(RenderInterval)
	}
	defer trailingFlush.Stop()

	for {
		select {
		case <-ctx.Done():
			model.Cancelled = true
			model.Result = model.Input
			render(true)
			return &RunResult{Output: model.Result}, ctx.Err()
		case ev, ok := <-events:
			if !ok {
				model.Cancelled = true
				model.Result = model.Input
				render(true)
				return &RunResult{Output: model.Result}, errors.New("input closed")
			}
			if debugW != nil {
				fmt.Fprintf(debugW, "[zhe] event: %+v input=%q\n", ev, model.Input)
			}
			if model.Update(ev) {
				render(true)
				return &RunResult{Output: model.Result}, nil
			}
			render(false)
			armTrailing()
		case <-trailingFlush.C:
			// Throttle window has elapsed since the last event; flush
			// whatever the latest model state is so the user sees the
			// final view of a burst.
			render(true)
		}
	}
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

// HandleError produces the exit-0-always behaviour required by the
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

// Default os.Stderr proxy so callers in tests can swap it out cleanly.
//
//nolint:gochecknoglobals // intentional swap point.
var Stderr io.Writer = os.Stderr
