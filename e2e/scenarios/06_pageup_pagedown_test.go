package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestPageUpPageDown ports e2e/scenarios/06-pageup-pagedown.exp.
// PgDn x2 + PgUp x1 — the picker must keep rendering throughout.
func TestPageUpPageDown(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if !s.Screen().Contains("echo ok") {
		t.Fatalf("expected 'echo ok' on screen\nscreen:\n%s", s.Screen().Dump())
	}

	// PgDn × 2.
	for i := 0; i < 2; i++ {
		if err := s.SendKey(harness.KeyPageDown); err != nil {
			t.Fatalf("PgDn %d: %v", i, err)
		}
		s.WaitQuiescent(100*time.Millisecond, 800*time.Millisecond)
	}

	if !strings.Contains(s.Screen().Dump(), "command-") {
		t.Fatalf("no command- after PgDn x2\nscreen:\n%s", s.Screen().Dump())
	}

	if err := s.SendKey(harness.KeyPageUp); err != nil {
		t.Fatalf("PgUp: %v", err)
	}
	s.WaitQuiescent(100*time.Millisecond, 800*time.Millisecond)

	cancelAndExit(t, s)
}
