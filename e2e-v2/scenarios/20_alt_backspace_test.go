package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestAltBackspace ports e2e/scenarios/20-alt-backspace.exp.
// Alt+Backspace (\e\x7f) must map to backward-kill-word, not cancel.
// Two Alt-Backspaces from "git xyz" should arrive at empty filter.
func TestAltBackspace(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("git xyz"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("no-matches-git-xyz", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	// Alt+Backspace strips "xyz".
	if err := s.SendKey(harness.KeyAltBackspace); err != nil {
		t.Fatalf("Alt-Bsp 1: %v", err)
	}
	s.WaitFor("git-status-tail", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains(" status")
		})

	// Alt+Backspace strips "git ".
	if err := s.SendKey(harness.KeyAltBackspace); err != nil {
		t.Fatalf("Alt-Bsp 2: %v", err)
	}
	s.WaitFor("echo-ok-empty-filter", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	cancelAndExit(t, s)
}
