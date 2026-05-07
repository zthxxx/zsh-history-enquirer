package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/creack/pty"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
	"github.com/zthxxx/zsh-history-enquirer/internal/ui"
)

// TestReadGeometry_ReportsExplicitWinsize pins the happy-path
// case: when TIOCGWINSZ returns positive rows/cols, readGeometry
// surfaces them verbatim. Uses pty + IoctlSetWinsize so the test
// is independent of the host terminal.
func TestReadGeometry_ReportsExplicitWinsize(t *testing.T) {
	t.Parallel()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer slave.Close()

	require.NoError(t, unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: 30, Col: 100,
	}), "TIOCSWINSZ must succeed on the pty slave")

	ttyHandle, err := tty.NewFromFile(slave)
	require.NoError(t, err)

	rows, cols, err := readGeometry(ttyHandle)
	require.NoError(t, err)
	require.Equal(t, 30, rows)
	require.Equal(t, 100, cols)
}

// TestRunEventLoop_SubmitReturnsFocusedEntry pins the happy path:
// feed a single KeyEnter event with a non-empty filtered list, and
// the loop should return a RunResult whose Output is the focused
// entry.
func TestRunEventLoop_SubmitReturnsFocusedEntry(t *testing.T) {
	t.Parallel()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer slave.Close()

	require.NoError(t, unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: 24, Col: 80,
	}))
	ttyHandle, err := tty.NewFromFile(slave)
	require.NoError(t, err)

	model := ui.NewModel("git", []string{"git status", "git log"}, 24, 80, 1, 1, ui.DefaultMaxLimit)
	require.NotEmpty(t, model.Filter, "fixture must have at least one match")

	events := make(chan keys.Event, 1)
	events <- keys.KeyEvent{Key: keys.KeyEnter}
	close(events)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Drain master so writes from runEventLoop don't block the pty buffer.
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, rerr := master.Read(buf); rerr != nil {
				return
			}
		}
	}()

	result, err := runEventLoop(ctx, ttyHandle, model, events, nil, io.Discard)
	require.NoError(t, err, "submit must terminate cleanly with no error")
	require.NotNil(t, result)
	require.Equal(t, "git status", result.Output,
		"Enter on the focused first match must return that entry")
	require.True(t, model.Submitted, "model must record the submit")
}

// TestRunEventLoop_CtxDoneCancelsAndReturnsInput pins the cancel
// path triggered by an external context cancellation (e.g. a
// SIGTERM through signal.NotifyContext). The loop must set
// model.Canceled, set Result = Input, and return a non-nil result
// alongside ctx.Err() — so invokeRun's preserveOnError still has a
// RunResult to print and the user's typed text survives the
// teardown.
func TestRunEventLoop_CtxDoneCancelsAndReturnsInput(t *testing.T) {
	t.Parallel()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer slave.Close()

	require.NoError(t, unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: 24, Col: 80,
	}))
	ttyHandle, err := tty.NewFromFile(slave)
	require.NoError(t, err)

	model := ui.NewModel("typed-input", []string{"any"}, 24, 80, 1, 1, ui.DefaultMaxLimit)
	events := make(chan keys.Event)

	go func() {
		buf := make([]byte, 4096)
		for {
			if _, rerr := master.Read(buf); rerr != nil {
				return
			}
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already-canceled ctx — loop must observe it on first select

	result, err := runEventLoop(ctx, ttyHandle, model, events, nil, io.Discard)
	require.ErrorIs(t, err, context.Canceled,
		"loop must surface ctx.Err on cancel so HandleError can dispatch")
	require.NotNil(t, result, "result must be non-nil so PrintResult writes Input back")
	require.Equal(t, "typed-input", result.Output,
		"cancel path returns Input verbatim — widget contract")
	require.True(t, model.Canceled, "model must record the cancel")
}

// TestReadGeometry_FallsBackOnZeroSize pins the docker-pty fallback:
// some pty configurations (notably docker's pty without an explicit
// SIGWINCH driver) report rows=0, cols=0 from TIOCGWINSZ. Without
// the fallback, the picker would draw against a 0×0 budget and
// produce a degenerate frame. readGeometry promotes those to 24×80
// so the picker still has a usable canvas.
func TestReadGeometry_FallsBackOnZeroSize(t *testing.T) {
	t.Parallel()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer slave.Close()

	// Explicitly clear the winsize. Some pty implementations leave
	// row/col uninitialized at Open time; setting both to zero pins
	// the test to the fallback path regardless.
	require.NoError(t, unix.IoctlSetWinsize(int(slave.Fd()), unix.TIOCSWINSZ, &unix.Winsize{
		Row: 0, Col: 0,
	}))

	ttyHandle, err := tty.NewFromFile(slave)
	require.NoError(t, err)

	rows, cols, err := readGeometry(ttyHandle)
	require.NoError(t, err)
	require.Equal(t, 24, rows, "rows<=0 must fall back to 24")
	require.Equal(t, 80, cols, "cols<=0 must fall back to 80")
}

func TestComputeInitCol_Normal(t *testing.T) {
	t.Parallel()
	// Cursor was at column 12; input was 5 chars long → prompt starts at 7.
	got := computeInitCol(12, 5, 80)
	if got != 7 {
		t.Fatalf("computeInitCol(12, 5, 80) = %d, want 7", got)
	}
}

func TestComputeInitCol_NegativeFallsToOne(t *testing.T) {
	t.Parallel()
	// 5 - 8 = -3 → clamp up to 1.
	got := computeInitCol(5, 8, 80)
	if got != 1 {
		t.Fatalf("computeInitCol(5, 8, 80) = %d, want 1", got)
	}
}

func TestComputeInitCol_OverflowFallsToOne(t *testing.T) {
	t.Parallel()
	// curCol > cols → clamp to 1.
	got := computeInitCol(200, 5, 80)
	if got != 1 {
		t.Fatalf("computeInitCol(200, 5, 80) = %d, want 1", got)
	}
}

func TestClampCursor_BoundsClamp(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "abc"}
	cur := cursorResult{row: 9999, col: 9999}
	clampCursor(&cur, cfg, 24, 80)
	if cur.row != 1 {
		t.Fatalf("row = %d, want 1", cur.row)
	}
	// col is reset to len(Input)+1 = 4.
	if cur.col != 4 {
		t.Fatalf("col = %d, want 4 (len(Input)+1)", cur.col)
	}
}

func TestClampCursor_InBoundsUnchanged(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "abc"}
	cur := cursorResult{row: 5, col: 10}
	clampCursor(&cur, cfg, 24, 80)
	if cur.row != 5 || cur.col != 10 {
		t.Fatalf("clamping changed in-bounds value: row=%d col=%d", cur.row, cur.col)
	}
}

// TestClampCursor_NonASCIIUsesCellWidth pins the regression where
// the fallback computed `cur.col = len(cfg.Input) + 1` in bytes.
// We now route through ui.CellWidth so the column count matches
// the actual rendered cells: 1 per ASCII / Latin-extended / Cyrillic
// / Greek rune, 2 per CJK / emoji rune.
func TestClampCursor_NonASCIIUsesCellWidth(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "café"}
	cur := cursorResult{row: 9999, col: 9999}
	clampCursor(&cur, cfg, 24, 80)
	// "café" is 4 cells; col = 4 + 1 = 5.
	if cur.col != 5 {
		t.Fatalf("col = %d, want 5 (cells + 1)", cur.col)
	}
}

// TestHandleProbeFallback_NonASCIIUsesCellWidth mirrors the fix on
// the probe-failure path. Verifies CJK gets the proper 2-cell-per-
// glyph treatment (East Asian Width-aware).
func TestHandleProbeFallback_NonASCIIUsesCellWidth(t *testing.T) {
	t.Parallel()
	var stderr bytes.Buffer
	cfg := &Config{Input: "你好"}
	cur := cursorResult{err: &tty.TimeoutError{Cause: errors.New("silent")}}
	_ = handleProbeFallback(&cur, cfg, &stderr)
	// "你好" is 4 cells (2 per CJK glyph); col = 4 + 1 = 5.
	if cur.col != 5 {
		t.Fatalf("col = %d, want 5 (CJK cells + 1)", cur.col)
	}
}

func TestHandleProbeFallback_NilErrIsNoOp(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "x"}
	cur := cursorResult{row: 3, col: 4, err: nil}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "" {
		t.Fatalf("leftover = %q, want empty", leftover)
	}
	if cur.row != 3 || cur.col != 4 {
		t.Fatalf("unchanged probe was modified: row=%d col=%d", cur.row, cur.col)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty for nil err, got %q", stderr.String())
	}
}

// TestHandleProbeFallback_NilErrPropagatesLeftover pins the
// success-with-leftover path: when the user typed something during
// the DSR probe window and the probe parsed cleanly anyway,
// cur.leftover holds those bytes and handleProbeFallback must
// surface them so Reader.Prefeed sees them. Without this, fast-
// typing users pressing `^R git` would lose every keystroke that
// landed before the picker's first render — a UX regression that's
// invisible to the user (they see no error; they just see the
// picker open with their first chars missing).
func TestHandleProbeFallback_NilErrPropagatesLeftover(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: ""}
	cur := cursorResult{row: 3, col: 4, leftover: "git ", err: nil}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "git " {
		t.Fatalf("leftover = %q, want %q (probe-success leftover must propagate)",
			leftover, "git ")
	}
	if cur.row != 3 || cur.col != 4 {
		t.Fatalf("success path must NOT mutate row/col: row=%d col=%d", cur.row, cur.col)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty for nil err, got %q", stderr.String())
	}
}

func TestHandleProbeFallback_TimeoutErrorReturnsLeftover(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "abc"}
	cur := cursorResult{
		err: &tty.TimeoutError{
			Cause:    errors.New("read /dev/tty: i/o timeout"),
			Leftover: "log",
		},
	}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "log" {
		t.Fatalf("leftover = %q, want %q", leftover, "log")
	}
	if cur.row != 1 {
		t.Fatalf("row fallback = %d, want 1", cur.row)
	}
	if cur.col != 4 {
		t.Fatalf("col fallback = %d, want len(Input)+1 = 4", cur.col)
	}
	if !strings.Contains(stderr.String(), "DSR cursor probe failed") {
		t.Fatalf("stderr should warn, got %q", stderr.String())
	}
}

func TestHandleProbeFallback_NonTimeoutErrorWithoutLeftoverStaysEmpty(t *testing.T) {
	t.Parallel()
	// Generic non-timeout error path (e.g. write-DSR failure, poll
	// failure): Probe.Cursor returns cur.leftover == "" because the
	// failure happened before any read accumulated bytes. Fallback
	// must surface that empty leftover unchanged — there's nothing to
	// replay, but we should not synthesize garbage either.
	cfg := &Config{Input: "x"}
	cur := cursorResult{err: errors.New("write failed")}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "" {
		t.Fatalf("leftover = %q, want empty for non-timeout error with empty cur.leftover", leftover)
	}
}

// TestHandleProbeFallback_MalformedParseErrorRoundTripsLeftover pins
// the regression where a malformed-DSR-parse error dropped every byte
// the probe had consumed.
//
// The realistic trigger is a malformed body that the scan-forward
// parser also cannot recover (e.g. `\x1b[abc;1R` — letters in the
// row position). parseDSRResponse populates leftover with the entire
// input string and returns the malformed-parse error. Pre-fix:
// handleProbeFallback only honored TimeoutError.Leftover, so the
// bytes were silently dropped. Post-fix: cur.leftover is the default
// fallthrough, so the bytes round-trip through reader.Prefeed.
//
// Note: the more common fast-typing-arrow shape (`\x1b[A\x1b[12;5R`)
// is now resolved cleanly by the scan-forward parseDSRResponse and
// never reaches this fallback. That case is exercised in
// TestParseDSRResponse_ScanForwardSkipsNonDSRCSI.
func TestHandleProbeFallback_MalformedParseErrorRoundTripsLeftover(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "abc"}
	// Mirror what parseDSRResponse populates on a malformed parse:
	// leftover = full input string, err = malformed-DSR diagnostic.
	cur := cursorResult{
		leftover: "\x1b[abc;1R",
		err:      errors.New(`malformed DSR response "\x1b[abc;1R" (non-numeric)`),
	}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "\x1b[abc;1R" {
		t.Fatalf("leftover = %q, want full probe input on malformed parse", leftover)
	}
	if cur.col != 4 {
		t.Fatalf("col fallback = %d, want len(Input)+1 = 4", cur.col)
	}
	if !strings.Contains(stderr.String(), "DSR cursor probe failed") {
		t.Fatalf("stderr should warn, got %q", stderr.String())
	}
}

func TestPrintResult_NilOutputSkipsWrite(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintResult(&buf, nil)
	if buf.Len() != 0 {
		t.Fatalf("nil RunResult should write nothing, got %q", buf.String())
	}
}

func TestPrintResult_EmptyOutputSkipsWrite(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintResult(&buf, &RunResult{Output: ""})
	if buf.Len() != 0 {
		t.Fatalf("empty Output should write nothing, got %q", buf.String())
	}
}

func TestPrintResult_AppendsTrailingNewline(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	PrintResult(&buf, &RunResult{Output: "git status"})
	if buf.String() != "git status\n" {
		t.Fatalf("PrintResult got %q, want %q", buf.String(), "git status\n")
	}
}

func TestHandleError_NilReturnsZero(t *testing.T) {
	t.Parallel()
	if HandleError(&bytes.Buffer{}, nil) != 0 {
		t.Fatal("HandleError(nil) should return 0")
	}
}

func TestHandleError_NonNilStillReturnsZero(t *testing.T) {
	t.Parallel()
	// Widget contract: every termination path exits 0 so
	// `BUFFER=$(...)` is not aborted.
	var buf bytes.Buffer
	rc := HandleError(&buf, errors.New("boom"))
	if rc != 0 {
		t.Fatalf("HandleError(err) = %d, widget contract requires 0", rc)
	}
	if !strings.Contains(buf.String(), "boom") {
		t.Fatalf("stderr should contain error message, got %q", buf.String())
	}
}

// TestHandleError_ContextCanceledIsSilent pins the ctx.Err() branch:
// a parent-cancelled run (Ctrl-C upstream of the picker, e.g. the
// surrounding shell sent SIGINT to the whole pipeline) is a "user
// asked to stop" signal — not an error worth printing. The widget
// contract still expects exit 0, but stderr must stay empty so the
// surrounding shell doesn't get a spurious "context canceled" line
// after every aborted Ctrl-R.
func TestHandleError_ContextCanceledIsSilent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	rc := HandleError(&buf, context.Canceled)
	if rc != 0 {
		t.Fatalf("HandleError(ctx.Canceled) = %d, widget contract requires 0", rc)
	}
	if buf.Len() != 0 {
		t.Fatalf("ctx.Canceled must not write to stderr, got %q", buf.String())
	}
}

// TestHandleError_WrappedContextCanceledIsSilent — errors.Is unwraps
// through fmt.Errorf("%w", …) wrappers, so a runEventLoop layer that
// returns `fmt.Errorf("loop: %w", context.Canceled)` must still take
// the silent branch. Pins the unwrap behaviour explicitly so a
// future refactor can't switch the check to `==` without breaking
// this contract.
func TestHandleError_WrappedContextCanceledIsSilent(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	wrapped := fmt.Errorf("loop terminated: %w", context.Canceled)
	rc := HandleError(&buf, wrapped)
	if rc != 0 {
		t.Fatalf("HandleError(wrapped ctx.Canceled) = %d, widget contract requires 0", rc)
	}
	if buf.Len() != 0 {
		t.Fatalf("wrapped ctx.Canceled must not write to stderr, got %q", buf.String())
	}
}

// TestOpenDebugLog_Unset returns io.Discard when ZHE_DEBUG is not
// set. Discarder is the load-bearing default — production runs
// must not pay any cost for diagnostic logging.
func TestOpenDebugLog_Unset(t *testing.T) {
	t.Setenv("ZHE_DEBUG", "")
	w := openDebugLog()
	require.Equal(t, io.Discard, w)
}

// TestOpenDebugLog_Writable opens a real file and verifies writes
// land on disk.
func TestOpenDebugLog_Writable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "zhe.log")
	t.Setenv("ZHE_DEBUG", path)

	w := openDebugLog()
	require.NotEqual(t, io.Discard, w)

	_, err := io.WriteString(w, "hello\n")
	require.NoError(t, err)
	if c, ok := w.(io.Closer); ok {
		_ = c.Close()
	}

	got, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "hello\n", string(got))
}

// TestOpenDebugLog_UnwritableFallsBackToDiscard — pointing
// ZHE_DEBUG at an unwritable path (e.g., a directory or a
// nonexistent parent) must not crash the picker; we degrade to
// io.Discard so diagnostics are best-effort.
func TestOpenDebugLog_UnwritableFallsBackToDiscard(t *testing.T) {
	// /this/path/does/not/exist will fail OpenFile with ENOENT.
	t.Setenv("ZHE_DEBUG", "/this/path/does/not/exist/zhe.log")
	require.Equal(t, io.Discard, openDebugLog())
}

// TestDebugProbe_DiscardIsNoOp — debugProbe must not write when the
// destination is io.Discard or nil. Saves cost on the production
// path.
func TestDebugProbe_DiscardIsNoOp(t *testing.T) {
	t.Parallel()
	debugProbe(io.Discard, cursorResult{row: 1, col: 1}, "", 24, 80)
	debugProbe(nil, cursorResult{row: 1, col: 1}, "", 24, 80)
}

// TestDebugProbe_WritesProbeAndGeom — debugProbe writes both lines
// to the destination when it is a real writer.
func TestDebugProbe_WritesProbeAndGeom(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	debugProbe(&buf, cursorResult{row: 7, col: 42}, "log", 24, 80)
	require.Contains(t, buf.String(), "row=7 col=42")
	require.Contains(t, buf.String(), "leftover=\"log\"")
	require.Contains(t, buf.String(), "rows=24 cols=80")
}

// panickingLoader is a test-only Loader that panics during Load.
// Used to exercise fetchInitialState's panic-recovery defer.
type panickingLoader struct{ msg string }

func (p panickingLoader) Load(_ context.Context) ([]string, error) {
	panic(p.msg)
}

// TestFetchInitialState_LoaderPanicConvertsToError pins that a panic
// inside the history-load goroutine is recovered and surfaced as an
// error result, so the parent join doesn't hang on a never-closed
// channel and the picker still opens with empty history. Without
// this defense the panic would propagate and crash the process.
func TestFetchInitialState_LoaderPanicConvertsToError(t *testing.T) {
	t.Parallel()
	master, slave, err := pty.Open()
	require.NoError(t, err)
	defer master.Close()
	defer slave.Close()
	ttyHandle, err := tty.NewFromFile(slave)
	require.NoError(t, err)

	loader := panickingLoader{msg: "loader blew up"}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, hist := fetchInitialState(ctx, ttyHandle, loader, io.Discard)

	require.Error(t, hist.err, "panic must be reported as the loader's err")
	require.Contains(t, hist.err.Error(), "history load panic",
		"err message must surface the recovered panic for debugging")
	require.Contains(t, hist.err.Error(), "loader blew up",
		"original panic value must round-trip into the err message")
}

// TestDebugEvent_DiscardIsNoOp pins the io.Discard / nil short-
// circuit on debugEvent. The hot loop calls this once per event;
// production must not pay the fmt.Fprintf cost. m can be nil
// because the no-op path returns before dereferencing it.
func TestDebugEvent_DiscardIsNoOp(t *testing.T) {
	t.Parallel()
	debugEvent(io.Discard, "key", keys.PasteEvent{Payload: "a"}, nil)
	debugEvent(nil, "key", keys.PasteEvent{Payload: "a"}, nil)
}

// TestDebugEvent_WritesWhenEnabled covers the active branch with a
// real bytes.Buffer + a minimal Model. The output format is
// stable enough to grep ("[zhe] <label>:").
func TestDebugEvent_WritesWhenEnabled(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := ui.NewModel("typed-input", []string{}, 24, 80, 1, 1, 15)
	debugEvent(&buf, "key", keys.RuneEvent{R: 'a'}, m)
	require.Contains(t, buf.String(), "[zhe] key:")
	require.Contains(t, buf.String(), "input=\"typed-input\"")
}
