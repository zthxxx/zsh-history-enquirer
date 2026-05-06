// Package keys parses raw byte streams from a TTY into discrete
// events: rune insertions, named keys, bracketed-paste payloads,
// terminal resizes.
//
// The parser exists because:
//   - bubbletea's built-in key parser splits bracketed-paste payloads
//     across keystrokes, and we need the payload as one event;
//   - we want to swallow DSR responses (`\e[<row>;<col>R`) silently
//     since the cursor probe already consumed them once;
//   - testing the state machine end-to-end is easier with a small
//     home-grown parser than with bubbletea's internals.
package keys

// Event is the discriminated union produced by Reader.
type Event interface{ event() }

// RuneEvent is a printable rune that should append to input.
type RuneEvent struct {
	R rune
}

// KeyEvent is a named non-character key (Enter, arrows, …).
type KeyEvent struct {
	Key Key
}

// PasteEvent contains the full payload of a bracketed paste.
type PasteEvent struct {
	Payload string
}

// ResizeEvent fires when the terminal reports a new geometry.
type ResizeEvent struct {
	Rows, Cols int
}

func (RuneEvent) event()   {}
func (KeyEvent) event()    {}
func (PasteEvent) event()  {}
func (ResizeEvent) event() {}

// Key enumerates non-character keys we care about.
type Key int

// Each named key value is stable; do not reorder or insert.
const (
	KeyUnknown Key = iota
	KeyEnter
	KeyEsc
	KeyBackspace
	KeyTab
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyCtrlA
	KeyCtrlB
	KeyCtrlC
	KeyCtrlD
	KeyCtrlE
	KeyCtrlF
	KeyCtrlH
	KeyCtrlK
	KeyCtrlL
	KeyCtrlN
	KeyCtrlP
	KeyCtrlR
	KeyCtrlU
	KeyCtrlW
	KeyCtrlY
)
