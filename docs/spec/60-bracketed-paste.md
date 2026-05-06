# spec/60-bracketed-paste — paste handling

## Why this is its own spec

When a user pastes text into a terminal that has signalled support for
bracketed paste mode (most modern terminals do, by default), the
terminal wraps the paste in `\e[200~` and `\e[201~`. Inside that
window, no escape sequence inside the payload should be interpreted as
a key — the payload is just text.

The legacy enquirer Node.js library did **not** handle this, with the
result that pasting `git log` could trigger arrow-key handlers, cancels,
or page-down events depending on what bytes happened to be in the
clipboard. The Node.js port worked around it in `historySearcher.ts:dispatch`.

The Go port shall implement the same protection, by recognising the
two markers in the raw byte stream and entering a "literal append"
mode between them.

## State machine

```
state := normal

on byte b:
  case state, b in:
    (normal, '\e[200~'): state = paste; continue
    (paste, '\e[201~'):  state = normal; continue
    (paste, _):          append b to input; continue
    (normal, _):         interpret b as key
```

`\e[200~` and `\e[201~` may arrive split across reads. The byte parser
shall buffer partial sequences so they are recognised correctly across
read boundaries.

## Test scenarios

- paste `st` between `pasteStart` and `pasteEnd`, then type `at` — input
  should become `stat`.
- paste digits between markers when input already contains digits —
  digits are appended (no number-as-keypress handler).
- paste a payload containing the literal bytes of `^C` or `\e[A` — neither
  triggers cancel nor up-arrow.
