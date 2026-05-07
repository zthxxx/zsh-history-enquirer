package ui

import (
	"os"
	"slices"
	"strings"

	"github.com/zthxxx/zsh-history-enquirer/internal/ansi"
	"github.com/zthxxx/zsh-history-enquirer/internal/search"
)

// noColor reports whether the user has opted out of color output via
// the standard NO_COLOR env var (https://no-color.org). Any non-empty
// value disables ANSI color escapes — only token highlighting in our
// case; the picker has no other color output to suppress.
//
// Function (not const) so tests can stub via `t.Setenv("NO_COLOR", ...)`.
func noColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

// Highlight ANSI codes used to mark matched tokens inside a rendered
// choice. Bold cyan is distinguishable on every common terminal theme
// while staying neutral against typical accent colors.
const (
	highlightOn  = "\x1b[1;36m"
	highlightOff = "\x1b[0m"
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
	tokens := search.Tokenize(m.Input)

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

	// We compute wrap-rows on the SANITIZED text (the version that
	// will actually be written to the terminal), not the raw entry.
	// `sanitizeChoiceForRender` replaces control bytes like \x1b /
	// \x07 / \x7f with caret-notation (`^[`, `^G`, `^?`) — each 2
	// cells. The raw entry treats those bytes as 0 cells (runewidth
	// behaviour) and the wrap math would silently undercount, so an
	// entry like `cmd \x1b[2J` would slip into the visible window
	// claiming to fit when its actual rendered width forces a wrap.
	// Net effect: terminal auto-scrolls to fit the wrap, the next
	// renderPre erases too few rows, stale artefacts remain. Caching
	// the sanitized text and reusing it in the render loop below
	// keeps the cost to a single sanitize per visible entry.
	rows := 0
	limit := 0
	sanitizedCache := make([]string, 0, len(m.Filter))
	for _, choice := range m.Filter {
		s := sanitizeChoiceForRender(choice)
		choiceRows := WrappedRowCount(s, m.Width)
		if rows+choiceRows > heightLimit {
			break
		}
		rows += choiceRows
		sanitizedCache = append(sanitizedCache, s)
		limit++
		if limit >= m.MaxLimit {
			break
		}
	}

	if limit == 0 && len(m.Filter) > 0 {
		// At minimum one row should be drawn even when the list does
		// not fit the terminal — we cannot help the user otherwise.
		// The dynamic-limit walk above broke before populating the
		// cache, so we have to sanitize the first entry now to keep
		// the cache parallel with `limit`.
		sanitizedCache = append(sanitizedCache, sanitizeChoiceForRender(m.Filter[0]))
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
		// Multi-line entries are written after newline-to-CRLF
		// translation (terminals in raw mode need explicit CR for
		// the next line to start at col 0). Sanitization (raw
		// control bytes → caret notation) was already done above
		// during the dynamic-limit walk; reusing the cached value
		// here guarantees the rendered text and the wrap-row math
		// agree byte-for-byte. Original m.Filter[i] is untouched
		// so SubmitResult still re-runs the command faithfully.
		highlighted := highlight(sanitizedCache[i], tokens)
		body.WriteString(strings.ReplaceAll(highlighted, "\n", "\r\n"))
		// Belt-and-braces SGR reset after every entry — guards against
		// a history line containing an unterminated escape sequence
		// that would otherwise bleed colour into the next row. Skipped
		// under NO_COLOR — the user has signalled they don't want our
		// SGR bytes hitting the stream at all.
		if !noColor() {
			body.WriteString(highlightOff)
		}
	}

	if limit == 0 {
		body.WriteString("\r\n")
		body.WriteString("  (no matches)")
		rows = 1
	}

	return body.String(), rows, limit
}

// highlight wraps every occurrence of any token in `s` with the
// highlight ANSI sequence. Matching is case-insensitive but the
// original-case bytes are preserved in the output. Overlapping or
// adjacent matches are merged so the user never sees two open-codes
// in a row.
//
// Empty tokens are skipped — Tokenize already removes them, but be
// defensive against direct callers.
func highlight(s string, tokens []string) string {
	if len(tokens) == 0 || s == "" {
		return s
	}
	// NO_COLOR opt-out: emit no SGR escapes. Match-detection is
	// orthogonal — search.AndFilter still applies; only the visual
	// highlight is suppressed. Conforms to the no-color.org convention.
	if noColor() {
		return s
	}

	lc := strings.ToLower(s)
	// strings.ToLower can change byte length for some Unicode runes
	// (Turkish 'İ' → 'i' loses one byte; the ToLower implementation
	// also has expanding cases). When that happens, byte indices into
	// `lc` no longer point to character boundaries in `s`, and slicing
	// `s` with them would emit invalid UTF-8 to the terminal. The
	// highlight is purely cosmetic, so when the case-fold reshapes
	// byte offsets we fall back to returning the original string
	// unhighlighted — match-detection in `search.AndFilter` already
	// handled the lookup correctly; we just don't draw the SGR codes.
	if len(lc) != len(s) {
		return s
	}
	type span struct{ start, end int }
	var spans []span
	for _, t := range tokens {
		if t == "" {
			continue
		}
		offset := 0
		for offset < len(lc) {
			idx := strings.Index(lc[offset:], t)
			if idx < 0 {
				break
			}
			begin := offset + idx
			end := begin + len(t)
			spans = append(spans, span{begin, end})
			offset = end
		}
	}
	if len(spans) == 0 {
		return s
	}

	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	merged := spans[:1]
	for _, sp := range spans[1:] {
		last := &merged[len(merged)-1]
		switch {
		case sp.start <= last.end && sp.end > last.end:
			last.end = sp.end
		case sp.start <= last.end:
			// Fully contained; ignore.
		default:
			merged = append(merged, sp)
		}
	}

	var b strings.Builder
	cursor := 0
	for _, sp := range merged {
		b.WriteString(s[cursor:sp.start])
		b.WriteString(highlightOn)
		b.WriteString(s[sp.start:sp.end])
		b.WriteString(highlightOff)
		cursor = sp.end
	}
	b.WriteString(s[cursor:])
	return b.String()
}

// sanitizeChoiceForRender replaces raw control bytes in a choice
// with visible caret-notation placeholders so a corrupt or
// malicious history entry cannot disrupt the picker frame.
//
// Bytes preserved:
//   - '\t' and '\n' — already handled by the wrap math and the
//     newline → CRLF translation in renderBody.
//
// Bytes replaced:
//   - 0x1b (ESC) → "^[" — would otherwise let an entry like
//     `printf '\x1b[2J'` clear the user's screen on render.
//   - 0x7f (DEL) → "^?".
//   - Other 0x00..0x1f control bytes → "^X" caret notation.
//
// Sanitization is render-only; m.Filter[i] keeps the original
// bytes so SubmitResult returns the un-sanitized entry verbatim
// and re-running the picked command behaves the same as if the
// user had typed it.
func sanitizeChoiceForRender(s string) string {
	if !strings.ContainsAny(s,
		"\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\r\x0e\x0f"+
			"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b"+
			"\x1c\x1d\x1e\x1f\x7f") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '\t' || c == '\n':
			b.WriteByte(c)
		case c == 0x1b:
			b.WriteString("^[")
		case c == 0x7f:
			b.WriteString("^?")
		case c < 0x20:
			b.WriteByte('^')
			b.WriteByte(c + 0x40)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
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
