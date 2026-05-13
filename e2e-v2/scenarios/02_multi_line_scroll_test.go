package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestMultiLineScroll ports e2e/scenarios/02-multi-line-scroll.exp.
// Filter to "command-", scroll down through ~8 entries crossing
// multi-line command-6 (6 rows) and command-8 (8 rows). The
// renderer must keep emitting command-N entries throughout.
func TestMultiLineScroll(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("command"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("command-filter-applied", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Contains(scr.Dump(), "-15")
		})

	for i := 0; i < 8; i++ {
		if err := s.SendKey(harness.KeyDown); err != nil {
			t.Fatalf("send Down %d: %v", i, err)
		}
		s.WaitQuiescent(60*time.Millisecond, 500*time.Millisecond)
	}

	// After 8 down-presses we should be far into the list; any
	// command-N rendering proves the renderer survived the multi-
	// line eviction math.
	if !strings.Contains(s.Screen().Dump(), "command-") {
		t.Fatalf("no command- rendering after deep scroll\nscreen:\n%s", s.Screen().Dump())
	}

	cancelAndExit(t, s)
}
