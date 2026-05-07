# spec/50-keybindings — what each key does

| Key | Action |
| --- | --- |
| any text byte | append to input, re-filter, reset selection to row 0 |
| <kbd>Backspace</kbd> | delete one rune to the left (NOT one byte — multi-byte UTF-8 chars must delete atomically), re-filter |
| <kbd>Ctrl</kbd>+<kbd>U</kbd> | clear input, re-filter |
| <kbd>Ctrl</kbd>+<kbd>W</kbd> | delete the previous word (strip trailing whitespace + the run of non-whitespace before it). Matches zsh's default `backward-kill-word` muscle memory |
| <kbd>Alt</kbd>+<kbd>Backspace</kbd> | alias for <kbd>Ctrl</kbd>+<kbd>W</kbd>. Maps the xterm-style meta prefix `\e\x7f` (and `\e\x08`) to the same word-delete path. Without this, the lone Esc would cancel the picker on every Alt+Backspace press, which is high-frequency muscle memory on macOS and Linux |
| <kbd>↑</kbd> / <kbd>Ctrl</kbd>+<kbd>P</kbd> | move selection up; if at top of visible window, scroll the visible window up by 1 (rotate-in-place). Ctrl-P matches zsh's emacs-keymap `up-line-or-history` |
| <kbd>↓</kbd> / <kbd>Ctrl</kbd>+<kbd>N</kbd> | move selection down; if at bottom, scroll down enough rows to make the next entry fit (taking multi-line into account). Ctrl-N matches zsh's emacs-keymap `down-line-or-history` |
| <kbd>PageUp</kbd> | rotate visible window up by `limit` |
| <kbd>PageDown</kbd> | rotate visible window down by `limit` |
| <kbd>Home</kbd> | reset visible window to top, selection to row 0 |
| <kbd>End</kbd> | reset visible window so the last match is at the bottom, selection on it |
| <kbd>Enter</kbd> | submit current focused match. If there is no match, submit the typed input verbatim |
| <kbd>Esc</kbd> | cancel; output equals typed input |
| <kbd>Ctrl</kbd>+<kbd>C</kbd> | cancel (same as Esc); output equals typed input |
| bracketed paste `\e[200~ … \e[201~` | append payload as text; never trigger key handlers inside the payload |

## Rotate-in-place semantics

The visible list is the same `[]string` as the matches list (or a slice
of it). "Scroll up by 1" means: pop the last element, push it to the
front. "Scroll down by 1": pop the first, push to the back.

This is an O(1) operation per row, which matters at HISTSIZE=100k. The
naïve approach (slice from i to i+limit) allocates a new array each
keystroke; that's the legacy bug we are not re-introducing.

## Multi-line scroll-down

When the selection is on the last visible row and the next match is
multi-line, scrolling down by 1 may evict more than one current visible
row in order to make space. The picker walks the visible list from the
bottom backward, accumulating wrapped row counts alongside the target
entry's rows, and keeps as many tail entries as fit in the
`terminal.height - 3 - inputExtra` budget. The leftover head entries
are rotated to the back; focus lands on the target.

```
budget := terminal.height - 3 - inputExtra
target := matches[m.Limit]                  # entry just below visible
total  := wrapped_row_count(target)
keep   := 0
for i := m.Limit - 1; i >= 0; i--:
    rows := wrapped_row_count(visible[i])
    if total + rows > budget: break
    total += rows
    keep++
shift := m.Limit - keep
if shift == 0:
    m.Idx = m.Limit          # render expands the limit on next pass
else:
    rotate-down by shift
    m.Idx = keep             # target now sits at this index
```

Without this, scrolling onto a long heredoc either pushes the picker
off the bottom of the terminal (if the dynamic limit isn't enforced)
or, worse, gets stuck — `renderBody` shrinks the limit and clamps
`m.Idx` back to the same logical entry, so the user observes a "lost"
keypress.

## Wrap-around at edges

When the entire filter fits in the visible window
(`len(Filter) <= m.Limit`), pressing ↓ at the bottom or ↑ at the
top wraps focus to the other end **without rotating the visible
list**. The displayed order is preserved across the wrap so the
user sees `[a-1, a-2, a-3]` continuously, not `[a-2, a-3, a-1]`
on the wrap-down or `[a-3, a-1, a-2]` on the wrap-up.

When `len(Filter) > m.Limit`, the wrap path doesn't apply — there
are entries to scroll into view, and ↑/↓ rotate the window
through them.

## Cancel / no-match-submit invariants

- <kbd>Esc</kbd> and <kbd>Ctrl</kbd>+<kbd>C</kbd>: the binary writes the
  current `input` to stdout and exits 0.
- <kbd>Enter</kbd> on an empty match list: same — write `input` to
  stdout and exit 0.
- <kbd>Enter</kbd> on a non-empty match list: write the focused match.

This is the "your typed text is never lost" invariant.

## Unbound keys (silently consumed)

The picker silently ignores keys that aren't in the table above —
no event reaches the model, the picker stays open, the user keeps
typing. Examples:

- <kbd>F1</kbd>..<kbd>F4</kbd> emit single-shift-three sequences
  (`\eOP`, `\eOQ`, `\eOR`, `\eOS`) that we don't bind to actions.
  An earlier version emitted `Esc + 'O' + body byte` here, so a
  stray F-key bumped the picker closed via the leading <kbd>Esc</kbd>
  — hostile UX.
- Aborted CSI / SS3 preludes from flaky links (`\e[` or `\eO`
  paused mid-sequence for >50 ms) were similarly mistaken for a
  deliberate <kbd>Esc</kbd> + body and cancelled the picker.
- <kbd>←</kbd> / <kbd>→</kbd> / <kbd>Tab</kbd> / <kbd>Delete</kbd>
  parse cleanly into recognized `KeyEvent`s but have no handler in
  the model — the picker has no in-input cursor movement and no
  in-line completion.

The shared rule: an event the model doesn't handle is a no-op, not
a cancel. Matches every modern fuzzy finder (fzf, peco, percol).
