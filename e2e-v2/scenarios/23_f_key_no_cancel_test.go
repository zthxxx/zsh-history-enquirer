package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestFKeyNoCancel ports e2e/scenarios/23-f-key-no-cancel.exp.
// F1 (\eOP) and F2 (\eOQ) sent inside the picker must NOT cancel.
// We type "git ", press F1+F2, then "log" — if F1 had cancelled,
// "log" would land on the surrounding zsh prompt and "fuller"
// would never appear in the picker.
func TestFKeyNoCancel(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("git "); err != nil {
		t.Fatalf("type git: %v", err)
	}
	s.WaitQuiescent(150*time.Millisecond, 1*time.Second)

	// F1 + F2 — picker must remain open.
	if err := s.SendKey(harness.KeyF1); err != nil {
		t.Fatalf("F1: %v", err)
	}
	s.WaitQuiescent(150*time.Millisecond, 1*time.Second)
	if err := s.SendKey(harness.KeyF2); err != nil {
		t.Fatalf("F2: %v", err)
	}
	s.WaitQuiescent(150*time.Millisecond, 1*time.Second)

	// Continue typing "log" — only lands in the picker if it survived.
	if err := s.Type("log"); err != nil {
		t.Fatalf("type log: %v", err)
	}
	s.WaitFor("fuller-after-fkeys", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("fuller")
		})

	cancelAndExit(t, s)
}
