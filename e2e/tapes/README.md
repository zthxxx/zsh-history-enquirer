# VHS recording tapes

Human-watchable MP4 / GIF recordings of the picker for documentation
and README embedding. **These are NOT automated tests** — the
assertion-bearing scenarios live in `e2e/scenarios/*_test.go`.

## Why they exist

- README hero loop and feature demos
- Visual regression *review* (eyeball, not assertion) when a
  renderer change lands
- Onboarding: a 60-second tape says more than a 200-line `.exp`

## Running

```sh
task record:examples            # render every *.tape under tapes/
task record:examples TAPE=01    # render only matching tape(s)
```

The Taskfile target is **not** part of `task check` / `task ci`.

## Output

`tapes/out/<name>.mp4` and `tapes/out/<name>.gif`. The `out/`
directory is gitignored — recordings regenerate on demand. For
docs publishing, copy the chosen file into `docs/examples/`.

## Authoring conventions

- One `.tape` per scenario worth showcasing. Mirror the Go scenario
  filename where applicable: `01-basic-pick.tape` ↔ `TestBasicPick`.
- Start every tape with the shared header (see `_header.tape.snippet`)
  so font, theme, and dimensions stay consistent.
- Slow keystroke pacing on purpose — `Sleep 200ms` between keys
  reads like a human, `Sleep 50ms` reads like a script.
- `Hide` / `Show` brackets hide picker-setup typing (sourcing the
  plugin) so the recording shows only the user-visible flow.

## How VHS interacts with our binary

VHS launches its own internal terminal (ghostty) running a shell
(`Set Shell zsh`). The tape:

1. `Type "source /path/to/plugin.zsh"` + Enter — wires `^R`
2. Optionally pre-seeds `$HISTFILE` with a fixture
3. The visible part: types, presses `^R`, interacts with the picker

The picker binary itself must be on `$PATH` inside VHS's runtime.
Use the official VHS docker image for reproducibility:

```sh
docker run --rm -v "$PWD:/vhs" ghcr.io/charmbracelet/vhs <tape>
```

The image already includes ghostty, ttyd, ffmpeg, and node — VHS
needs all four.
