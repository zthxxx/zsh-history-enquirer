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
	"os"
	"time"

	"go.uber.org/fx"

	"github.com/zthxxx/zsh-history-enquirer/internal/app"
)

func main() {
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
		// hook. Honor the widget contract: exit 0.
		_ = err
		os.Exit(0)
	}

	// Wait for fx.Shutdowner. Done() returns the requested exit code
	// (always 0 in our case).
	<-a.Done()

	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()
	_ = a.Stop(stopCtx)
}
