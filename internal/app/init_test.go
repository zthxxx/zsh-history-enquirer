package app

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

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

func TestHandleProbeFallback_NonTimeoutErrorClearsLeftover(t *testing.T) {
	t.Parallel()
	cfg := &Config{Input: "x"}
	cur := cursorResult{err: errors.New("write failed")}
	var stderr bytes.Buffer
	leftover := handleProbeFallback(&cur, cfg, &stderr)
	if leftover != "" {
		t.Fatalf("leftover = %q, want empty for non-timeout error", leftover)
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
