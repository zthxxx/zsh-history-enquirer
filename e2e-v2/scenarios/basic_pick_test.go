package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestBasicPick is the Go-native port of e2e/scenarios/01-basic-pick.exp.
//
// Steps:
//
//  1. spawn zsh -il under a pty
//  2. wait for the picker-ready prompt to settle
//  3. press ^R
//  4. settle through the DSR-probe timeout + first render
//  5. assert "echo ok" is on screen (most-recent seed entry)
//  6. press Enter to submit
//  7. wait for the picker to fully exit (focus glyph `›` gone) AND
//     BUFFER captured
//  8. press Enter again to run the captured command
//  9. wait for the command output to appear
//
// 10. send "exit\r" and confirm zsh exits cleanly
func TestBasicPick(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if !s.Screen().Contains("%") {
		t.Fatalf("initial prompt not visible. screen:\n%s", s.Screen().Dump())
	}

	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	// First-render window absorbs the always-fires 250 ms DSR probe
	// fallback (internal/app/run.go:CursorTimeout, see
	// docs/plan/20-followups.md:38-44).
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if !s.Screen().Contains("echo ok") {
		t.Fatalf("expected 'echo ok' on screen after ^R; raw=%s\nscreen:\n%s",
			s.DebugRaw(), s.Screen().Dump())
	}

	// Submit — picker writes "echo ok" to stdout and exits; widget
	// sets BUFFER="echo ok".
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("send Enter: %v", err)
	}

	// Wait for the picker's focus glyph to disappear AND BUFFER to be
	// visible. Time-only quiescence would race with the picker's
	// raw-mode input drain (see REVIEW-FINDINGS F1 + the PoC fix log).
	s.WaitFor("picker-gone-buffer-set", 3*time.Second,
		func(scr harness.Screen) bool {
			return !scr.Contains("›") && scr.Contains("echo ok")
		})

	// Run the captured BUFFER — zsh prints "ok" + new prompt.
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("send Enter to run: %v", err)
	}
	// "ok" appears as command output, distinct from the BUFFER text
	// "echo ok" — count occurrences to disambiguate.
	s.WaitFor("ok-output", 2*time.Second,
		func(scr harness.Screen) bool {
			return strings.Count(scr.Dump(), "ok") >= 2
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("send exit: %v", err)
	}

	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
