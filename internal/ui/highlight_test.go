package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/zthxxx/zsh-history-enquirer/internal/keys"
)

func TestHighlight_NoTokens(t *testing.T) {
	t.Parallel()
	require.Equal(t, "git status", highlight("git status", nil))
	require.Equal(t, "git status", highlight("git status", []string{}))
	require.Equal(t, "git status", highlight("git status", []string{""}))
}

func TestHighlight_SingleToken(t *testing.T) {
	t.Parallel()
	got := highlight("git status", []string{"git"})
	require.Equal(t, "\x1b[1;36mgit\x1b[0m status", got)
}

func TestHighlight_CaseInsensitive(t *testing.T) {
	t.Parallel()
	got := highlight("Git STATUS", []string{"git", "status"})
	require.Equal(t, "\x1b[1;36mGit\x1b[0m \x1b[1;36mSTATUS\x1b[0m", got)
}

func TestHighlight_OverlappingMatchesMerged(t *testing.T) {
	t.Parallel()
	// In "abab", "ab" matches at positions 0 and 2; "ba" matches at
	// position 1. The three spans (0,2), (2,4), (1,3) merge into
	// the single span (0,4), so the whole string is wrapped in one
	// open-close pair (no double SGR codes mid-string).
	got := highlight("abab", []string{"ab", "ba"})
	require.Equal(t, "\x1b[1;36mabab\x1b[0m", got)
}

func TestHighlight_NonAdjacentMatchesSeparate(t *testing.T) {
	t.Parallel()
	got := highlight("foo bar foo", []string{"foo"})
	require.Equal(t,
		"\x1b[1;36mfoo\x1b[0m bar \x1b[1;36mfoo\x1b[0m",
		got,
	)
}

func TestHighlight_NoMatch(t *testing.T) {
	t.Parallel()
	require.Equal(t, "git status", highlight("git status", []string{"xyz"}))
}

// TestHighlight_UnicodeFoldChangesByteLength pins the regression
// guard: when strings.ToLower(s) shrinks (or grows) byte length —
// classic example: Turkish capital İ (U+0130, 2 bytes) folds to
// "i" (U+0069, 1 byte) — slicing `s` with byte indices computed
// against the case-folded string produces invalid UTF-8. The
// highlighter must detect the length divergence and fall back to
// returning the original string unhighlighted, rather than emitting
// `\xb0...` mojibake to the terminal.
func TestHighlight_UnicodeFoldChangesByteLength(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		s    string
		toks []string
	}{
		{"turkish-I-fold", "İSTANBUL", []string{"stan"}},
		{"turkish-I-prefixed", "AİSTANBUL", []string{"stan"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := highlight(tc.s, tc.toks)
			// Sanity: must equal the original (no SGR codes added)
			// because byte-offset slicing is unsafe here.
			require.Equal(t, tc.s, got, "fallback should return %q unmodified", tc.s)
			// Sanity: must be valid UTF-8 (no byte-mangling).
			require.True(t, utf8.ValidString(got), "highlight produced invalid UTF-8: %q", got)
		})
	}
}

// TestHighlight_NoColorSuppressesSGR pins the NO_COLOR opt-out
// (https://no-color.org). When the env var is set to any non-empty
// value, the highlighter must return the input string unmodified —
// no `\x1b[1;36m` / `\x1b[0m` bytes. Match detection itself is
// orthogonal (search.AndFilter still works); only the visual
// markup is suppressed.
func TestHighlight_NoColorSuppressesSGR(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	got := highlight("git status", []string{"git"})
	require.Equal(t, "git status", got,
		"NO_COLOR set → highlighter must emit no SGR escapes")
}

// TestHighlight_EmptyNoColorStillHighlights — only NON-EMPTY values
// of NO_COLOR opt out (per the spec). NO_COLOR="" is the same as
// unset and color should still be emitted.
func TestHighlight_EmptyNoColorStillHighlights(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	got := highlight("git status", []string{"git"})
	require.Equal(t, "\x1b[1;36mgit\x1b[0m status", got,
		"NO_COLOR empty → behaves like unset; SGR still emitted")
}

// TestHighlight_AsciiPathStillHighlights — make sure the Unicode
// fallback didn't accidentally break the common-case ASCII path.
func TestHighlight_AsciiPathStillHighlights(t *testing.T) {
	t.Parallel()
	got := highlight("Git Log", []string{"git"})
	require.Equal(t, "\x1b[1;36mGit\x1b[0m Log", got,
		"ASCII still hits the SGR-wrap path despite the Unicode guard")
}

// TestSanitizeChoiceForRender pins the render-only sanitization
// of raw control bytes in history entries. A corrupt or
// malicious $HISTFILE entry could contain ESC bytes (e.g. from a
// pasted color sequence or a `printf '\x1b...' >> $HISTFILE`
// programmatic append). Without sanitization, rendering such an
// entry would let the embedded `\x1b[2J` clear the screen, or
// `\r` carriage-return mid-frame.
//
// Sanitization is render-only — m.Filter[i] keeps the original
// bytes so SubmitResult re-runs the user's command faithfully.
func TestSanitizeChoiceForRender(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "git status", "git status"},
		{"newline-kept", "echo a\nb", "echo a\nb"},
		{"tab-kept", "echo\tfoo", "echo\tfoo"},
		{"esc-replaced", "echo \x1b[31m red", "echo ^[[31m red"},
		{"cr-replaced", "echo a\rb", "echo a^Mb"},
		{"bel-replaced", "echo \x07 bell", "echo ^G bell"},
		{"del-replaced", "echo \x7f", "echo ^?"},
		{"clear-screen-neutered", "evil \x1b[2J", "evil ^[[2J"},
		{"multiple-esc", "\x1b[31m\x1b[1m", "^[[31m^[[1m"},
		{"unicode-untouched", "你好 world", "你好 world"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, sanitizeChoiceForRender(tc.in))
		})
	}
}

// TestRender_EntryESCNotPassedThrough is the integration-level
// guard: even when an entry contains a raw ESC, the rendered
// frame must not pass it to the terminal.
func TestRender_EntryESCNotPassedThrough(t *testing.T) {
	t.Parallel()
	choices := []string{"echo \x1b[2J malicious"}
	m := NewModel("", choices, 24, 80, 1, 1, DefaultMaxLimit)
	frame := m.Render(RenderOptions{})
	// The body string must NOT contain the literal ESC byte from
	// the entry. (Our own SGR codes — \x1b[1G cursor moves, etc.
	// — are present, but those are OUR escapes, not the entry's
	// raw bytes.) Concretely: "\x1b[2J" must be absent.
	require.NotContainsf(t, frame.Body, "\x1b[2J",
		"raw ESC from entry leaked through to terminal: %q", frame.Body)
	// The caret-notation form should appear instead.
	require.Contains(t, frame.Body, "^[[2J",
		"sanitized form must reach the rendered body")
}

// TestRender_SubmitReturnsUnsanitized — the entry-rendering
// sanitization must NOT affect the value returned by
// SubmitResult. The user pressing Enter on an entry containing
// a raw escape (whatever their reason — they typed it, or they
// have a corrupted history) must get the ORIGINAL bytes back so
// re-running the command behaves identically.
func TestRender_SubmitReturnsUnsanitized(t *testing.T) {
	t.Parallel()
	original := "echo \x1b[31m red"
	m := NewModel("", []string{original}, 24, 80, 1, 1, DefaultMaxLimit)
	require.Equal(t, original, m.SubmitResult(),
		"SubmitResult must return the un-sanitized entry bytes")
}

// TestRender_InputRowESCNotPassedThrough is the symmetric guard
// for the input row. The choice-rendering path is sanitized via
// sanitizeChoiceForRender; the input-row path is sanitized at
// write-time via sanitizeInputRune (so m.Input itself never
// contains a raw ESC). This integration check pins the contract
// from the renderer's perspective: even after a paste of a
// payload containing an ESC sequence, the rendered frame must
// not carry that ESC through to the terminal.
func TestRender_InputRowESCNotPassedThrough(t *testing.T) {
	t.Parallel()
	m := NewModel("", []string{"git status"}, 24, 80, 1, 1, DefaultMaxLimit)
	// Simulate a paste of a payload with an embedded clear-screen.
	m.Update(keys.PasteEvent{Payload: "find\x1b[2J ."})
	frame := m.Render(RenderOptions{})
	require.NotContainsf(t, frame.Body, "\x1b[2J",
		"raw ESC from paste leaked through to terminal: %q", frame.Body)
	// And m.Input itself must already be clean — no ESC byte
	// reaches storage, so subsequent renders / SubmitResult are
	// also safe.
	require.NotContains(t, m.Input, "\x1b",
		"m.Input must not retain raw ESC after sanitization")
}

// TestProperty_Highlight_Idempotent: running highlight twice with the
// same tokens on the result produces the same bytes — once we have
// inserted SGR codes around tokens, a second pass would only add new
// codes if it found unhighlighted token occurrences (it shouldn't,
// because the case-insensitive match would re-hit the same payload).
//
// In practice the second pass DOES re-find tokens (because the SGR
// codes don't change the payload's case-insensitive content). To keep
// the property meaningful, we assert a weaker invariant: highlighting
// a string that contains no tokens is a no-op.
func TestProperty_Highlight_NoMatchIsIdentity(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringMatching(`[a-y ]{0,40}`).Draw(rt, "s")
		// Token "z" never appears in `s` (alphabet is a-y).
		require.Equal(rt, s, highlight(s, []string{"z"}))
	})
}

// TestProperty_Highlight_PreservesPayload: stripping the SGR escapes
// from highlight(s, tokens) gives back s.
func TestProperty_Highlight_PreservesPayload(t *testing.T) {
	t.Parallel()
	rapid.Check(t, func(rt *rapid.T) {
		s := rapid.StringMatching(`[a-z ]{0,40}`).Draw(rt, "s")
		toks := rapid.SliceOf(rapid.StringMatching(`[a-z]{1,3}`)).Draw(rt, "tokens")

		got := highlight(s, toks)
		stripped := strings.ReplaceAll(got, highlightOn, "")
		stripped = strings.ReplaceAll(stripped, highlightOff, "")
		require.Equal(rt, s, stripped)
	})
}
