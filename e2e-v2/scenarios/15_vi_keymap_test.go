package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e-v2/harness"
)

// TestViKeymap ports e2e/scenarios/15-vi-keymap.exp.
// Switch to vi mode, then ^R from both viins and vicmd keymaps.
// The plugin's bindkey -M emacs/viins/vicmd ensures the picker
// reaches us in every keymap.
func TestViKeymap(t *testing.T) {
	s := harness.NewSession(t, harness.Opts{})

	s.Settle()

	// Switch to vi mode. zsh is now in viins.
	if err := s.Type("bindkey -v\r"); err != nil {
		t.Fatalf("bindkey -v: %v", err)
	}
	s.WaitQuiescent(200*time.Millisecond, 1*time.Second)

	// ^R from viins.
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("first ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	s.WaitFor("first-open-shows-echo-ok", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	// Cancel back to the prompt.
	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("first Esc: %v", err)
	}
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)
	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("Ctrl-U: %v", err)
	}
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)

	// One more Esc to enter vicmd, then ^R from vicmd.
	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("Esc → vicmd: %v", err)
	}
	s.WaitQuiescent(120*time.Millisecond, 500*time.Millisecond)
	if err := s.SendKey(harness.KeyCtrlR); err != nil {
		t.Fatalf("second ^R: %v", err)
	}
	s.WaitQuiescent(400*time.Millisecond, 5*time.Second)

	s.WaitFor("second-open-shows-echo-ok", 2*time.Second,
		func(scr harness.Screen) bool {
			return scr.Contains("echo ok")
		})

	// In vicmd, typing "i" enters insert mode; "exit" then "\r" runs.
	// The original .exp uses `iexit\r` here.
	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("Esc final: %v", err)
	}
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)
	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("Ctrl-U final: %v", err)
	}
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)
	if err := s.Type("iexit\r"); err != nil {
		t.Fatalf("iexit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
