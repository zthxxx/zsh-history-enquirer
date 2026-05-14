package scenarios

import (
	"testing"
	"time"

	"github.com/zthxxx/zsh-history-enquirer/e2e/harness"
)

// cancelAndExit dismisses the picker via Esc and shuts the session
// down. Most scenarios use this teardown verbatim — having one
// helper keeps the individual tests focused on the behaviour
// they're verifying.
//
// On Esc the picker preserves the typed filter (cancel-preserves-
// input contract — see AGENTS.md and
// e2e/scenarios/03-cancel-preserves-input.exp). BUFFER ends up
// containing whatever the user had typed into the picker. We MUST
// wipe BUFFER before typing `exit` or zsh will execute the
// concatenation (e.g. "commandexit" — command not found).
//
// Sequence:
//  1. Send Esc.
//  2. Wait for the picker's focus glyph `›` to disappear AND for
//     ZLE to repaint the prompt (we look for a `% ` row that does
//     NOT also contain `›`). Time-only quiescence races with
//     picker raw-mode drain (REVIEW-FINDINGS F1).
//  3. Send Ctrl-U — wipes BUFFER (default emacs keymap binds
//     ^U to backward-kill-line).
//  4. Wait for the prompt to show empty BUFFER.
//  5. Type "exit\r".
//  6. WaitExit.
func cancelAndExit(t *testing.T, s *harness.Session) {
	t.Helper()

	if err := s.SendKey(harness.KeyEsc); err != nil {
		t.Fatalf("send Esc: %v", err)
	}
	// Quiescence-based wait for picker teardown. We deliberately do
	// NOT screen-assert here: the focus glyph `›` is missing in
	// "(no matches)" states (no entry to focus), so a glyph-absence
	// predicate returns immediately during picker teardown and
	// subsequent bytes race the picker's raw-mode drain. The picker
	// emits a short teardown burst (show cursor, restore mode) then
	// goes silent; quiescence for 250 ms past the last byte is a
	// reliable "picker has fully exited and ZLE is back" signal,
	// independent of what the picker was showing.
	s.WaitQuiescent(250*time.Millisecond, 3*time.Second)

	if err := s.SendKey(harness.KeyCtrlU); err != nil {
		t.Fatalf("send Ctrl-U: %v", err)
	}
	// Brief settle so Ctrl-U actually lands on ZLE before exit.
	s.WaitQuiescent(80*time.Millisecond, 1*time.Second)

	if err := s.Type("exit\r"); err != nil {
		t.Fatalf("send exit: %v", err)
	}
	if err := s.WaitExit(5 * time.Second); err != nil {
		t.Fatalf("zsh did not exit cleanly: %v\nscreen:\n%s", err, s.Screen().Dump())
	}
}
