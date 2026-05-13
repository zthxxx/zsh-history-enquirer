package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestMultiWordSearch ports e2e/scenarios/04-multi-word-search.exp.
// Type "log iso" — only "git log --pretty=fuller --date=iso -n 1"
// matches both tokens; assert the unique tail "fuller" is rendered.
func TestMultiWordSearch(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("log iso"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("multi-word-match", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("fuller")
		})

	cancelAndExit(t, s)
}
