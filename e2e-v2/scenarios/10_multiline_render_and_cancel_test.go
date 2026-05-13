package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestMultilineRenderAndCancel ports e2e/scenarios/10-multiline-render-and-cancel.exp.
// Filter to "command" so the multi-line command-8 entry is in the
// visible window. Verify all 8 logical rows render, then cancel
// cleanly and verify the prompt is reachable for a sentinel command.
func TestMultilineRenderAndCancel(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("command"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("multi-line-rows", 2*time.Second,
		func(scr harness.Screen) bool {
			dump := scr.Dump()
			return strings.Contains(dump, "- line 0") &&
				strings.Contains(dump, "- line 5")
		})

	// Cancel + wipe BUFFER (filter "command" preserved by cancel).
	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("Esc: %v", err)
	}
	s.WaitFor("picker-gone", 3*time.Second,
		func(scr harness.Screen) bool { return !scr.Contains("›") })
	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("Ctrl-U: %v", err)
	}
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)

	// Sentinel: zsh prompt must be reachable post-cancel.
	if err := s.Type("echo done\r"); err != nil {
		t.Fatalf("sentinel: %v", err)
	}
	s.WaitFor("sentinel-output", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Count(scr.Dump(), "done") >= 2
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
