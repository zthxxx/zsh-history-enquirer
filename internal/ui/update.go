package ui

import (
	"unicode/utf8"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
)

// Update applies an event to the model. It mutates the receiver in
// place because every callsite owns a *Model; returning a copy each
// frame would just allocate without making the function any easier
// to reason about.
//
// Returns true if the event terminates the session (submit/cancel).
func (m *Model) Update(ev keys.Event) (terminate bool) {
	switch e := ev.(type) {
	case keys.RuneEvent:
		m.appendRune(e.R)
	case keys.PasteEvent:
		m.appendString(e.Payload)
	case keys.KeyEvent:
		return m.applyKey(e.Key)
	case keys.ResizeEvent:
		m.Height = e.Rows
		m.Width = e.Cols
	}
	return false
}

func (m *Model) appendRune(r rune) {
	// Translate control runes to spaces — the input row is rendered
	// verbatim, so a stray \r would carriage-return into the prompt
	// prefix, a \n would push the picker down a row, and a \t would
	// jump to the next tabstop. None are useful in a search filter.
	m.Input += string(sanitizeInputRune(r))
	m.Cursor = CellWidth(m.Input)
	m.recomputeFilter()
}

func (m *Model) appendString(s string) {
	// Same sanitization as appendRune, applied to the whole paste
	// payload. Bracketed paste of multi-line text would otherwise
	// scribble across the terminal.
	m.Input += sanitizeInputString(s)
	m.Cursor = CellWidth(m.Input)
	m.recomputeFilter()
}

// sanitizeInputRune collapses any C0 control byte (0x00-0x1f) or
// DEL (0x7f) to a space, leaving every other rune unchanged. The
// input row is rendered verbatim into the picker frame, so any
// pass-through control byte would corrupt the layout: \n / \t mean
// nothing in a single-line filter, \r would carriage-return into
// the prompt prefix, BEL would beep on every keystroke, and most
// dangerously a bare \x1b would let a pasted ANSI escape (e.g. a
// clipboard with `\x1b[2J`) reposition the cursor or clear the
// screen mid-render.
//
// Mapping to space (rather than dropping or caret-quoting) matches
// the existing \n / \r / \t behaviour: pasted multi-line shell
// commands collapse into space-separated search tokens, which is
// the most useful interpretation for a filter box.
func sanitizeInputRune(r rune) rune {
	if r < 0x20 || r == 0x7f {
		return ' '
	}
	return r
}

// sanitizeInputString applies sanitizeInputRune across an entire
// string. Used by paste handling.
func sanitizeInputString(s string) string {
	if !containsControlByte(s) {
		return s
	}
	out := make([]rune, 0, len(s))
	for _, r := range s {
		out = append(out, sanitizeInputRune(r))
	}
	return string(out)
}

// containsControlByte reports whether s holds any C0 / DEL byte —
// the trigger condition for sanitizeInputString to do real work.
// Plain UTF-8 multi-byte runes (CJK, emoji) have no bytes < 0x20,
// so the check is byte-level rather than rune-level for speed.
func containsControlByte(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c < 0x20 || c == 0x7f {
			return true
		}
	}
	return false
}

//nolint:gocyclo // straightforward dispatch; each branch is one line
func (m *Model) applyKey(k keys.Key) (terminate bool) {
	switch k {
	case keys.KeyBackspace:
		if m.Input != "" {
			// Delete one rune, not one byte. For ASCII the two are
			// identical; for multi-byte UTF-8 (CJK, emoji, accented
			// Latin) a byte-level slice would leave a trailing
			// continuation byte and corrupt the input into invalid
			// UTF-8. Using DecodeLastRuneInString keeps the buffer
			// valid even when the user types `你<bs>` or `🚀<bs>`.
			_, size := utf8.DecodeLastRuneInString(m.Input)
			m.Input = m.Input[:len(m.Input)-size]
			m.Cursor = CellWidth(m.Input)
			m.recomputeFilter()
		}
		return false
	case keys.KeyCtrlU:
		m.Input = ""
		m.Cursor = 0
		m.recomputeFilter()
		return false
	case keys.KeyCtrlW:
		// Delete the previous word — strip trailing whitespace then
		// the run of non-whitespace before it. Matches zsh's default
		// `backward-kill-word` and shell users' muscle memory.
		m.Input = deleteLastWord(m.Input)
		m.Cursor = CellWidth(m.Input)
		m.recomputeFilter()
		return false
	case keys.KeyEnter:
		m.Submitted = true
		m.Result = m.SubmitResult()
		return true
	case keys.KeyEsc, keys.KeyCtrlC:
		m.Canceled = true
		// Route through SubmitResult so the cancel path is exercised
		// by the same function as submit. Avoids a silent dual-source
		// of the "what is m.Result on cancel" semantics.
		m.Result = m.SubmitResult()
		return true
	case keys.KeyUp, keys.KeyCtrlP:
		// Ctrl-P is zsh's emacs-keymap "previous history" — same
		// motion as ↑ in this picker (move selection up by one).
		m.moveUp()
		return false
	case keys.KeyDown, keys.KeyCtrlN:
		// Ctrl-N is zsh's emacs-keymap "next history" — same motion
		// as ↓ in this picker.
		m.moveDown()
		return false
	case keys.KeyPageUp:
		m.rotateUp(max1(m.Limit))
		return false
	case keys.KeyPageDown:
		m.rotateDown(max1(m.Limit))
		return false
	case keys.KeyHome:
		m.recomputeFilter()
		return false
	case keys.KeyEnd:
		m.scrollToEnd()
		return false
	default:
		return false
	}
}

// deleteLastWord returns s with the trailing word removed. A "word"
// is a run of non-whitespace; the trailing whitespace separator is
// also stripped so successive ^W's eat one word at a time. Walks
// rune-by-rune so multi-byte characters (CJK, emoji, accented
// Latin) are deleted atomically rather than leaving partial bytes.
func deleteLastWord(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	// Strip trailing whitespace.
	end := len(runes)
	for end > 0 && isSpace(runes[end-1]) {
		end--
	}
	// Strip trailing non-whitespace (the word itself).
	for end > 0 && !isSpace(runes[end-1]) {
		end--
	}
	return string(runes[:end])
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func (m *Model) moveUp() {
	if len(m.Filter) == 0 {
		return
	}
	if m.Idx > 0 {
		m.Idx--
		return
	}
	// Already at the top of the visible window — rotate one in.
	m.rotateUp(1)
}

func (m *Model) moveDown() {
	if len(m.Filter) == 0 {
		return
	}
	if m.Limit <= 0 {
		// Layout has not run yet (e.g. before the first render);
		// just advance the index modulo the filter length.
		m.Idx = (m.Idx + 1) % len(m.Filter)
		return
	}
	if m.Idx+1 < min(m.Limit, len(m.Filter)) {
		m.Idx++
		return
	}
	if len(m.Filter) <= m.Limit {
		// Wrap around — there is nothing more to scroll into view.
		m.Idx = 0
		m.rotateDown(1)
		return
	}
	// At the bottom of the visible window with more entries below.
	m.rotateDown(1)
}

// scrollToEnd places the *last* filtered entry at the bottom of the
// visible window with focus on it.
//
// The legacy Node.js port (and earlier versions of this package)
// rotated by m.Limit and set Idx = m.Limit-1. That works when every
// entry is single-line, but breaks when multi-line entries reshuffle
// into the visible window after rotation: the renderer's recomputed
// dynamic limit shrinks and m.Idx gets clamped off the last match.
//
// We instead walk Filter from the back, accumulating wrapped row
// counts until heightLimit (or MaxLimit) is hit. That gives us the
// precise number of "tail" entries that fit in the visible window
// post-rotation. We then rotate by that count, putting the last
// match at position visibleCount-1, where the renderer's forward walk
// will land on it identically.
func (m *Model) scrollToEnd() {
	if len(m.Filter) == 0 {
		return
	}

	heightLimit := m.Height - 3
	if heightLimit < 1 {
		heightLimit = 1
	}

	visibleCount := 0
	rows := 0
	for i := len(m.Filter) - 1; i >= 0; i-- {
		choiceRows := WrappedRowCount(m.Filter[i], m.Width)
		if rows+choiceRows > heightLimit {
			break
		}
		rows += choiceRows
		visibleCount++
		if visibleCount >= m.MaxLimit {
			break
		}
	}
	if visibleCount == 0 {
		// Even an entry that overflows the terminal alone deserves
		// to be selected; the renderer will at least show the focus
		// on row 0.
		visibleCount = 1
	}

	m.rotateUp(visibleCount)
	m.Idx = visibleCount - 1
}
