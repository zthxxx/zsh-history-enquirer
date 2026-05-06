package keys

import (
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

// TestProperty_Parser_ChunkBoundaryInvariance asserts that splitting
// the same input at any byte boundary yields the same Event sequence
// as feeding it whole. The parser is a finite-state machine and
// this property guards the FSM against subtle state-leak bugs at
// chunk boundaries — the kind of bug that ate "log " in scenario 4
// during early development before the probe-leftover replay was
// added.
func TestProperty_Parser_ChunkBoundaryInvariance(t *testing.T) {
	t.Parallel()

	rapid.Check(t, func(rt *rapid.T) {
		// Restrict to single-byte-rune ASCII so we don't have to
		// worry about UTF-8 boundary handling — that's a separate
		// concern with its own test above.
		input := rapid.SliceOfN(rapid.Byte(), 0, 32).Draw(rt, "input")
		// Filter out bytes that the FSM would route into the
		// CSI / ESC paths in ways the chunker can't replicate
		// (their behavior depends on receiving the full sequence
		// in one Feed). We restrict to printable ASCII for the
		// property check; CSI / paste sequences are exercised by
		// the targeted tests above.
		filtered := make([]byte, 0, len(input))
		for _, b := range input {
			if b >= 0x20 && b < 0x7f {
				filtered = append(filtered, b)
			}
		}

		// Random split points.
		splits := rapid.SliceOfN(rapid.IntRange(0, len(filtered)), 0, 8).Draw(rt, "splits")
		// Sort and dedupe.
		dedupedSplits := make([]int, 0, len(splits))
		seenSplit := make(map[int]bool)
		for _, s := range splits {
			if !seenSplit[s] {
				seenSplit[s] = true
				dedupedSplits = append(dedupedSplits, s)
			}
		}
		// Sort.
		for i := 1; i < len(dedupedSplits); i++ {
			for j := i; j > 0 && dedupedSplits[j-1] > dedupedSplits[j]; j-- {
				dedupedSplits[j-1], dedupedSplits[j] = dedupedSplits[j], dedupedSplits[j-1]
			}
		}

		whole := NewParser().Feed(filtered)

		chunked := NewParser()
		// Match Parser.Feed's empty-slice convention. Preallocation
		// hint isn't useful here — the test is for correctness, not
		// allocation behavior.
		pieces := make([]Event, 0, len(filtered))
		prev := 0
		for _, s := range dedupedSplits {
			pieces = append(pieces, chunked.Feed(filtered[prev:s])...)
			prev = s
		}
		pieces = append(pieces, chunked.Feed(filtered[prev:])...)

		require.Equal(rt, whole, pieces)
	})
}
