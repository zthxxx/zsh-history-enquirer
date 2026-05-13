package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestPasteBracketed ports e2e/scenarios/05-paste-bracketed.exp.
// Send a bracketed-paste payload "git" — the picker must treat the
// payload as text (not key events) and filter to "git status".
func TestPasteBracketed(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.SendBracketedPaste([]byte("git")); err != nil {
		t.Fatalf("paste: %v", err)
	}
	s.WaitFor("git-status-rendered", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("git status")
		})

	cancelAndExit(t, s)
}
