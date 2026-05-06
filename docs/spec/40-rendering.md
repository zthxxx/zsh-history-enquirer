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
   `initCol = current_col - len(initial_input) - 1` (`-1` because
   DSR positions are 1-indexed and we treat them 0-indexed internally).
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
    ceil(len(L) / width)        # 0-length line counts as 1
```

This is deliberately a byte-length estimate (treat one rune = one cell);
for non-CJK text it is correct, for CJK it slightly under-counts, which
is acceptable — the user just sees one fewer match.

`options.limit` defaults to **15**. It can be lowered in tests or by an
env var, but the user-facing default is fixed.

## Erase + restore

On every render, the picker:

1. **Clears** the area it owns: cursor down `state.size` rows, then
   `(erase line + cursor prev-line) × state.size`, then move cursor to
   `(row=current, col=initCol)` and erase from there to end-of-line.
   The crucial detail: clearing erases only **from `initCol` rightward**
   — the left of the prompt is never touched.
2. **Draws** the new input + visible list.
3. **Restores** the cursor up `state.size` rows and to `(initCol + cursor)`
   on the input row, where `cursor` is the user's caret offset within
   their typed input.

If `state.size` is 0 (empty matches), the picker still reserves one row
to draw a "no matches" hint or a blank line — it does not collapse
into the prompt row.

## Render frequency

The render call shall be **throttled to ~72 ms** with a leading-edge
fire (`render(); throttle = setTimeout(...)`). This is what keeps
keystroke combos and pasted text from blocking, while still feeling
instant. The legacy code uses lodash `throttle(..., 72, { leading: true })`;
the Go port uses an equivalent custom throttler since
bubbletea does not natively expose this knob.
