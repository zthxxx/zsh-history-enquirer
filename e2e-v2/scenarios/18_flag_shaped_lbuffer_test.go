package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestFlagShapedLBuffer ports e2e/scenarios/18-flag-shaped-lbuffer.exp.
// Type "--version" (a flag-shaped string) before ^R — the plugin's
// `--` separator must protect the doc fast-path so the picker
// actually opens with "--version" as the filter (with no matches).
// Cancel preserves "--version" in BUFFER (cancel-preserves-input
// contract); Ctrl-U wipes; sentinel `echo recovered` confirms zsh
// is still healthy.
func TestFlagShapedLBuffer(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{PreFilter: "--version"})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	// Picker opened with "--version" filter; "(no matches)" expected.
	s.WaitFor("no-matches-flag", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	// Cancel — BUFFER returns to "--version".
	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("Esc: %v", err)
	}
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)
	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("Ctrl-U: %v", err)
	}
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)

	// Sentinel — proves zsh's prompt is healthy post-cancel.
	if err := s.Type("echo recovered\r"); err != nil {
		t.Fatalf("sentinel: %v", err)
	}
	s.WaitFor("sentinel-output", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Count(scr.Dump(), "recovered") >= 2
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
