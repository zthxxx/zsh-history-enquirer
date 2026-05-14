package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestMultilineScrollIntoView ports e2e/scenarios/11-multiline-scroll-into-view.exp.
// Filter to "command", scroll down past the single-line entries so
// a multi-line entry enters the visible window. The dynamic-limit
// math must shrink without dropping the focused row.
func TestMultilineScrollIntoView(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("command"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitQuiescent(150*time.Millisecond, 1*time.Second)

	// Walk Down 7 times to bring a multi-line entry into view.
	for i := 0; i < 7; i++ {
		if err := s.SendKey(harness.KeyDown); err != nil {
			t.Fatalf("Down %d: %v", i, err)
		}
		s.WaitQuiescent(60*time.Millisecond, 500*time.Millisecond)
	}

	s.WaitFor("multi-line-in-view", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Contains(scr.Dump(), "- line 0")
		})

	// Continue scrolling — other command-N entries should still be
	// reachable below the multi-line block.
	for i := 0; i < 5; i++ {
		if err := s.SendKey(harness.KeyDown); err != nil {
			t.Fatalf("Down %d: %v", i+7, err)
		}
		s.WaitQuiescent(60*time.Millisecond, 500*time.Millisecond)
	}

	if !strings.Contains(s.Screen().Dump(), "command") {
		t.Fatalf("expected command-N visible after deep scroll\nscreen:\n%s", s.Screen().Dump())
	}

	cancelAndExit(t, s)
}
