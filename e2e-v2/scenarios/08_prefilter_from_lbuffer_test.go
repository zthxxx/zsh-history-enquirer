package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestPrefilterFromLBuffer ports e2e/scenarios/08-prefilter-from-lbuffer.exp.
// Type "git" BEFORE pressing ^R so zsh's $LBUFFER carries that into
// the picker as initial filter (the widget's `--$LBUFFER` arg path).
// Verifies the unique unhighlighted tail " status" is visible.
func TestPrefilterFromLBuffer(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{PreFilter: "git"})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	s.WaitFor("status-tail-visible", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains(" status")
		})

	cancelAndExit(t, s)
}
