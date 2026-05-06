# design/20-history — package internals

> Spec: [spec/20-history-loading.md](../spec/20-history-loading.md)

## Public API

```go
package history

// Loader returns a list of history entries already reversed,
// deduped, and with embedded "\n" sequences un-escaped.
type Loader interface {
    Load(ctx context.Context) ([]string, error)
}

// NewZshLoader produces a Loader that shells out to zsh.
func NewZshLoader(opts Options) Loader

// FixtureLoader parses a file in the zsh extended-history format
// (or a flat one-command-per-line form) and returns its lines
// without invoking zsh. Used for unit tests.
func FixtureLoader(path string) Loader

// Options controls the zsh loader.
type Options struct {
    HistFile  string // override for $HISTFILE; empty → user default
    HistSize  int    // override for HISTSIZE; 0 → 100000
    ZshScript string // override for the inline script; empty → built-in
}
```

## ZshLoader internals

The loader runs:

```sh
zsh -c '
  export HISTFILE="${1:-${HISTFILE:-$HOME/.zsh_history}}"
  export HISTSIZE=${2:-100000}
  fc -R
  fc -ln 1
' _ "$histfile" "$histsize"
```

Notes:

- The `fc -R` re-reads from disk so other sibling shells' commands
  appear immediately.
- `fc -ln 1` emits raw entries, one per line (multi-line commands have
  embedded `\n` literals).
- We pipe the script via `zsh -c` instead of invoking a separate file,
  so there is no path-resolution concern at runtime. The legacy port
  shipped a `history.zsh` file, which led to release-time bugs about
  whether the file made it into the bundle. We sidestep that.

## Transform pipeline

```go
func ReverseDedupeUnescape(lines []string) []string
```

is a pure function. Given `[a, b, a, c\nd, b]` it returns
`[b, "c<NL>d", a]` — reverse first, dedupe, then per-line replace
`\\n` with `\n`.

The reverse-then-dedupe order matters: deduping after reversing means
"keep the most recent occurrence", not "keep the oldest". That is what
matches user expectation of `^R`.

## Property-based test contract

Using `pgregory.net/rapid`:

- generator: arbitrary list of strings, possibly containing `\n` literals
- properties:
  - `len(out) ≤ len(in)` (deduplication can only shrink)
  - `set(out) == set(reverse(in)) - duplicates`
  - the order of unique elements in `out` matches their order in
    `reverse(in)` (when an element appears multiple times in `in`, only
    its latest position counts)
  - no element of `out` contains the substring `"\\n"` (literal
    backslash-n) unless it was escaped (`"\\\\n"` in the source)

## Unit tests touch nothing on disk

- `FixtureLoader` reads a checked-in fixture under `testdata/`.
- `ZshLoader` tests use `t.TempDir()` and write a synthetic
  `~/.zsh_history` there, with `HISTFILE` pointed at it. They never
  touch the user's real history file.
- Property tests use generators only — they never spawn zsh.

E2E tests (in Docker, see `e2e/`) exercise the full
`ZshLoader → ZshLoader` round trip with synthetic histories.
