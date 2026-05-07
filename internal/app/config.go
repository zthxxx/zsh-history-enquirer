// Package app composes every other internal package into a runnable
// fx graph. Other packages must not import app.
package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zthxxx/zsh-history-enquirer/pkg/version"
)

// Config carries the parsed command-line / environment configuration
// for one run of the binary. Constructed by NewConfig and consumed by
// the rest of the fx graph.
type Config struct {
	// Input is the initial value the picker opens with — typically
	// the contents of $LBUFFER, joined by space if multi-arg.
	Input string

	// HistFile, if non-empty, overrides $HISTFILE.
	HistFile string

	// HistSize, if non-zero, overrides the default of 100k.
	HistSize int

	// MaxLimit caps the number of choices rendered (default 15).
	MaxLimit int

	// PrintVersion short-circuits after writing the version string.
	PrintVersion bool
}

// PrintHelp writes the binary's auto-generated usage to `out`. Used
// by `cmd/zsh-history-enquirer/main.go` for the `--help` fast-path
// so the help text isn't stacked under a confusing "startup failed:"
// message from the fx graph (which would otherwise see flag.ErrHelp
// from the inner Parse call).
func PrintHelp(out io.Writer) {
	fs := flag.NewFlagSet("zsh-history-enquirer", flag.ContinueOnError)
	fs.SetOutput(out)
	// Re-declare the same flags as NewConfig so the help text stays
	// in sync. Drift is caught by TestPrintHelp_MatchesNewConfig.
	fs.String("histfile", os.Getenv("HISTFILE"), "override $HISTFILE")
	fs.Int("histsize", 0, "HISTSIZE; 0 means default (100000)")
	fs.Int("max-limit", 0, "max choices rendered (0 = default 15)")
	fs.Bool("version", false, "print version and exit")
	fmt.Fprintf(out, "Usage: %s [flags] [initial input...]\n", fs.Name())
	fs.PrintDefaults()
	// Document environment variables so users discover them without
	// spelunking the README. Drift caught by
	// TestPrintHelp_MentionsEnvVars.
	fmt.Fprintln(out, "\nEnvironment:")
	fmt.Fprintln(out, "  HISTFILE   path to the zsh history file (overridden by --histfile)")
	fmt.Fprintln(out, "  NO_COLOR   any non-empty value disables token highlighting (no-color.org)")
	fmt.Fprintln(out, "  ZHE_DEBUG  path to a file for diagnostic logs (best-effort, never required)")
}

// NewConfig parses os.Args into a Config and returns it. Errors are
// wrapped with the full usage so the caller can write them to /dev/tty.
func NewConfig(args []string, stderr io.Writer) (*Config, error) {
	fs := flag.NewFlagSet("zsh-history-enquirer", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		histFile = fs.String("histfile", os.Getenv("HISTFILE"), "override $HISTFILE")
		histSize = fs.Int("histsize", 0, "HISTSIZE; 0 means default (100000)")
		maxLimit = fs.Int("max-limit", 0, "max choices rendered (0 = default 15)")
		showVer  = fs.Bool("version", false, "print version and exit")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, "Usage: %s [flags] [initial input...]\n", fs.Name())
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	cfg := &Config{
		Input:        strings.Join(fs.Args(), " "),
		HistFile:     *histFile,
		HistSize:     *histSize,
		MaxLimit:     *maxLimit,
		PrintVersion: *showVer,
	}
	return cfg, nil
}

// VersionLine renders the line printed when --version is set.
func VersionLine() string {
	return fmt.Sprintf("zsh-history-enquirer %s", version.Full())
}
