package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestCtrlWWordDelete ports e2e/scenarios/19-ctrl-w-word-delete.exp.
// Type "git xyz" — no matches. Ctrl-W strips "xyz" → filter "git "
// → " status" tail visible (highlighted "git" + space + "status").
// Another Ctrl-W strips "git " → empty filter → "echo ok" visible.
func TestCtrlWWordDelete(t *testing.T) {
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

	// Ctrl-W strips "xyz" — filter becomes "git ".
	if err := s.SendKey(harness.KeyCtrlW); err != nil {
		t.Fatalf("Ctrl-W 1: %v", err)
	}
	s.WaitFor("git-status-tail", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains(" status")
		})

	// Ctrl-W strips "git " — empty filter.
	if err := s.SendKey(harness.KeyCtrlW); err != nil {
		t.Fatalf("Ctrl-W 2: %v", err)
	}
	s.WaitFor("echo-ok-empty-filter", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	cancelAndExit(t, s)
}
