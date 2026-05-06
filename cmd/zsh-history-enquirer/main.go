// Command zsh-history-enquirer is the binary the zsh widget invokes
// inside `BUFFER=$(zsh-history-enquirer "$LBUFFER")`.
//
// The contract is documented in docs/spec/10-widget-contract.md:
//
//   - argv[1..] joined by ' ' is the initial input
//   - stdout is the chosen line (no trailing newline beyond the one
//     `fmt.Println` adds, which `$(...)` strips)
//   - exit code is always 0 — even on cancel and on hard errors
//   - interactive I/O goes to /dev/tty, not stdout
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.uber.org/fx"

	"github.com/zthxxx/zsh-history-enquirer/internal/app"
)

// isVersionFlag reports whether os.Args is invoked as a pure
// `--version` query: the binary plus exactly one of the version
// flag tokens, with nothing else.
//
// The pickier check exists because the widget invokes the binary as
// `BUFFER=$(zsh-history-enquirer "$LBUFFER")`. If $LBUFFER is the
// literal string "--version", a sloppy contains-check would
// short-circuit and print the version into BUFFER instead of opening
// the picker — silently destroying the user's typed input. With this
// check, the picker opens normally because there's a positional arg
// alongside the flag.
func isVersionFlag(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[1] == "--version" || args[1] == "-version"
}

func main() {
	// Fast path: a bare `--version` doesn't need a TTY at all. Detect
	// it before fx.New so we don't open /dev/tty in environments where
	// it isn't usable (CI runners, scripts piped from a non-tty
	// shell, etc.). Print to stdout — that is the CLI convention
	// (so `zsh-history-enquirer --version | grep` works) and only
	// the *interactive* picker path uses stdout for the chosen
	// line. Version output and picker output are mutually exclusive.
	if isVersionFlag(os.Args) {
		fmt.Fprintln(os.Stdout, app.VersionLine())
		return
	}

	a := fx.New(
		app.Module,
		fx.NopLogger, // silence fx; the widget contract requires
		// stderr to stay quiet.
		fx.StartTimeout(5*time.Second),
		fx.StopTimeout(5*time.Second),
	)

	// Start runs every constructor + every Invoke synchronously. Our
	// only Invoke runs the picker. After it shuts down via
	// fx.Shutdowner the call returns and we exit the process.
	startCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.Start(startCtx); err != nil {
		// Errors from Start are already printed to stderr by the app
		// hook. Honor the widget contract: exit 0. Run cancel()
		// explicitly because os.Exit skips deferred functions.
		_ = err
		cancel()
		os.Exit(0) //nolint:gocritic // exitAfterDefer is acknowledged.
	}

	// Wait for fx.Shutdowner. Done() returns the requested exit code
	// (always 0 in our case).
	<-a.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	_ = a.Stop(stopCtx)
}
