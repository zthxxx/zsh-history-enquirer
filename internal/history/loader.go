package history

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Loader returns a list of history entries already reversed, deduped,
// and with embedded literal "\n" sequences un-escaped.
//
// The caller may rely on the order:
//   - first element = most recently used unique command,
//   - last element  = oldest unique command.
type Loader interface {
	Load(ctx context.Context) ([]string, error)
}

// Options configures the canonical zsh-backed loader.
type Options struct {
	// HistFile overrides $HISTFILE / ~/.zsh_history. Empty string
	// means use the user's default.
	HistFile string

	// HistSize overrides HISTSIZE. Zero means use the default of
	// 100000, matching the legacy implementation.
	HistSize int

	// ZshBinary is the absolute path to the zsh executable. Empty
	// string means use whatever PATH lookup finds first.
	ZshBinary string
}

// DefaultHistSize is the value injected when Options.HistSize is zero.
// 100k matches the legacy Node.js implementation and is large enough
// that real users do not run into truncation.
const DefaultHistSize = 100_000

// NewZshLoader returns a Loader that shells out to zsh and runs
// `fc -R; fc -ln 1` to obtain raw history lines. The pipeline is
// completed by ReverseDedupeUnescape.
//
// Why shell out instead of parsing $HISTFILE directly?
//   - `fc -R` re-reads the file from disk so commands typed in
//     sibling shells appear immediately.
//   - `fc -ln 1` produces the canonical zsh decoding (extended-history
//     timestamps stripped, escapes handled). Parsing the file
//     ourselves would re-implement this badly.
//   - The cost is one process exec, ~5 ms — acceptable next to the
//     DSR round-trip we already pay.
func NewZshLoader(opts Options) Loader {
	if opts.HistSize == 0 {
		opts.HistSize = DefaultHistSize
	}
	return &zshLoader{opts: opts}
}

type zshLoader struct {
	opts Options
}

// inlineScript runs once per Load() and is fed to `zsh -c`. We pass
// HISTFILE / HISTSIZE via positional arguments instead of environment
// variables so any HISTFILE/HISTSIZE in the caller's environment
// cannot accidentally override the test fixture path.
const inlineScript = `
HISTFILE="${1:-${HISTFILE:-$HOME/.zsh_history}}"
HISTSIZE="${2:-100000}"
export HISTFILE HISTSIZE
fc -R 2>/dev/null || :
fc -ln 1
`

func (l *zshLoader) Load(ctx context.Context) ([]string, error) {
	bin := l.opts.ZshBinary
	if bin == "" {
		bin = "zsh"
	}

	histFile := l.opts.HistFile
	if histFile == "" {
		histFile = os.Getenv("HISTFILE")
	}

	cmd := exec.CommandContext(ctx, bin, "-c", inlineScript, "_",
		histFile,
		fmt.Sprintf("%d", l.opts.HistSize),
	)
	// The widget runs inside `$(zsh-history-enquirer "$LBUFFER")`,
	// where stdin is a tty. We do not want zsh to inherit it, since
	// that would compete with our key reader for input. We connect
	// stdin to /dev/null instead.
	cmd.Stdin = nil
	// stderr is sent to /dev/null on purpose: a missing $HISTFILE
	// or pre-existing zsh warnings should not corrupt the picker.
	// Programmatic errors (zsh missing entirely) surface as a
	// non-zero exit code which we report.
	cmd.Stderr = nil

	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("zsh history exec failed: %w (stderr: %s)", err, string(ee.Stderr))
		}
		return nil, fmt.Errorf("zsh history exec failed: %w", err)
	}

	raw := splitNonEmptyLines(string(out))
	return ReverseDedupeUnescape(raw), nil
}

// FixtureLoader returns a Loader that reads a file containing one
// history entry per line, applies the same transform pipeline as
// ZshLoader, and returns the result. Used in unit tests so they do
// not require a zsh installation.
//
// Lines beginning with `: <ts>:<dur>;` (extended-history) are
// trimmed of that prefix to match what `fc -ln 1` would produce.
func FixtureLoader(path string) Loader {
	return &fixtureLoader{path: path}
}

type fixtureLoader struct{ path string }

func (l *fixtureLoader) Load(_ context.Context) ([]string, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return nil, fmt.Errorf("read fixture %q: %w", l.path, err)
	}
	raw := splitNonEmptyLines(string(data))
	cleaned := make([]string, 0, len(raw))
	for _, ln := range raw {
		cleaned = append(cleaned, stripExtendedHistoryPrefix(ln))
	}
	return ReverseDedupeUnescape(cleaned), nil
}

func splitNonEmptyLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// stripExtendedHistoryPrefix removes the optional `: <ts>:<dur>;`
// prefix added when zsh's `extended_history` option is enabled.
func stripExtendedHistoryPrefix(line string) string {
	if !strings.HasPrefix(line, ": ") {
		return line
	}
	if i := strings.Index(line, ";"); i >= 0 {
		return line[i+1:]
	}
	return line
}
