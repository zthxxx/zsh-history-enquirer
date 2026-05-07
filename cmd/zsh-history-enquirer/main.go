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
	"runtime"
	"time"

	"go.uber.org/fx"

	"github.com/zthxxx/zsh-history-enquirer/internal/app"
)

// Flag tokens that cause main() to short-circuit before fx.New.
// Constants exist to satisfy goconst — also used by the test.
const (
	versionFlagLong  = "--version"
	versionFlagShort = "-version"
	helpFlagLong     = "--help"
	helpFlagShort    = "-help"
	helpFlagShortest = "-h"
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
	return args[1] == versionFlagLong || args[1] == versionFlagShort
}

// isHelpFlag mirrors isVersionFlag for the help-text fast-path. A
// bare `--help` / `-help` / `-h` is treated as a documentation
// query — print the usage to stdout and exit 0 cleanly without
// running the fx graph (which would otherwise fail to parse the
// flag and emit a confusing "startup failed:" error to the user).
//
// The same pickier-check rationale applies: we never short-circuit
// when --help arrives in combination with positional args, so
// $LBUFFER text containing "--help" still opens the picker and
// the user's typed input is not destroyed.
func isHelpFlag(args []string) bool {
	if len(args) != 2 {
		return false
	}
	return args[1] == helpFlagLong || args[1] == helpFlagShort || args[1] == helpFlagShortest
}

// recoverStartFailure echoes the user's typed input ($LBUFFER) back
// to stdout when fx.App.Start fails before invokeRun could run. The
// widget contract requires `BUFFER=$(...)` to never blank user input.
// Errors are logged to stderr; argv parse errors are tolerated (fall
// back to no stdout, but at least don't crash). Exposed for testing.
func recoverStartFailure(stdout, stderr *os.File, args []string, startErr error) {
	fmt.Fprintln(stderr, "zsh-history-enquirer: startup failed:", startErr)
	if cfg, cfgErr := app.NewConfig(args, stderr); cfgErr == nil && cfg.Input != "" {
		fmt.Fprintln(stdout, cfg.Input)
	}
}

// recoverPanic preserves the widget contract through a runtime panic.
// fx surfaces provider/invoker panics as start errors, but a goroutine
// panic later in the picker session (a bug in update.go or render.go,
// or a third-party panic from x/ansi / uniseg) would otherwise let the
// process crash with no stdout output — `BUFFER=$(...)` then resolves
// to empty and the user's typed `$LBUFFER` is gone. Top-level recover
// echoes argv back to stdout so BUFFER survives even on the crash
// path. The crash itself is reported to stderr (invisible to `$(...)`)
// and process exits 0 so the substitution doesn't propagate failure.
func recoverPanic(stdout, stderr *os.File, args []string) {
	if r := recover(); r != nil {
		handlePanicRecovery(stdout, stderr, args, r)
		os.Exit(0)
	}
}

// handlePanicRecovery is the os.Exit-free body of recoverPanic so tests
// can exercise the recovery flow without terminating the test process.
func handlePanicRecovery(stdout, stderr *os.File, args []string, r any) {
	fmt.Fprintln(stderr, "zsh-history-enquirer: panic recovered:", r)
	fmt.Fprintln(stderr, "zsh-history-enquirer: stack:")
	_, _ = stderr.Write(debugStack())
	recoverStartFailure(stdout, stderr, args, fmt.Errorf("panic: %v", r))
}

// debugStack returns the runtime stack of the current goroutine in a
// format suitable for stderr logging. Wrapped so tests can stub.
//
//nolint:gochecknoglobals // intentional swap point for tests.
var debugStack = runtimeStack

func runtimeStack() []byte {
	const stackBufSize = 64 * 1024
	buf := make([]byte, stackBufSize)
	n := runtime.Stack(buf, false)
	return buf[:n]
}

func main() {
	defer recoverPanic(os.Stdout, os.Stderr, os.Args[1:])
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

	// Same fast-path treatment for --help: don't run fx, just print
	// the auto-generated usage to stdout and exit 0. Without this,
	// flag.ErrHelp from inside NewConfig surfaces as a confusing
	// "startup failed: ... flag: help requested" message stacked
	// after the help text the user actually wanted.
	if isHelpFlag(os.Args) {
		// Help text goes to stdout — matches the CLI convention so
		// `zsh-history-enquirer --help | grep histfile` works the
		// same way `--version | grep` does.
		app.PrintHelp(os.Stdout)
		return
	}

	a := fx.New(
		app.Module,
		fx.NopLogger, // silence fx; the widget contract requires
		// stderr to stay quiet.
		// StartTimeout covers the *interactive picker session* because
		// invokeRun runs Run() synchronously inside OnStart. A real
		// human can keep the picker open for arbitrary time (typing,
		// stepping out for coffee). 1h is a "no realistic upper bound"
		// stand-in. SIGINT/SIGTERM still tears it down via the
		// runCtx-wrapping signal.NotifyContext in invokeRun. The
		// previous 5s timeout caused interactive sessions to crash
		// with "context deadline exceeded" after sitting idle on the
		// picker — a real e2e regression caught by scenario 19.
		fx.StartTimeout(1*time.Hour),
		fx.StopTimeout(5*time.Second),
	)

	// Start runs every constructor + every Invoke synchronously. Our
	// only Invoke runs the picker. After it shuts down via
	// fx.Shutdowner the call returns and we exit the process.
	startCtx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
	defer cancel()
	if err := a.Start(startCtx); err != nil {
		// Provider-time failure (e.g. /dev/tty unopenable in a headless
		// container). invokeRun never ran, so preserveOnError inside
		// the app module never had the chance to echo back $LBUFFER.
		// Honor the widget contract here as a top-level safety net:
		// reconstruct the input from argv and print it so that
		// `BUFFER=$(...)` does not blank the user's typed input.
		recoverStartFailure(os.Stdout, os.Stderr, os.Args[1:], err)
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
