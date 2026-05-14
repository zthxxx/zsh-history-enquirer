package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestNarrowTerminalWrap ports e2e/scenarios/16-narrow-terminal-wrap.exp.
// Force 24×40 geometry. The picker must keep rendering with long
// entries wrapping across rows; the seed's "echo ok" must still be
// visible.
func TestNarrowTerminalWrap(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{Rows: 24, Cols: 40})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	s.WaitFor("echo-ok-in-narrow", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	cancelAndExit(t, s)
}
