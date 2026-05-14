package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestInputEdit ports e2e/scenarios/17-input-edit.exp.
// In-picker editing: Backspace shrinks input + re-filters; Ctrl-U
// clears input + re-filters (locks the spec/50-keybindings contract).
func TestInputEdit(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("xyz"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("no-matches-after-xyz", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	// Backspace twice — input becomes "x", still no matches.
	if err := s.SendKey(harness.KeyBackspace); err != nil {
		t.Fatalf("bsp 1: %v", err)
	}
	if err := s.SendKey(harness.KeyBackspace); err != nil {
		t.Fatalf("bsp 2: %v", err)
	}
	s.WaitFor("no-matches-after-bsp", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	// Ctrl-U clears input — full history visible again.
	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("Ctrl-U: %v", err)
	}
	s.WaitFor("echo-ok-after-ctrlu", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	cancelAndExit(t, s)
}
