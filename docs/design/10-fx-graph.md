# design/10-fx-graph — DI wiring

## Module list (top down)

```go
// cmd/zsh-history-enquirer/main.go
fx.New(
    app.Module,             // composes everything below
    fx.NopLogger,           // silence fx; the widget contract requires
                            // stderr to stay quiet.
    fx.StartTimeout(5*time.Second),
    fx.StopTimeout(5*time.Second),
)
```

```go
// internal/app/module.go
var Module = fx.Module("app",
    fx.Provide(
        // config: parses os.Args[1:] into a *Config.
        func() (*Config, error) {
            return NewConfig(os.Args[1:], os.Stderr)
        },

        // Named writer types so the fx graph can disambiguate
        // io.Writer providers.
        func() Stdout       { return os.Stdout },
        func() StderrWriter { return os.Stderr },

        // /dev/tty open + close-on-shutdown.
        tty.NewDevTTY,

        // history loader; FixtureLoader is substituted in tests.
        func(cfg *Config) history.Loader {
            return history.NewZshLoader(history.Options{
                HistFile: cfg.HistFile,
                HistSize: cfg.HistSize,
            })
        },
    ),

    // Single Invoke runs the picker synchronously inside the
    // fx OnStart hook (see invokeRun in module.go).
    fx.Invoke(invokeRun),
)
```

There is **no** `*tea.Program` provider. We do not use bubbletea
(see [design/00](./00-architecture.md) for the rationale). The
event loop is in `internal/app/loop.go`, calling pure functions in
`internal/ui/`.

## Named-type disambiguation

`Stdout` and `StderrWriter` are distinct named types both wrapping
`io.Writer`:

```go
type Stdout       io.Writer
type StderrWriter io.Writer
```

Without these names, fx refused to resolve which of the two
`io.Writer` providers should satisfy each parameter — silent
failure during graph construction. The named types make the
binding explicit at the type level.

## Lifecycle hooks

`tty.NewDevTTY` registers an `OnStop` that:

1. If raw mode was entered, restores the original termios.
2. Closes the `/dev/tty` fd.

`invokeRun` registers an `OnStart` that:

1. Calls `Run(ctx, cfg, t, loader, stderr)` synchronously (the
   picker runs in the start hook so fx's stop sequence — TTY
   cleanup — does not race the terminal print).
2. Always prints the result (even canceled paths produce the
   user's typed input as Result).
3. Calls `shutdowner.Shutdown(fx.ExitCode(0))` so the binary
   always exits 0 — widget contract.

This guarantees that a panic anywhere in the graph still leaves
the terminal in a sane state (no stuck raw-mode, no swallowed
cursor).

## Test substitution

`internal/app/module_test.go` builds a smaller graph that
overrides specific providers via `fx.Replace`. Two main paths:

1. Replace the loader with `history.FixtureLoader(path)` so unit
   tests do not require zsh on the host.
2. Replace `*tty.TTY` via `tty.NewFromFile(slave)` against a
   `creack/pty` slave, so reader / cursor-probe tests can drive
   bytes from the master end.

`fx.Replace` merges by type, so the production providers in
`app.Module` are overridden cleanly without forking the module
graph.

## Visualisation

`fx.Visualize` is available but not wired into a Task. Use:

```bash
go run ./cmd/zsh-history-enquirer --version  # exits before fx.New
```

— or write a one-off `fx.New(app.Module, fx.Visualize(&out))` in
a debug script. There's no CI step to archive a DOT graph; the
graph is small enough (5 providers + 1 invoke) to read directly
from `internal/app/module.go`.
