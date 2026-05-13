package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestPasteWithControlBytes ports e2e/scenarios/22-paste-with-control-bytes.exp.
// Paste payload contains a raw 0x03 (Ctrl-C byte) between "git" and
// "log". Bracketed paste must deliver the whole payload as one
// EventPaste — the picker must NOT exit on the embedded 0x03 nor
// process it as a key event. With control bytes sanitized to
// spaces, the resulting filter is "git log" and the multi-token
// entry "git log --pretty=fuller --date=iso -n 1" is rendered.
func TestPasteWithControlBytes(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	// "git" + raw 0x03 + "log"
	payload := []byte{'g', 'i', 't', 0x03, 'l', 'o', 'g'}
	if err := s.SendBracketedPaste(payload); err != nil {
		t.Fatalf("paste with control byte: %v", err)
	}
	s.WaitFor("fuller-visible", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("fuller")
		})

	cancelAndExit(t, s)
}
