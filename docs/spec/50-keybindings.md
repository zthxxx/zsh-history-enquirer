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
row in order to make space. The eviction loop:

```
nextRows := wrapped_row_count(visible[idx+1])
totalRows := sum(wrapped_row_count(v) for v in visible) + nextRows
while totalRows >= terminal.height - 3:
    totalRows -= wrapped_row_count(visible.shift())
    rotate-down once
    idx -= 1
```

Without this, scrolling onto a long heredoc would push the picker off
the bottom of the terminal.

## Cancel / no-match-submit invariants

- <kbd>Esc</kbd> and <kbd>Ctrl</kbd>+<kbd>C</kbd>: the binary writes the
  current `input` to stdout and exits 0.
- <kbd>Enter</kbd> on an empty match list: same — write `input` to
  stdout and exit 0.
- <kbd>Enter</kbd> on a non-empty match list: write the focused match.

This is the "your typed text is never lost" invariant.
