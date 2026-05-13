# Example recordings

Rendered VHS recordings of representative picker flows. Linked
from `README.md` and `docs/design/` where helpful.

These are **documentation artifacts** — committed to the repo as
small `.mp4` / `.gif` files for browser previews and GitHub
README embedding. They are generated from the tape sources at
`e2e-v2/tapes/*.tape` via `task record:examples`.

## Regeneration

```sh
task record:examples              # all tapes
task record:examples TAPE=01      # only matching tapes
```

The Taskfile target uses the official Charmbracelet VHS docker
image so output is reproducible across macOS and Linux. After
rendering, copy the chosen `e2e-v2/tapes/out/<name>.{mp4,gif}`
into this directory and commit. Skip GIFs over ~500 KB unless
they're hero loops — prefer MP4 for size + quality.

## Current recordings

| File | Source tape | Shows |
| ---- | ----------- | ----- |
| (none yet) | `e2e-v2/tapes/01-basic-pick.tape` | substring filter + submit |
| (none yet) | `e2e-v2/tapes/09-multiline-submit.tape` | multi-line entry render + submit |
| (none yet) | `e2e-v2/tapes/16-narrow-wrap.tape` | 40-col wrap behaviour |

The table is updated when each recording is rendered and
committed. CI does not gate on file presence — these are
human-curated assets, not test outputs.
