package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestUnicodeEntries ports e2e/scenarios/13-unicode-entries.exp.
// History contains CJK, accented Latin, and emoji entries. The
// renderer must measure cell width with East Asian Width
// awareness (rivo/uniseg). Filter to "café" — the unique accented
// entry must remain.
func TestUnicodeEntries(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{HistoryFixture: "13-unicode"})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	// The rocket emoji should appear in the initial render.
	s.WaitFor("emoji-rendered", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("🚀")
		})

	if err := s.Type("café"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitFor("accented-match", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("résumé")
		})

	cancelAndExit(t, s)
}
