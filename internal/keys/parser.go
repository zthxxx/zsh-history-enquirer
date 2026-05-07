package keys

import (
	"strings"
	"unicode/utf8"
)

// Parser is a stateful byte-stream → Event decoder. Designed so that
// any byte boundary is safe — the caller may feed bytes one at a time
// or in arbitrary chunks and still get exactly the same event sequence.
//
// The parser is single-threaded; one Parser per Reader.
type Parser struct {
	state state
	buf   []byte // bytes accumulated for the current sequence
	paste []byte // bytes accumulated inside a paste payload
}

type state int

const (
	stateNormal state = iota
	stateEsc          // saw \e, awaiting next byte
	stateCSI          // saw \e[, accumulating params
	stateSS3          // saw \eO (Single Shift 3); awaiting key code
	statePaste        // inside bracketed paste, awaiting \e[201~
)

// PasteStart and PasteEnd are the bracketed-paste markers we consume.
const (
	pasteStart = "\x1b[200~"
	pasteEnd   = "\x1b[201~"
)

// NewParser returns a parser ready to consume bytes.
func NewParser() *Parser {
	return &Parser{}
}

// Feed appends bytes and returns any events that became complete.
// The slice ownership is transferred; the parser may retain it across
// calls.
func (p *Parser) Feed(in []byte) []Event {
	out := []Event{}
	for _, b := range in {
		switch p.state {
		case stateNormal:
			out = p.feedNormal(b, out)
		case stateEsc:
			out = p.feedEsc(b, out)
		case stateCSI:
			out = p.feedCSI(b, out)
		case stateSS3:
			out = p.feedSS3(b, out)
		case statePaste:
			out = p.feedPaste(b, out)
		}
	}
	return out
}

// FlushEsc treats a pending solitary ESC as Esc keypress. Called when
// the caller knows no more bytes are coming for some milliseconds —
// otherwise we would never emit the standalone Esc event.
//
// A pending SS3 prelude (`\eO` with no follow-up byte) flushes as
// Esc + 'O' rune so the picker doesn't sit forever holding the key
// state if the terminal aborts mid-sequence. Same pattern as Esc.
func (p *Parser) FlushEsc() []Event {
	switch p.state {
	case stateEsc:
		p.state = stateNormal
		return []Event{KeyEvent{Key: KeyEsc}}
	case stateSS3:
		p.state = stateNormal
		return []Event{KeyEvent{Key: KeyEsc}, RuneEvent{R: 'O'}}
	default:
		return nil
	}
}

func (p *Parser) feedNormal(b byte, out []Event) []Event {
	switch {
	case b == 0x1b:
		p.state = stateEsc
		p.buf = p.buf[:0]
		return out
	case b == 0x0d || b == 0x0a:
		return append(out, KeyEvent{Key: KeyEnter})
	case b == 0x7f || b == 0x08:
		return append(out, KeyEvent{Key: KeyBackspace})
	case b == 0x09:
		return append(out, KeyEvent{Key: KeyTab})
	case b >= 0x01 && b <= 0x1a:
		// Ctrl-A..Ctrl-Z (excluding 0x00, the actual Ctrl mappings
		// users care about — Ctrl-C, Ctrl-U, Ctrl-W, Ctrl-R, etc.).
		return append(out, KeyEvent{Key: ctrlByte(b)})
	case b == 0x00:
		// NUL — drop silently.
		return out
	default:
		// Decode UTF-8 starting at this byte. Buffer if incomplete.
		p.buf = append(p.buf, b)
		r, size := utf8.DecodeRune(p.buf)
		if r == utf8.RuneError && size <= 1 {
			// Either invalid byte sequence or partial rune; keep
			// accumulating. Cap the buffer to four bytes (max UTF-8
			// rune width) so a stream of garbage cannot grow forever.
			if len(p.buf) >= utf8.UTFMax {
				p.buf = p.buf[:0]
			}
			return out
		}
		p.buf = p.buf[:0]
		return append(out, RuneEvent{R: r})
	}
}

func (p *Parser) feedEsc(b byte, out []Event) []Event {
	switch b {
	case '[':
		p.state = stateCSI
		p.buf = p.buf[:0]
		return out
	case 'O':
		// SS3 (Single Shift 3) prelude — terminals running in
		// "application keypad mode" (xterm DECCKM, some VT-series
		// emulators, embedded firmware terminals) send `\eOA` for
		// arrow up, `\eOB` for down, etc., instead of the CSI form
		// `\e[A` / `\e[B` we already handle. Without this branch the
		// fallback would surface as "Esc + 'O' rune + arrow letter
		// rune" — Esc would CANCEL the picker on every arrow press.
		p.state = stateSS3
		return out
	case 0x1b:
		// ESC ESC: emit one Esc and treat the second as start of a
		// new sequence.
		p.state = stateEsc
		return append(out, KeyEvent{Key: KeyEsc})
	default:
		// ESC followed by an unrelated byte: emit Esc, then process
		// the byte as if it arrived in normal state.
		p.state = stateNormal
		out = append(out, KeyEvent{Key: KeyEsc})
		return p.feedNormal(b, out)
	}
}

// feedSS3 maps the byte after `\eO` to the equivalent CSI key. Only
// arrow keys + Home/End are observed in real-world SS3 streams; any
// other byte falls back to "Esc + 'O' + byte-as-rune" for safety,
// matching what the parser would have done before SS3 was wired in.
func (p *Parser) feedSS3(b byte, out []Event) []Event {
	p.state = stateNormal
	switch b {
	case 'A':
		return append(out, KeyEvent{Key: KeyUp})
	case 'B':
		return append(out, KeyEvent{Key: KeyDown})
	case 'C':
		return append(out, KeyEvent{Key: KeyRight})
	case 'D':
		return append(out, KeyEvent{Key: KeyLeft})
	case 'H':
		return append(out, KeyEvent{Key: KeyHome})
	case 'F':
		return append(out, KeyEvent{Key: KeyEnd})
	default:
		// Unrecognized SS3 — best-effort fallback so we don't swallow
		// the bytes silently. Emits Esc + 'O' + byte. The picker will
		// cancel on Esc; that's the same behavior as before SS3
		// support was wired in, so it is at most a no-op regression
		// for sequences we never claimed to handle.
		out = append(out, KeyEvent{Key: KeyEsc}, RuneEvent{R: 'O'})
		return p.feedNormal(b, out)
	}
}

func (p *Parser) feedCSI(b byte, out []Event) []Event {
	p.buf = append(p.buf, b)
	// CSI sequences end with a final byte in 0x40..0x7e.
	if b < 0x40 || b > 0x7e {
		return out
	}

	seq := string(p.buf)
	p.buf = p.buf[:0]
	p.state = stateNormal

	// Strip the optional `1;<modifier>` prefix so modifier-key forms
	// like Shift+Up (`\e[1;2A`), Alt+Up (`\e[1;3A`), Ctrl+Up
	// (`\e[1;5A`), etc. resolve to the same Key as the plain
	// counterpart. The picker has no per-modifier behavior anyway —
	// ignoring the modifier is friendlier than swallowing the press.
	bare := stripCSIModifier(seq)
	switch {
	case bare == "A":
		return append(out, KeyEvent{Key: KeyUp})
	case bare == "B":
		return append(out, KeyEvent{Key: KeyDown})
	case bare == "C":
		return append(out, KeyEvent{Key: KeyRight})
	case bare == "D":
		return append(out, KeyEvent{Key: KeyLeft})
	case bare == "H" || bare == "1~":
		return append(out, KeyEvent{Key: KeyHome})
	case bare == "F" || bare == "4~":
		return append(out, KeyEvent{Key: KeyEnd})
	case bare == "5~":
		return append(out, KeyEvent{Key: KeyPageUp})
	case bare == "6~":
		return append(out, KeyEvent{Key: KeyPageDown})
	case bare == "3~":
		return append(out, KeyEvent{Key: KeyDelete})
	case seq == "200~":
		// Bracketed paste start. Subsequent bytes accumulate until
		// we see the matching end marker.
		p.state = statePaste
		p.paste = p.paste[:0]
		return out
	case strings.HasSuffix(seq, "R"):
		// DSR cursor-position response. Swallowed silently — the
		// cursor probe consumed its own response, anything that
		// arrives here is leftover or unsolicited.
		return out
	default:
		// Unrecognized CSI; ignore quietly so unknown sequences do
		// not corrupt input.
		return out
	}
}

func (p *Parser) feedPaste(b byte, out []Event) []Event {
	p.paste = append(p.paste, b)
	// Cheap incremental check for the end marker.
	if !endsWith(p.paste, []byte(pasteEnd)) {
		return out
	}
	payload := p.paste[:len(p.paste)-len(pasteEnd)]
	ev := PasteEvent{Payload: string(payload)}
	p.paste = p.paste[:0]
	p.state = stateNormal
	return append(out, ev)
}

// stripCSIModifier reduces "1;<mod><letter>" CSI sequences (the
// xterm-style modifier-key encoding) to "<letter>" so the dispatch
// table matches both the plain and modified forms with one entry.
//
// Examples:
//
//	"A"      → "A"     (plain Up)
//	"1;2A"   → "A"     (Shift+Up)
//	"1;5A"   → "A"     (Ctrl+Up)
//	"1;3A"   → "A"     (Alt+Up)
//	"1;6A"   → "A"     (Ctrl+Shift+Up)
//	"5~"     → "5~"    (PgUp, no modifier)
//	"5;5~"   → "5~"    (Ctrl+PgUp)
//	"foo"    → "foo"   (unrecognized; passthrough)
//
// Only sequences with an explicit `1;<n>` prefix and a single-letter
// terminator OR `<row>;<n>~` form are normalized. Anything else
// passes through so unrelated CSI sequences (DSR, OSC, …) keep
// their unique form for the dispatch table.
func stripCSIModifier(seq string) string {
	if seq == "" {
		return seq
	}
	last := seq[len(seq)-1]
	// Form 1: "1;<digits><letter>" → "<letter>".
	if last >= '@' && last <= '~' && strings.HasPrefix(seq, "1;") {
		body := seq[2 : len(seq)-1]
		if isAllDigits(body) {
			return string(last)
		}
	}
	// Form 2: "<row>;<digits>~" → "<row>~". Used for PgUp/PgDn,
	// Home, End, Delete in modified form.
	if last == '~' {
		semi := strings.IndexByte(seq, ';')
		if semi > 0 {
			row := seq[:semi]
			rest := seq[semi+1 : len(seq)-1]
			if isAllDigits(row) && isAllDigits(rest) {
				return row + "~"
			}
		}
	}
	return seq
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return false
		}
	}
	return true
}

func endsWith(buf, suffix []byte) bool {
	if len(buf) < len(suffix) {
		return false
	}
	for i := range suffix {
		if buf[len(buf)-len(suffix)+i] != suffix[i] {
			return false
		}
	}
	return true
}

func ctrlByte(b byte) Key {
	switch b {
	case 0x01:
		return KeyCtrlA
	case 0x02:
		return KeyCtrlB
	case 0x03:
		return KeyCtrlC
	case 0x04:
		return KeyCtrlD
	case 0x05:
		return KeyCtrlE
	case 0x06:
		return KeyCtrlF
	case 0x0b:
		return KeyCtrlK
	case 0x0c:
		return KeyCtrlL
	case 0x0e:
		return KeyCtrlN
	case 0x10:
		return KeyCtrlP
	case 0x12:
		return KeyCtrlR
	case 0x15:
		return KeyCtrlU
	case 0x17:
		return KeyCtrlW
	case 0x19:
		return KeyCtrlY
	default:
		return KeyUnknown
	}
}
