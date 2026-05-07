# spec/40-rendering — how the picker draws itself

## Inline placement

The picker is **inline**: it draws *under* the existing prompt and
leaves everything to the left of the prompt's start column untouched.
That includes Starship/Powerlevel10k/Spaceship segments, git status,
exit-code colours, etc.

To achieve this:

1. On startup, the binary emits a DSR (Device Status Report) escape
   `\e[6n` to `/dev/tty`.
2. The terminal replies with `\e[<row>;<col>R`.
3. The binary parses that, computes
   `initCol = current_col - cells(initial_input)`. `cells()` is
   `mattn/go-runewidth.StringWidth` (wrapped as `ui.CellWidth`) —
   East Asian Width-aware, so CJK ideographs / fullwidth
   punctuation / emoji each contribute 2 cells, combining marks
   contribute 0, and everything else contributes 1. Earlier
   approximations (rune-count, byte-count) all visibly mis-aligned
   the picker against the prompt for non-ASCII LBUFFER text.
4. All subsequent draws and erases use `initCol` as the leftmost column
   they may touch.

> **Why a DSR query and not `tput cols`?** `tput` gives terminal width;
> DSR gives the *current* position. The width changes nothing about
> where the prompt actually starts. Multi-line themes and right-prompt
> users would otherwise be misaligned.

## Layout

```
┌──────────────────────────────── terminal ────────────────────────────────┐
│ <prompt segments>… ▶ <picker input row, edits in place from initCol>     │
│   <pointer> <visible[0]>                                                 │
│   <pointer> <visible[1]>                                                 │
│   …                                                                      │
│   <pointer> <visible[k]>  ← reserved tail: 3 rows below the input        │
└──────────────────────────────────────────────────────────────────────────┘
```

- The pointer is a 2-character glyph (`›` plus a space, or similar).
- The picker reserves **3 rows** of headroom below the input so the
  shell prompt + a freshly-run command don't get pushed off the bottom
  of the terminal.

## Dynamic limit

The picker renders at most `limit` matches *and* at most as many as fit
in `terminal.height - 3` rows. It computes this each draw:

```
rows := 0
limit := 0
for choice in visible {
    cr := wrapped_row_count(pointer + choice, terminal.width)
    if rows + cr >= terminal.height - 3 { break }
    rows += cr
    limit++
    if limit >= options.limit { break }
}
```

`wrapped_row_count` of a string is:

```
sum over each "\n"-split line L of:
    ceil(cell_width(L) / width)  # 0-length line counts as 1
```

The first logical line additionally counts the 2-cell selection
pointer prefix (`›` + space).

`cell_width()` is `mattn/go-runewidth.StringWidth` (wrapped as
`ui.CellWidth`). Concretely:

- ASCII / Latin extended / Cyrillic / Greek / Hebrew / Arabic: 1
  cell per rune.
- CJK ideographs, fullwidth punctuation, emoji, hangul syllables:
  2 cells per rune.
- Combining marks, zero-width joiners: 0 cells.

The library packages the Unicode East Asian Width and emoji
presentation tables, kept current with Unicode updates upstream.
This replaces the earlier rune-count approximation (off by 1 per
CJK glyph) and the byte-count formerly inherited from the legacy
Node.js port.

`options.limit` defaults to **15**. The `--max-limit N` CLI flag
overrides it (mostly used by the e2e harness and `task run -- --max-limit 5`
debug invocations); there is no env var because the picker auto-
shrinks based on terminal height for the legitimate small-terminal
case.

## Erase + restore

The picker draws as an **overlay** on the user's existing zsh prompt at
a captured `initCol`. The input row itself can wrap when its cell width
exceeds the available room from `initCol` to the terminal's right
margin — so the renderer's bookkeeping must distinguish "rows the input
occupied" from "rows the choice list occupied."

On every render, the picker:

1. **Walks back** to the input start row. After the previous frame's
   restore step the cursor sits on whichever wrap row of the input the
   user's caret was on. The renderer remembers the previous
   `cursorRow` and emits `cursor prev-line × prev_cursor_row` to land
   back on row N before doing anything else.
2. **Clears** the area it owns from row N downward: erase the input
   row's tail starting at `initCol`, then walk down `prev_size` rows
   (= `prev_input_extra + prev_choice_rows`) emitting `\r\n + erase
   line` per row, then walk back up `prev_size` and reset the column.
   The crucial detail: erasing happens only **from `initCol` rightward**
   on the input row — the left of the prompt is never touched. Choice
   rows are erased completely because they own the full row width.
3. **Draws** the new input + visible list. The dynamic-limit walk
   uses `height_limit = terminal.height - 3 - input_extra` so the
   choice list always fits below a wrapped input.
4. **Restores** the cursor by walking up `(size - cursor_row)` from
   the bottom of the body to the cursor's wrap row, then
   `cursor to col cursor_col`. The cursor convention is "one cell to
   the right of the last typed char on that char's row, clamped to
   `terminal.width`" — matching readline / fish / vim deferred-wrap
   behaviour.

If `state.size` is 0 (empty matches), the picker still reserves one row
to draw a "no matches" hint or a blank line — it does not collapse
into the prompt row.

## Render frequency

The render call shall be **throttled to ~72 ms** with a leading-edge
fire (`render(); throttle = setTimeout(...)`). This is what keeps
keystroke combos and pasted text from blocking, while still feeling
instant. The legacy code uses lodash `throttle(..., 72, { leading: true })`;
the Go port uses an equivalent custom throttler in
`internal/ui/throttle.go`. A trailing-edge flush after the last
event in a burst guarantees the final state is always rendered
even if the burst started inside an existing throttle window.
