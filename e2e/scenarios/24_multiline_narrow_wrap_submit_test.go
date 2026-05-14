package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// TestMultilineNarrowWrapSubmit ports e2e/scenarios/24-multiline-narrow-wrap-submit.exp.
// Compound boundary case: multi-line entry where each line wraps on a
// 40-col terminal. Submit + run; all three line markers must appear
// as command output — BUFFER preserved every byte across the pipeline.
func TestMultilineNarrowWrapSubmit(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{
		Rows:           24,
		Cols:           40,
		HistoryFixture: "24-multiline-narrow-wrap",
	})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("alpha"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	// The second line of the multi-line entry must render — proves
	// multi-line + wrap render survived without corruption.
	s.WaitFor("beta-rendered", 3*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("beta")
		})

	// Submit + run.
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("submit: %v", err)
	}
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("run: %v", err)
	}
	// All three markers must appear (each at least twice — once in
	// the echoed command, once as output).
	s.WaitFor("three-markers", 5*time.Second,
		func(scr harness.Screen) bool {
			dump := scr.Dump()
			return strings.Count(dump, "alpha") >= 2 &&
				strings.Count(dump, "beta") >= 2 &&
				strings.Count(dump, "gamma") >= 2
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
