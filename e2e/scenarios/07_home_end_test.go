package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestHomeEnd ports e2e/scenarios/07-home-end.exp.
// Filter to "command", scroll deep, press Home, assert the top
// match is back. Token highlighting splits "command-15" into SGR
// runs so we look for the unique unhighlighted tail "-15".
func TestHomeEnd(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("command"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("command-filter", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("-15")
		})

	// Scroll deep so Home has somewhere to return from.
	for i := 0; i < 2; i++ {
		if err := s.SendKey(harness.KeyPageDown); err != nil {
			t.Fatalf("PgDn: %v", err)
		}
		s.WaitQuiescent(100*time.Millisecond, 800*time.Millisecond)
	}

	if err := s.SendKey(harness.KeyHome); err != nil {
		t.Fatalf("Home: %v", err)
	}
	s.WaitFor("home-returns-top", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Contains(scr.Dump(), "-15")
		})

	cancelAndExit(t, s)
}
