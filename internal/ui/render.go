package ui

import (
	"os"
	"slices"
	"strings"

	"github.com/charmbracelet/x/ansi"

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

	// Size is the number of body rows BELOW the input start row N —
	// that is, input wrap rows (when the input overflows the terminal
	// width) plus the choice rows. Stored on the model so the next
	// frame's Pre can erase exactly that many rows.
	Size int

	// Limit is the number of choices that ended up visible. The caller
	// stores this on the model so subsequent up/down navigation can
	// use it.
	Limit int

	// CursorRow is the 0-indexed row offset (from input row N) where
	// Post left the terminal cursor. When the input wraps, the cursor
	// rests on one of the input wrap rows rather than on row N itself,
	// so the next frame's Pre must walk up CursorRow lines before it
	// can address the input row to redraw it.
	CursorRow int
}

// RenderOptions carries the previous frame's geometry into the next
// render pass. We do not store these on the Model because the Model is
// otherwise a pure data struct; mixing rendering bookkeeping into it
// would leak abstraction.
type RenderOptions struct {
	// PrevSize is the previous Frame.Size (rows below input row N).
	PrevSize int

	// PrevCursorRow is the previous Frame.CursorRow — required when
	// the previous input wrapped and Post left the cursor below row N.
	// Defaults to 0 (the natural value for the very first frame and
	// for any prior frame whose input fit on a single row).
	PrevCursorRow int
}

// pointerSelected is the glyph drawn before the focused choice.
const pointerSelected = "› "

// pointerUnselected is the glyph drawn before non-focused choices.
const pointerUnselected = "  "

// Render produces a Frame describing how to draw the model on the TTY.
// The Frame is purely a byte-string description; it does not touch
// the terminal directly.
func (m *Model) Render(opts RenderOptions) Frame {
	// Compute input geometry once — the wrap math is consumed by all
	// three render stages (Body reserves choice space, Pre walks up
	// from the previous cursor row, Post lands the caret on the right
	// wrap row). m.Cursor is a cell count maintained by update.go's
	// edit ops; pass it through to the rune-walking formula so wide
	// glyphs that straddle a wrap boundary land the caret correctly.
	inputCells := CellWidth(m.Input)
	cursorCells := m.Cursor
	if cursorCells < 0 {
		cursorCells = 0
	}
	if cursorCells > inputCells {
		cursorCells = inputCells
	}
	inputExtra := InputExtraRows(m.InitCol, inputCells, m.Width)
	cursorRow, cursorCol := InputCursorPosition(m.InitCol, m.Input, cursorCells, m.Width)

	body, choiceRows, limit := m.renderBody(inputExtra)
	size := inputExtra + choiceRows
	pre := m.renderPre(opts.PrevSize, opts.PrevCursorRow)
	post := m.renderPost(size, cursorRow, cursorCol)
	m.Limit = limit
	return Frame{Pre: pre, Body: body, Post: post, Size: size, Limit: limit, CursorRow: cursorRow}
}

// choiceHeightLimit returns the row budget the dynamic-limit walk has
// for choices, given that the input row consumes 1 + inputExtra rows.
// Floored at 1 so the picker still draws something on a tiny terminal.
//
// Shared between renderBody (forward walk) and scrollToEnd (backward
// walk) so the two never disagree on row arithmetic — disagreement
// here was the bug class behind the wrap-math/sanitize regression.
func choiceHeightLimit(height, inputExtra int) int {
	limit := height - 3 - inputExtra
	if limit < 1 {
		limit = 1
	}
	return limit
}

// renderBody returns the body string, the number of CHOICE rows it
// occupies (excluding input wrap rows), and the visible-choice count.
// inputExtra is fed in so the dynamic-limit walk can subtract those
// rows from the available height — without it, a wrapped input
// silently steals choice space and we draw past the bottom.
func (m *Model) renderBody(inputExtra int) (string, int, int) { //nolint:gocritic // unnamed result is clearer here
	tokens := search.Tokenize(m.Input)

	// Step 1: write the input row at the captured prompt column.
	var body strings.Builder
	body.WriteString(ansi.CursorHorizontalAbsolute(m.InitCol))
	body.WriteString(ansi.EraseLineRight)
	body.WriteString(m.Input)

	// Step 2: walk the filtered list, accumulating row counts until
	// either MaxLimit or the available choice height is reached.
	// Reserve room for the input row itself plus its wrap rows — when
	// input overflows the terminal width the wrap rows shift choices
	// down, so heightLimit must shrink accordingly.
	heightLimit := choiceHeightLimit(m.Height, inputExtra)

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
	// Cap the cache to MaxLimit (15 by default), not len(m.Filter):
	// the loop breaks at `limit >= m.MaxLimit`, so the cache never
	// stores more than MaxLimit entries. Sizing the backing array to
	// the filter length wastes ~160 KB per render at HISTSIZE=100k
	// when filters are still wide (e.g. before the user has typed
	// enough tokens to narrow the matches), churning the GC for no
	// gain — Render runs on every keystroke (modulo throttle). Using
	// min so a tiny filter (say 3 matches against a wide MaxLimit)
	// still allocates only what it needs.
	cacheCap := m.MaxLimit
	if cacheCap > len(m.Filter) {
		cacheCap = len(m.Filter)
	}
	sanitizedCache := make([]string, 0, cacheCap)
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
// on a clean slate.
//
// Cursor location at entry: wherever the previous frame's Post left
// it. With wrap-aware rendering that's row N+prevCursorRow (0-indexed
// from input row N), col=cursorCol from prev input. To erase the prev
// body we first walk back to row N, then erase row N's input portion,
// then walk down prevSize rows erasing each, then walk back up to row
// N for the next Body to draw on.
//
// First frame (prevSize==0 && prevCursorRow==0) skips the walk-down
// step — there is no prior body to erase.
//
// SIGWINCH handling: when m.NeedsFullErase is set, after walking back
// to row N we additionally emit `\x1b[J` (EraseScreenBelow). Most
// terminals reflow wrapped lines on resize, so the previous frame's
// row offsets no longer correspond to physical positions; row-by-row
// erase via prevSize would leave reflowed leftovers visible until the
// user types one more keystroke. The broader erase costs 3 bytes per
// resize burst and the picker owns everything below row N for its
// session — safe to wipe.
func (m *Model) renderPre(prevSize, prevCursorRow int) string {
	var b strings.Builder
	// Walk up to row N from wherever the previous Post landed.
	if prevCursorRow > 0 {
		b.WriteString(ansi.CursorPreviousLine(prevCursorRow))
	}
	// Erase the input row's tail starting at the captured prompt col.
	b.WriteString(ansi.CursorHorizontalAbsolute(m.InitCol))
	b.WriteString(ansi.EraseLineRight)
	if m.NeedsFullErase {
		// One-shot full screen-below wipe; consumes the flag.
		b.WriteString(ansi.EraseScreenBelow)
		m.NeedsFullErase = false
		// After EraseScreenBelow there is no prior body left, so the
		// row-by-row walk would erase already-blank rows. Skip it.
		return b.String()
	}
	if prevSize <= 0 {
		// First frame (or a previous frame with empty body, which
		// cannot actually happen because we always emit "(no matches)"
		// when nothing matches): nothing more to erase.
		return b.String()
	}
	// Walk down the prevSize body rows, erasing each. CR+LF advances
	// to col 0 of the next row in raw mode; EraseLine wipes the row.
	for range prevSize {
		b.WriteString("\r\n")
		b.WriteString(ansi.EraseEntireLine)
	}
	// Walk back up to the input row and reset the column for Body to
	// continue from the captured prompt position.
	b.WriteString(ansi.CursorPreviousLine(prevSize))
	b.WriteString(ansi.CursorHorizontalAbsolute(m.InitCol))
	return b.String()
}

// renderPost places the terminal cursor at the user's caret, even when
// the input wrapped to additional rows. After Body, the cursor sits at
// row N+currentSize (the bottom of the body); we walk up the difference
// to land on the cursor's wrap row, then position it at cursorCol.
func (m *Model) renderPost(currentSize, cursorRow, cursorCol int) string {
	var b strings.Builder
	walkUp := currentSize - cursorRow
	if walkUp > 0 {
		b.WriteString(ansi.CursorPreviousLine(walkUp))
	}
	b.WriteString(ansi.CursorHorizontalAbsolute(cursorCol))
	return b.String()
}
