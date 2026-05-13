package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestCancelPreservesInput ports e2e/scenarios/03-cancel-preserves-input.exp.
// Pre-type a string that won't match any history entry, press ^R,
// the picker should show "(no matches)" and Esc should leave the
// typed input intact in BUFFER (cancel-preserves-input contract,
// see AGENTS.md).
func TestCancelPreservesInput(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{PreFilter: "qwerty-no-match"})

	s.Settle()

	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	s.WaitFor("no-matches-hint", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	cancelAndExit(t, s)
}
