package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestLongLineWrap ports e2e/scenarios/14-long-line-wrap.exp.
// History has a 200-char entry. The renderer must wrap it onto
// multiple rows and the trailing 'Z' marker must appear, proving
// the full entry was emitted (not truncated).
func TestLongLineWrap(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{HistoryFixture: "14-long-line"})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	// "AZ" — the trailing "A" + the "Z" marker — only appears if the
	// renderer wrote the FULL wrapped entry to the tty.
	s.WaitFor("trailing-marker", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("AZ")
		})

	cancelAndExit(t, s)
}
