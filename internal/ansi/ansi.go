// Package ansi assembles the small set of ANSI/VT escape sequences this
// project emits. Kept minimal on purpose — we do not need a full
// terminal abstraction, only the cursor moves and erases the legacy
// implementation already proved necessary.
//
// Design constraints:
//   - Every function returns a string so callers may concatenate freely
//     without worrying about partial writes.
//   - We use 1-indexed column/row arguments because the underlying CSI
//     sequences are 1-indexed; converting at the boundary keeps internal
//     code 0-indexed where appropriate.
package ansi

import (
	"fmt"
	"strings"
)

// CSI is the Control Sequence Introducer prefix.
const CSI = "\x1b["

// CursorTo moves the cursor to (row, col) using a CSI <row>;<col>H
// sequence. Both coordinates are 1-indexed in the protocol; pass the
// 1-indexed values directly.
func CursorTo(row, col int) string {
	return fmt.Sprintf("%s%d;%dH", CSI, row, col)
}

// CursorToCol moves the cursor to a specific column on the current row
// (CSI <col>G). 1-indexed.
func CursorToCol(col int) string {
	return fmt.Sprintf("%s%dG", CSI, col)
}

// CursorUp moves the cursor up n rows; n=0 is a no-op (returns "").
func CursorUp(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%dA", CSI, n)
}

// CursorDown moves the cursor down n rows; n=0 is a no-op.
func CursorDown(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%dB", CSI, n)
}

// CursorPrevLine moves the cursor to column 1 of the previous line
// repeated n times.
func CursorPrevLine(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%s%dF", CSI, n)
}

// EraseLine erases the entire current line.
const EraseLine = CSI + "2K"

// EraseLineEnd erases from cursor to end of line.
const EraseLineEnd = CSI + "0K"

// EraseDisplayDown erases from cursor to end of screen.
const EraseDisplayDown = CSI + "0J"

// HideCursor / ShowCursor — used at startup and shutdown so a redraw
// never reveals an intermediate position.
const (
	HideCursor = CSI + "?25l"
	ShowCursor = CSI + "?25h"
)

// BracketedPaste enable/disable pair.
const (
	BracketedPasteOn  = CSI + "?2004h"
	BracketedPasteOff = CSI + "?2004l"
)

// DSRCursor is the Device Status Report query that asks the terminal
// for its current cursor position. The terminal replies with
// CSI <row>;<col>R.
const DSRCursor = CSI + "6n"

// EraseLines clears n full lines starting from the cursor's current
// row, returning the cursor to the original position.
func EraseLines(n int) string {
	if n <= 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(CursorDown(n))
	for range n {
		b.WriteString(EraseLine)
		b.WriteString(CSI + "1F") // move to col 1 of previous line
	}
	return b.String()
}
