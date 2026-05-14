package harness

// Key is a typed name for a raw input sequence the harness can send.
//
// The byte sequences mirror exactly what the legacy expect harness
// sends (see SCENARIOS-MANIFEST.md "Key catalog" for the inventory
// and the source .exp files for the raw byte literals).
type Key int

const (
	KeyInvalid Key = iota
	KeyEnter
	KeyEsc
	KeyBackspace
	KeyAltBackspace

	KeyCtrlR
	KeyCtrlU
	KeyCtrlW

	KeyUp
	KeyDown
	KeyLeft
	KeyRight

	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown

	KeyF1
	KeyF2
	KeyF3
	KeyF4
)

// Bytes returns the raw byte sequence the harness writes to the pty
// master for the given key. Unknown keys return an empty slice; the
// caller is responsible for using a defined constant.
func (k Key) Bytes() []byte {
	switch k {
	case KeyEnter:
		return []byte{'\r'}
	case KeyEsc:
		return []byte{0x1b}
	case KeyBackspace:
		return []byte{0x7f}
	case KeyAltBackspace:
		return []byte{0x1b, 0x7f}

	case KeyCtrlR:
		return []byte{0x12}
	case KeyCtrlU:
		return []byte{0x15}
	case KeyCtrlW:
		return []byte{0x17}

	case KeyUp:
		return []byte{0x1b, '[', 'A'}
	case KeyDown:
		return []byte{0x1b, '[', 'B'}
	case KeyRight:
		return []byte{0x1b, '[', 'C'}
	case KeyLeft:
		return []byte{0x1b, '[', 'D'}

	case KeyHome:
		return []byte{0x1b, '[', 'H'}
	case KeyEnd:
		return []byte{0x1b, '[', 'F'}
	case KeyPageUp:
		return []byte{0x1b, '[', '5', '~'}
	case KeyPageDown:
		return []byte{0x1b, '[', '6', '~'}

	case KeyF1:
		return []byte{0x1b, 'O', 'P'}
	case KeyF2:
		return []byte{0x1b, 'O', 'Q'}
	case KeyF3:
		return []byte{0x1b, 'O', 'R'}
	case KeyF4:
		return []byte{0x1b, 'O', 'S'}
	}
	return nil
}

// Bracketed paste sentinels per xterm DECPM 2004. The picker enables
// bracketed-paste mode at startup (ansi.SetModeBracketedPaste) and
// disables it on exit; the harness must wrap pasted payloads in these
// sentinels for the picker's parser FSM to deliver them as a single
// EventPaste rather than a stream of EventKey.
var (
	BracketedPasteOpen  = []byte{0x1b, '[', '2', '0', '0', '~'}
	BracketedPasteClose = []byte{0x1b, '[', '2', '0', '1', '~'}
)
