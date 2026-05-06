package keys

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func feedAll(p *Parser, b string) []Event {
	return p.Feed([]byte(b))
}

func TestParser_PrintableRunes(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "hello")
	require.Equal(t, []Event{
		RuneEvent{R: 'h'},
		RuneEvent{R: 'e'},
		RuneEvent{R: 'l'},
		RuneEvent{R: 'l'},
		RuneEvent{R: 'o'},
	}, got)
}

func TestParser_EnterBackspaceTab(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\r\x7f\t")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyEnter},
		KeyEvent{Key: KeyBackspace},
		KeyEvent{Key: KeyTab},
	}, got)
}

func TestParser_CtrlBytes(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x03\x15\x12")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyCtrlC},
		KeyEvent{Key: KeyCtrlU},
		KeyEvent{Key: KeyCtrlR},
	}, got)
}

func TestParser_ArrowKeys(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[A\x1b[B\x1b[C\x1b[D")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyUp},
		KeyEvent{Key: KeyDown},
		KeyEvent{Key: KeyRight},
		KeyEvent{Key: KeyLeft},
	}, got)
}

func TestParser_HomeEnd(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[H\x1b[F\x1b[1~\x1b[4~")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyHome},
		KeyEvent{Key: KeyEnd},
		KeyEvent{Key: KeyHome},
		KeyEvent{Key: KeyEnd},
	}, got)
}

func TestParser_PageUpDown(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[5~\x1b[6~")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyPageUp},
		KeyEvent{Key: KeyPageDown},
	}, got)
}

func TestParser_BracketedPaste_Simple(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[200~hello\x1b[201~")
	require.Equal(t, []Event{
		PasteEvent{Payload: "hello"},
	}, got)
}

func TestParser_BracketedPaste_SplitAcrossReads(t *testing.T) {
	t.Parallel()

	p := NewParser()
	require.Empty(t, feedAll(p, "\x1b[200"))
	require.Empty(t, feedAll(p, "~hel"))
	require.Empty(t, feedAll(p, "lo\x1b"))
	require.Empty(t, feedAll(p, "[201"))
	got := feedAll(p, "~")
	require.Equal(t, []Event{PasteEvent{Payload: "hello"}}, got)
}

func TestParser_BracketedPaste_PreservesControlBytes(t *testing.T) {
	t.Parallel()

	// A paste payload containing 0x03 must NOT trigger CtrlC.
	p := NewParser()
	got := feedAll(p, "\x1b[200~ab\x03cd\x1b[201~")
	require.Equal(t, []Event{PasteEvent{Payload: "ab\x03cd"}}, got)
}

func TestParser_EscFollowedByOtherChar_EmitsEscThenChar(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1bx")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyEsc},
		RuneEvent{R: 'x'},
	}, got)
}

func TestParser_EscEsc(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b\x1b")
	// After two ESCs we have emitted one Esc, with the second pending.
	require.Equal(t, []Event{KeyEvent{Key: KeyEsc}}, got)
	require.Equal(t, []Event{KeyEvent{Key: KeyEsc}}, p.FlushEsc())
}

func TestParser_EscFlushAlone(t *testing.T) {
	t.Parallel()

	p := NewParser()
	require.Empty(t, feedAll(p, "\x1b"))
	require.Equal(t, []Event{KeyEvent{Key: KeyEsc}}, p.FlushEsc())
	// Subsequent flush is a no-op.
	require.Empty(t, p.FlushEsc())
}

func TestParser_DSRResponseSwallowed(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[12;34R")
	require.Empty(t, got)
}

func TestParser_UTF8MultiByteRune(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "λ")
	require.Equal(t, []Event{RuneEvent{R: 'λ'}}, got)
}

func TestParser_UTF8SplitAcrossReads(t *testing.T) {
	t.Parallel()

	p := NewParser()
	all := []byte("世界")
	require.Empty(t, p.Feed(all[:1]))
	require.Empty(t, p.Feed(all[1:2]))
	got := p.Feed(all[2:])
	require.Equal(t, RuneEvent{R: '世'}, got[0])
}
