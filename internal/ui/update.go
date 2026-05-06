package ui

import (
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
	m.Input += string(r)
	m.Cursor = len(m.Input)
	m.recomputeFilter()
}

func (m *Model) appendString(s string) {
	m.Input += s
	m.Cursor = len(m.Input)
	m.recomputeFilter()
}

//nolint:gocyclo // straightforward dispatch; each branch is one line
func (m *Model) applyKey(k keys.Key) (terminate bool) {
	switch k {
	case keys.KeyBackspace:
		if m.Input != "" {
			m.Input = m.Input[:len(m.Input)-1]
			m.Cursor = len(m.Input)
			m.recomputeFilter()
		}
		return false
	case keys.KeyCtrlU:
		m.Input = ""
		m.Cursor = 0
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
	case keys.KeyUp:
		m.moveUp()
		return false
	case keys.KeyDown:
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
