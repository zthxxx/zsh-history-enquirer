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

// TestParser_ModifierArrowKeys covers the xterm-style modified-key
// CSI sequences. Without the modifier-strip the parser would only
// match plain `\e[A` / `\e[B` / etc. and silently drop every
// Shift+Up / Alt+Up / Ctrl+Up / etc. keypress. The picker has no
// per-modifier behavior, so we treat every modifier form as the
// plain navigation key.
func TestParser_ModifierArrowKeys(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		seq  string
		want Key
	}{
		{"shift-up", "\x1b[1;2A", KeyUp},
		{"alt-up", "\x1b[1;3A", KeyUp},
		{"shift-alt-up", "\x1b[1;4A", KeyUp},
		{"ctrl-up", "\x1b[1;5A", KeyUp},
		{"ctrl-shift-up", "\x1b[1;6A", KeyUp},
		{"ctrl-down", "\x1b[1;5B", KeyDown},
		{"shift-right", "\x1b[1;2C", KeyRight},
		{"alt-left", "\x1b[1;3D", KeyLeft},
		{"ctrl-home", "\x1b[1;5H", KeyHome},
		{"shift-end", "\x1b[1;2F", KeyEnd},
		{"ctrl-pgup", "\x1b[5;5~", KeyPageUp},
		{"ctrl-pgdn", "\x1b[6;5~", KeyPageDown},
		{"shift-delete", "\x1b[3;2~", KeyDelete},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := NewParser()
			got := feedAll(p, tc.seq)
			require.Equal(t,
				[]Event{KeyEvent{Key: tc.want}},
				got,
				"%s should resolve to %v", tc.name, tc.want)
		})
	}
}

// TestStripCSIModifier_PassthroughForUnrelatedSeqs makes sure the
// modifier-strip helper does NOT alter sequences that aren't in the
// xterm modifier-encoding shape — DSR replies, OSC strings, unknown
// CSI letters etc. should reach the dispatch table unchanged.
func TestStripCSIModifier_PassthroughForUnrelatedSeqs(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"":       "",
		"R":      "R",      // DSR cursor reply
		"15;5R":  "15;5R",  // unrelated DSR-like
		"4n":     "4n",     // unrelated CSI
		"1;abcA": "1;abcA", // not all digits → passthrough
		"5~":     "5~",     // PgUp plain, no modifier
		"~":      "~",      // single tilde
	}
	for seq, want := range cases {
		got := stripCSIModifier(seq)
		require.Equalf(t, want, got, "stripCSIModifier(%q)", seq)
	}
}

// TestParser_SS3ArrowKeys covers the SS3 (Single Shift 3) variant
// `\eOA` etc. that some terminals (xterm in app-keypad mode, certain
// VT-emulators, embedded firmware terminals) send instead of the CSI
// `\e[A` form. Without SS3 handling, every arrow press in such a
// terminal would surface as Esc + 'O' + arrow letter — the picker
// would CANCEL on the Esc and append "OA"/"OB" to the input.
func TestParser_SS3ArrowKeys(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1bOA\x1bOB\x1bOC\x1bOD\x1bOH\x1bOF")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyUp},
		KeyEvent{Key: KeyDown},
		KeyEvent{Key: KeyRight},
		KeyEvent{Key: KeyLeft},
		KeyEvent{Key: KeyHome},
		KeyEvent{Key: KeyEnd},
	}, got)
}

// TestParser_SS3UnknownByteFallsBackSafely ensures an unrecognized
// SS3 sequence (`\eOX`) doesn't get swallowed silently — we emit
// Esc + 'O' + 'X' so the user at least sees something happen
// (matches pre-SS3-fix behavior for those bytes).
func TestParser_SS3UnknownByteFallsBackSafely(t *testing.T) {
	t.Parallel()

	p := NewParser()
	got := feedAll(p, "\x1bOX")
	require.Equal(t, []Event{
		KeyEvent{Key: KeyEsc},
		RuneEvent{R: 'O'},
		RuneEvent{R: 'X'},
	}, got)
}

// TestParser_FlushEsc_DuringSS3Pending releases an unfinished SS3
// prelude (the user's terminal emitted `\eO` then nothing for >50ms).
// Before this fix it would have remained in stateSS3 indefinitely,
// blocking ALL subsequent input until a key code byte arrived.
func TestParser_FlushEsc_DuringSS3Pending(t *testing.T) {
	t.Parallel()

	p := NewParser()
	require.Empty(t, feedAll(p, "\x1bO"), "no events emitted yet")
	got := p.FlushEsc()
	require.Equal(t, []Event{
		KeyEvent{Key: KeyEsc},
		RuneEvent{R: 'O'},
	}, got)
}

// TestParser_AltBackspaceMapsToCtrlW pins the Mac/iTerm/xterm meta
// modifier behavior: Alt+Backspace arrives as `\e\x7f` (or `\e\x08`
// on terminals using BS as backspace). zsh's emacs keymap binds the
// chord to backward-kill-word; the picker mirrors it as Ctrl-W.
//
// Without the fix, the lone Esc would cancel the picker on every
// Alt+Backspace press — a high-frequency footgun for shell users
// who reach for word-delete muscle memory mid-search. The chord
// arrives as a single Read() (terminals emit the two bytes in one
// write), so the parser sees them paired and routes through this
// branch.
func TestParser_AltBackspaceMapsToCtrlW(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
	}{
		{"alt-backspace-del", "\x1b\x7f"},
		{"alt-backspace-bs", "\x1b\x08"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := NewParser()
			got := feedAll(p, tc.in)
			require.Equal(t, []Event{KeyEvent{Key: KeyCtrlW}}, got,
				"Alt+Backspace must map to Ctrl-W, not Esc + Backspace")
		})
	}
}

// TestParser_PlainEscThenBackspaceStillCancels guards the slow path:
// when the user presses Esc deliberately, then Backspace some time
// later, the bytes arrive in separate Feed calls (the reader's
// flushTimer fires between them and emits the standalone Esc). The
// Alt+Backspace fast-path must NOT swallow the Esc in that case.
func TestParser_PlainEscThenBackspaceStillCancels(t *testing.T) {
	t.Parallel()

	p := NewParser()
	// First feed: just Esc. State enters stateEsc.
	got := feedAll(p, "\x1b")
	require.Empty(t, got, "Esc alone emits nothing until flush or follow-up byte")

	// Caller's flushTimer would fire here (50ms timeout in reader.go).
	flushed := p.FlushEsc()
	require.Equal(t, []Event{KeyEvent{Key: KeyEsc}}, flushed)

	// Subsequent Backspace arrives in stateNormal — emitted as plain
	// Backspace, no chord interpretation.
	got2 := feedAll(p, "\x7f")
	require.Equal(t, []Event{KeyEvent{Key: KeyBackspace}}, got2)
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

// TestParser_InvalidLeadDoesNotSwallowFollowingASCII pins the
// resync regression. Earlier the parser's UTFMax-cap logic dropped
// the entire 4-byte buffer when decode failed; if a stray
// continuation byte (0xbd here) was followed by valid ASCII, those
// 3 ASCII bytes were silently lost — the user's `abc` typed after
// a stray byte simply never reached the picker. The fix walks the
// buffer one byte at a time on resync, so only the invalid lead
// is dropped and the trailing ASCII surfaces.
func TestParser_InvalidLeadDoesNotSwallowFollowingASCII(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []byte
		want []rune
	}{
		{
			name: "stray-continuation-then-ascii",
			in:   []byte{0xbd, 'a', 'b', 'c'},
			want: []rune{'a', 'b', 'c'},
		},
		{
			name: "lone-0xff-then-ascii",
			in:   []byte{0xff, 'g', 'i', 't'},
			want: []rune{'g', 'i', 't'},
		},
		{
			name: "valid-emoji-then-stray-then-ascii",
			in:   []byte{0xf0, 0x9f, 0x9a, 0x80, 0xff, 'x'},
			want: []rune{0x1f680, 'x'},
		},
		{
			name: "incomplete-4byte-lead-then-ascii",
			// 0xf0 expects 3 continuation bytes. With only ASCII
			// after, decode fails until the buffer hits UTFMax;
			// then the lead is dropped and the ASCII resyncs.
			in:   []byte{0xf0, 'a', 'b', 'c'},
			want: []rune{'a', 'b', 'c'},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := NewParser()
			events := p.Feed(tc.in)
			got := make([]rune, 0, len(events))
			for _, ev := range events {
				if re, ok := ev.(RuneEvent); ok {
					got = append(got, re.R)
				}
			}
			require.Equal(t, tc.want, got,
				"valid bytes after invalid lead must surface")
		})
	}
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
