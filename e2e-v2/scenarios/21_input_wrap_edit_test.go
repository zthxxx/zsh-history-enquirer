package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestInputWrapEdit ports e2e/scenarios/21-input-wrap-edit.exp.
// On a 40-col terminal, prefill the filter with 50 x's so the input
// row wraps. Backspace 50 times to clear. The renderer must redraw
// cleanly without leaving stale wrap rows; the seed entry "hello"
// must appear once the filter is empty.
func TestInputWrapEdit(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{
		Rows:           24,
		Cols:           40,
		HistoryFixture: "21-input-wrap",
		PreFilter:      strRepeat("x", 50),
	})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	// Initial render: filter of 50 x's, "(no matches)" hint visible.
	s.WaitFor("initial-no-matches", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("(no matches)")
		})

	// Backspace 50 times to clear the input one char at a time.
	for i := 0; i < 50; i++ {
		if err := s.SendKey(harness.KeyBackspace); err != nil {
			t.Fatalf("backspace %d: %v", i, err)
		}
	}

	// With empty filter, "hello" matches and must be rendered.
	s.WaitFor("hello-revealed", 3*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("hello")
		})

	cancelAndExit(t, s)
}

func strRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
