package ui

import (
	"strings"

	"github.com/zthxxx/zsh-history-enquirer/internal/ansi"
)

// Frame is the byte payload to write to the TTY for one render pass.
// Pre erases the previous frame's body, Body draws the new content,
// Post returns the cursor to the editing position. They are kept
// separate so tests can examine each stage independently.
type Frame struct {
	Pre  string
	Body string
	Post string

	// Size is the number of rows the new Body occupies (counting only
	// the choice rows, not the input row). Stored on the model after
	// each render so the next frame's Pre can erase exactly that many
	// rows.
	Size int

	// Limit is the number of choices that ended up visible. The caller
	// stores this on the model so subsequent up/down navigation can
	// use it.
	Limit int
}

// RenderOptions lets the caller pass the previous-frame size into the
// renderer. We do not store it on the Model because the Model is
// otherwise a pure data struct; mixing rendering bookkeeping into it
// would leak abstraction.
type RenderOptions struct {
	PrevSize int
}

// pointerSelected is the glyph drawn before the focused choice.
const pointerSelected = "› "

// pointerUnselected is the glyph drawn before non-focused choices.
const pointerUnselected = "  "

// Render produces a Frame describing how to draw the model on the TTY.
// The Frame is purely a byte-string description; it does not touch
// the terminal directly.
func (m *Model) Render(opts RenderOptions) Frame {
	body, size, limit := m.renderBody()
	pre := m.renderPre(opts.PrevSize)
	post := m.renderPost(size)
	m.Limit = limit
	return Frame{Pre: pre, Body: body, Post: post, Size: size, Limit: limit}
}

func (m *Model) renderBody() (string, int, int) { //nolint:gocritic // unnamed result is clearer here
	// Step 1: write the input row at the captured prompt column.
	var body strings.Builder
	body.WriteString(ansi.CursorToCol(m.InitCol))
	body.WriteString(ansi.EraseLineEnd)
	body.WriteString(m.Input)

	// Step 2: walk the filtered list, accumulating row counts until
	// either MaxLimit or terminal-3 is reached. This is the dynamic
	// limit logic from spec/40.
	heightLimit := m.Height - 3
	if heightLimit < 1 {
		heightLimit = 1
	}

	rows := 0
	limit := 0
	for _, choice := range m.Filter {
		choiceRows := WrappedRowCount(choice, m.Width)
		if rows+choiceRows > heightLimit {
			break
		}
		rows += choiceRows
		limit++
		if limit >= m.MaxLimit {
			break
		}
	}

	if limit == 0 && len(m.Filter) > 0 {
		// At minimum one row should be drawn even when the list does
		// not fit the terminal — we cannot help the user otherwise.
		limit = 1
		rows = 1
	}

	// Clamp the index in case a previous PageDown left it past the
	// visible window.
	if m.Idx >= limit {
		m.Idx = limit - 1
	}
	if m.Idx < 0 {
		m.Idx = 0
	}

	for i := range limit {
		body.WriteString("\r\n")
		if i == m.Idx {
			body.WriteString(pointerSelected)
		} else {
			body.WriteString(pointerUnselected)
		}
		// Multi-line entries are written verbatim; the wrap math
		// above already accounted for newlines and width.
		// We translate "\n" to "\r\n" so terminals in raw mode advance
		// to column 0 on each new logical line.
		body.WriteString(strings.ReplaceAll(choiceLine(m.Filter[i]), "\n", "\r\n"))
	}

	if limit == 0 {
		body.WriteString("\r\n")
		body.WriteString("  (no matches)")
		rows = 1
	}

	return body.String(), rows, limit
}

func choiceLine(s string) string {
	return s
}

// renderPre erases the previous frame's body so the next draw lands
// on a clean slate. We assume the cursor is currently on the input
// row (where Render leaves it after every frame).
func (m *Model) renderPre(prevSize int) string {
	if prevSize <= 0 {
		// First frame: nothing to erase below, but we still want to
		// wipe the input row from initCol onwards.
		return ansi.CursorToCol(m.InitCol) + ansi.EraseLineEnd
	}
	var b strings.Builder
	// Walk down the prevSize body rows, erasing each.
	for range prevSize {
		b.WriteString("\r\n")
		b.WriteString(ansi.EraseLine)
	}
	// Walk back up to the input row and erase from initCol onward.
	b.WriteString(ansi.CursorPrevLine(prevSize))
	b.WriteString(ansi.CursorToCol(m.InitCol))
	b.WriteString(ansi.EraseLineEnd)
	return b.String()
}

// renderPost returns the cursor to the input row at the user's caret.
func (m *Model) renderPost(currentSize int) string {
	var b strings.Builder
	// Move from after the body back up to the input row.
	if currentSize > 0 {
		b.WriteString(ansi.CursorPrevLine(currentSize))
	}
	b.WriteString(ansi.CursorToCol(m.InitCol + m.Cursor))
	return b.String()
}
