# design/10-fx-graph — DI wiring

## Module list (top down)

```
fx.New(
    app.Module,             // composes everything below
)
```

```go
// internal/app/module.go
var Module = fx.Module("app",
    fx.Provide(
        config.New,            // *config.Config (parsed argv + env)
        tty.NewDevTTY,         // *tty.TTY (opens /dev/tty; close on shutdown)
        cursor.NewProbe,       // cursor.Probe (DSR query with timeout)
        history.NewZshLoader,  // history.Loader interface
        keys.NewReader,        // keys.Reader streaming Events from tty
        ui.NewModel,           // *ui.Model (input, visible, idx, limit, …)
        ui.NewProgram,         // *tea.Program (with custom output writer)
    ),
    fx.Invoke(app.Run),        // entrypoint — runs the program, prints result
)
```

## Lifecycle hooks

`tty.NewDevTTY` registers an `OnStop` that:

1. Restores the original termios.
2. Disables bracketed paste if we enabled it.
3. Closes the `/dev/tty` fd.

`ui.NewProgram` registers an `OnStop` that:

1. Sends a `tea.Quit` if the program is still running.
2. Drains the channel.

This guarantees that a panic anywhere in the graph still leaves the
terminal in a sane state (no stuck raw-mode, no swallowed cursor).

## Test substitution

In tests we replace providers with fixtures:

```go
fx.New(
    fx.Replace(
        // unit tests
        history.FixtureLoader(testHistoryFile),
        tty.NewMemoryTTY(rows, cols),
        keys.NewScriptedReader(scripted),
    ),
    app.Module,
)
```

`fx.Replace` merges by type, so the production providers in `app.Module`
are overridden cleanly without forking the module graph.

## Visualisation

`task fx:graph` runs the binary with `FX_VISUALIZE=1`, which prints a
DOT description of the graph. CI archives this output as a build
artifact so PRs touching DI can be eyeballed quickly.
