package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestMultilineSubmit ports e2e/scenarios/09-multiline-submit.exp.
// Select the multi-line `echo "alpha\nbeta\ngamma"` entry, submit,
// run it — verify all three output lines appear (BUFFER preserved
// embedded newlines through the full widget pipeline).
func TestMultilineSubmit(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("alpha"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	// All three lines of the multi-line entry must render.
	s.WaitFor("multi-line-rendered", 2*time.Second,
		func(scr harness.Screen) bool {
			dump := scr.Dump()
			return strings.Contains(dump, "beta") && strings.Contains(dump, "gamma")
		})

	// Submit — picker exits, BUFFER = multi-line echo.
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("submit: %v", err)
	}
	s.WaitFor("picker-gone", 3*time.Second,
		func(scr harness.Screen) bool { return !scr.Contains("›") })

	// Run — zsh executes the multi-line BUFFER, prints all three
	// lines as output.
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("run: %v", err)
	}
	s.WaitFor("multi-line-output", 3*time.Second,
		func(scr harness.Screen) bool {
			dump := scr.Dump()
			// Each marker should appear at least twice: once in the
			// echoed command line, once as the output line.
			return strings.Count(dump, "alpha") >= 2 &&
				strings.Count(dump, "beta") >= 2 &&
				strings.Count(dump, "gamma") >= 2
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("send exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
