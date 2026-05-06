package app

import (
	"context"
	"fmt"
	"io"
	"os"

	"go.uber.org/fx"

	"github.com/zthxxx/zsh-history-enquirer/internal/history"
	"github.com/zthxxx/zsh-history-enquirer/internal/tty"
)

// Module is the canonical fx module used by cmd/zsh-history-enquirer.
// Tests build a smaller graph that overrides individual providers.
//
//nolint:gochecknoglobals // standard fx idiom.
var Module = fx.Module("app",
	fx.Provide(
		func() (*Config, error) {
			return NewConfig(os.Args[1:], os.Stderr)
		},
		func() io.Writer { return os.Stdout },
		func() io.Writer { return os.Stderr },
		tty.NewDevTTY,
		func(cfg *Config) history.Loader {
			return history.NewZshLoader(history.Options{
				HistFile: cfg.HistFile,
				HistSize: cfg.HistSize,
			})
		},
	),
	fx.Invoke(invokeRun),
)

// invokeRun is the only Invoke in the fx graph. It bridges from the
// constructed providers to the (synchronous) Run() entrypoint.
func invokeRun(
	lc fx.Lifecycle,
	cfg *Config,
	t *tty.TTY,
	loader history.Loader,
	shutdowner fx.Shutdowner,
	stdout, stderr io.Writer,
) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Run synchronously inside the start hook so that fx's
			// shutdown sequence (TTY cleanup) does not race the
			// terminal print. Returning a non-nil error from OnStart
			// aborts startup and triggers stop hooks in reverse,
			// which is exactly what we want on probe / load failure.
			_ = ctx
			result, err := Run(context.Background(), cfg, t, loader, stderr)

			// Always print the result — even cancelled paths produce
			// the user's original input as Result.
			if result != nil {
				PrintResult(stdout, result)
			}

			// Shutdown signals fx to begin teardown. The exit code
			// reported back here is forced to 0 to honour the widget
			// contract.
			code := HandleError(stderr, err)
			if shErr := shutdowner.Shutdown(fx.ExitCode(code)); shErr != nil {
				return fmt.Errorf("shutdown: %w", shErr)
			}
			return nil
		},
	})
}
