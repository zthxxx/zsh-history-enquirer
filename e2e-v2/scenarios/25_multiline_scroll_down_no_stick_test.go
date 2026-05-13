package scenarios

import (
	"strings"
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestMultilineScrollDownNoStick ports e2e/scenarios/25-multiline-scroll-down-no-stick.exp.
// 10×80 terminal. Filter shows 6 single-line entries + 1 multi-line.
// Press ↓ 6 times: the 6th MUST advance focus onto the multi-line
// even though it consumes 3 visual rows and forces heightLimit
// eviction. Submit+run must emit the multi-line stdout markers,
// not the single-line ones.
func TestMultilineScrollDownNoStick(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{
		Rows:           10,
		Cols:           80,
		HistoryFixture: "25-multiline-stick",
	})

	s.Settle()
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("send ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	if err := s.Type("scrolltok"); err != nil {
		t.Fatalf("type filter: %v", err)
	}
	s.WaitQuiescent(200*time.Millisecond, 1*time.Second)

	// Walk Down 6 times. The 6th must land on the multi-line.
	for i := 0; i < 6; i++ {
		if err := s.SendKey(harness.KeyDown); err != nil {
			t.Fatalf("Down %d: %v", i, err)
		}
		s.WaitQuiescent(80*time.Millisecond, 500*time.Millisecond)
	}

	// Submit + run.
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("submit: %v", err)
	}
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)
	if err := s.SendKey(harness.KeyEnter); err != nil {
		t.Fatalf("run: %v", err)
	}
	// Multi-line markers must appear — if focus had stuck on a
	// single-N entry, "scrolltok-mline-*" would be absent.
	s.WaitFor("mline-markers", 5*time.Second,
		func(scr harness.Screen) bool {
			dump := scr.Dump()
			return strings.Contains(dump, "scrolltok-mline-alpha") &&
				strings.Contains(dump, "scrolltok-mline-beta") &&
				strings.Contains(dump, "scrolltok-mline-gamma")
		})

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
