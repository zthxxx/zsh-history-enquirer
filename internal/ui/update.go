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
		if len(m.Input) > 0 {
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
		m.Cancelled = true
		m.Result = m.Input
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

func (m *Model) scrollToEnd() {
	if len(m.Filter) == 0 {
		return
	}
	limit := m.Limit
	if limit <= 0 || limit > len(m.Filter) {
		limit = len(m.Filter)
	}
	// Rotate so the last filtered entry is at position limit-1 in the
	// visible window.
	m.rotateUp(limit)
	m.Idx = limit - 1
}
