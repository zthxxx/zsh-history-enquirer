# spec/20-history-loading — what the picker shows

## Source

- `$HISTFILE` (default `~/.zsh_history`) is the only source of truth.
- The legacy Node.js port shells out to a tiny zsh script that does
  `fc -R; fc -ln 1` with `HISTSIZE=100000`. The Go port shall do the
  same, because:
  - `fc -R` re-reads `$HISTFILE` from disk → entries from sibling
    interactive shells appear immediately.
  - `fc -ln 1` emits raw entries; piping `history` would pull in zsh's
    formatting, line numbers, and global filters.
  - The 100000 ceiling matches the legacy ceiling. Lower is annoying,
    higher is wasted memory in 99% of cases.

## Transformations applied (in order)

Given the raw lines from `fc -ln 1`:

1. **Reverse** so the most recent entry is index 0.
2. **Deduplicate**, keeping the first occurrence (i.e. the most recent
   instance of any duplicate command).
3. **Un-escape multi-line.** zsh stores commands containing newlines as
   one line per entry, with the embedded newlines written literally as
   `\n` (backslash-n). The picker shall replace every `\n` with a real
   newline so multi-line commands render as multiple lines.
4. **Strip trailing whitespace** is **not** applied; some commands
   legitimately end in space.

The cleaned list shall be passed to the search/UI layer as a sequence of
strings (`[]string` in Go).

## What is read straight from disk vs. via zsh

The default impl shells out to zsh because `fc -R; fc -ln 1` is the
only reliable way to read history that respects extended-history,
shared-history, and other zsh options. Reading the file directly with
Go would re-implement zsh's history parser badly. The Go port shall
keep the shell-out shape — it is fast enough (microseconds) and
indistinguishable in user-visible behaviour.

The fallback for unit testing shall be: read a fixture file directly
with the same parser zsh would use (extended-history `: <ts>:<dur>;<cmd>`
and a flat `<cmd>` form), so tests do not depend on a zsh process.

## Test invariants

For any input fixture:

- output is a permutation-with-deletion of the input lines
- duplicates do not appear
- the order of unique entries is the reverse-first order of the source
- no entry contains a literal `\n` substring (always real newlines)
