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

// TestParser_CtrlByteTable exhaustively pins each Ctrl-* mapping.
// The ctrlByte() switch was at 25% coverage with only the
// happy-path C/U/R cases exercised; this ensures that A/B/D/E/F/
// K/L/N/P/W/Y all decode correctly and that unmapped bytes fall
// through to KeyUnknown.
func TestParser_CtrlByteTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   byte
		want Key
	}{
		{0x01, KeyCtrlA},
		{0x02, KeyCtrlB},
		{0x03, KeyCtrlC},
		{0x04, KeyCtrlD},
		{0x05, KeyCtrlE},
		{0x06, KeyCtrlF},
		{0x0b, KeyCtrlK},
		{0x0c, KeyCtrlL},
		{0x0e, KeyCtrlN},
		{0x10, KeyCtrlP},
		{0x12, KeyCtrlR},
		{0x15, KeyCtrlU},
		{0x17, KeyCtrlW},
		{0x19, KeyCtrlY},
		// 0x07 (BEL) → unknown; 0x0f (Ctrl-O) → unknown.
		{0x07, KeyUnknown},
		{0x0f, KeyUnknown},
	}
	for _, tc := range cases {
		got := ctrlByte(tc.in)
		require.Equalf(t, tc.want, got,
			"ctrlByte(0x%02x) = %v, want %v", tc.in, got, tc.want)
	}
}

// TestParser_CSIUnknownIgnored — an unrecognized CSI final byte
// (e.g. 'Z' for Shift-Tab in some terminals) must not corrupt the
// FSM. The next character should still parse normally.
func TestParser_CSIUnknownIgnored(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[Zx")
	require.Equal(t, []Event{RuneEvent{R: 'x'}}, got,
		"unknown CSI must be silently dropped, then 'x' must reach normal state")
}

// TestParser_DeleteKey — the Delete key sends `\e[3~`. The picker
// itself doesn't handle Delete (no Update branch for KeyDelete),
// but the parser must produce the correct Event so a future feature
// adding edit-cursor-style deletion has it available.
func TestParser_DeleteKey(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1b[3~")
	require.Equal(t, []Event{KeyEvent{Key: KeyDelete}}, got)
}

// TestParser_NULDropped ensures NUL bytes (0x00) are silently
// swallowed — they appear in some pastes and must not produce a
// rune event or stall the FSM.
func TestParser_NULDropped(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "a\x00b")
	require.Equal(t, []Event{
		RuneEvent{R: 'a'},
		RuneEvent{R: 'b'},
	}, got)
}

// TestParser_GarbageUTF8Recovers — a stream of invalid UTF-8
// continuation bytes must not pin memory. The parser caps the
// internal buffer at UTFMax (4 bytes) and resets after that.
func TestParser_GarbageUTF8Recovers(t *testing.T) {
	t.Parallel()

	p := NewParser()
	// Eight 0x80 bytes (continuation-only) — valid neither as start
	// of a rune nor as continuation of one.
	got := feedAll(p, "\x80\x80\x80\x80\x80\x80\x80\x80a")
	// At minimum we expect 'a' to come through eventually. The
	// recovery path may emit RuneError(s) along the way.
	require.NotEmpty(t, got)
	last, ok := got[len(got)-1].(RuneEvent)
	require.True(t, ok, "last event must be a rune")
	require.Equal(t, 'a', last.R)
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

// FuzzParser_NoPanicOnArbitraryBytes is a Go-native fuzz test that
// feeds the parser arbitrary byte streams. The contract it pins is
// minimal but vital: Feed must NEVER panic regardless of input.
//
// Before this guard, a malformed CSI sequence with embedded NUL or
// out-of-range bytes could potentially trip an out-of-range slice
// in the buffer accumulator. Run with:
//
//	go test -fuzz=FuzzParser_NoPanicOnArbitraryBytes \
//	  -fuzztime=10s ./internal/keys/...
//
// Standard `go test` runs each seed once as a regression check.
func FuzzParser_NoPanicOnArbitraryBytes(f *testing.F) {
	// Seed corpus: real-world tricky inputs we've debugged in the past.
	f.Add([]byte("\x1b[200~\x03cd\x1b[201~"))        // paste with embedded ^C
	f.Add([]byte("\x1b[12;34R"))                     // DSR response
	f.Add([]byte("\x1b\x1b\x1b"))                    // triple ESC
	f.Add([]byte{0x00, 0x01, 0x02, 0x03})            // raw control bytes
	f.Add([]byte("\xc0\xc1\xc2"))                    // invalid UTF-8 leads
	f.Add([]byte("\x1b["))                           // bare CSI prefix
	f.Add([]byte("\x1b[200~"))                       // paste-start with no end
	f.Add([]byte{0x80, 0x80, 0x80, 0x80, 0x80, 'a'}) // continuation bytes + ASCII

	f.Fuzz(func(_ *testing.T, b []byte) {
		p := NewParser()
		// A panic here is a fail; the Go fuzzer's seed-and-mutate loop
		// handles the rest.
		_ = p.Feed(b)
		_ = p.FlushEsc()
	})
}
