# spec/30-search-and-filter — multi-word AND-filter

## Tokenisation

- Input is the combined `argv[1..]` plus anything the user has typed.
- Tokenisation: split on ASCII whitespace, drop empty tokens, **lowercase
  every token**.
- If the resulting list is empty, the filter is the identity function
  (every entry passes).

## Match predicate

An entry `e` matches an input `i` iff every token of `i` is a substring
of `lowercase(e)`. Order does not matter. Tokens never anchor — there
is no `^` or `$` semantics.

## Examples (mandatory test cases)

| input | matches | does not match |
| --- | --- | --- |
| `git` | `git status`, `cd git-repo`, `Git Push` | `where php` |
| `git st` | `git status`, `git stash` | `git log` |
| `log iso` | `git log --pretty=fuller --date=iso -n 1` | `git log` |
| `LOG ISO` | same as above (case-insensitive) | |
| (empty) | every entry | nothing |

## Ranking / ordering

Filter does **not** rank. The order of matches is the order in the
post-load list, which is reverse-chronological with duplicates removed.
This is what makes "most recent matching command first" feel right,
and what differentiates this from `fzf` (which scores by edit distance
and produces unstable orderings as you type).

## Out of scope (intentionally)

- Fuzzy/typo-tolerant matching (`fzf`-style scoring).
- Glob or regex matching.
- Per-token negation.

These are deliberately excluded: a sharper filter is one of the things
this picker has over `fzf` and we do not want to chase feature parity.
