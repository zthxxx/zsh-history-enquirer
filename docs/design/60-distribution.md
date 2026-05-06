# design/60-distribution — npm + homebrew release pipeline

> Spec: [spec/70](../spec/70-distribution.md)

## Repo shape (esbuild-style)

```
pnpm-workspace.yaml                              (at repo root)

npm/                                             (git-tracked)
├─ packages/
│   └─ zsh-history-enquirer/                     (umbrella source)
│       ├─ package.json
│       ├─ bin/cli.js                            (platform detect → exec)
│       ├─ README.md                             (npm-specific short README)
│       └─ LICENSE                               (canonical MIT text)
└─ templates/                                    (per-platform package source)
    └─ platform/
        ├─ package.json.tmpl
        ├─ README.md.tmpl
        ├─ LICENSE.tmpl                          (canonical MIT text)
        └─ bin/                                  (.gitkeep dir; binary lands at render)
```

The plugin file lives at the project root (`plugin/zsh-history-enquirer.plugin.zsh`)
and is **not** stored under `npm/packages/`. It is copied into the
rendered umbrella at release time so there's a single source of
truth — see [the followups note about the previous stale duplicate](../plan/20-followups.md).

`npm/build/` (gitignored) is the render output:

```
npm/build/
├─ zsh-history-enquirer/                         (rendered umbrella)
│   ├─ package.json                              (with optionalDependencies)
│   ├─ bin/cli.js
│   ├─ plugin/zsh-history-enquirer.plugin.zsh    (copied from project root)
│   ├─ README.md
│   └─ LICENSE
├─ darwin-arm64/                                 (rendered platform package)
├─ darwin-amd64/
├─ linux-arm64/
└─ linux-amd64/
```

At release time, `scripts/release/build-npm.sh` walks the platform
list (`darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`)
— every binary is built with `CGO_ENABLED=0` so the same `linux-*`
artifact works on glibc, musl (Alpine, OpenWrt), and uclibc — and
for each:

1. Renders `templates/platform/package.json.tmpl` with `{NPM_OS,
   NPM_CPU, NPM_PLATFORM, VERSION, name = @zsh-history-enquirer/<os>-<arch>}`.
2. Renders the README and LICENSE.
3. Copies `bin/zsh-history-enquirer-<os>-<arch>` into
   `bin/zsh-history-enquirer` inside the rendered package.
4. With `--publish`: `cd npm/build/<os>-<arch> && npm publish --access public`.

The umbrella's `optionalDependencies` map is built the same way:
each platform package listed at the same version. The umbrella is
published **last**, after all platform packages are live, so a user
who installs while the release is in flight either gets the
previous version (consistent) or the new version with all deps
resolvable.

## install.js / postinstall

The umbrella package's `package.json` declares:

```json
"scripts": {
  "postinstall": "node bin/cli.js --print-install-hint || true"
}
```

After `npm install -g zsh-history-enquirer`, npm runs the postinstall
script, which re-enters `bin/cli.js` with `--print-install-hint`. The
shim then prints (to stderr):

```
  zsh-history-enquirer installed.

  Add this line to your ~/.zshrc to enable the Ctrl+R picker:

    source <package-root>/plugin/zsh-history-enquirer.plugin.zsh
```

The `|| true` guard ensures that any failure in the hint path
(extremely rare) does not abort the install.

It **does not** edit `.zshrc`. The legacy Node.js port did edit it
unconditionally and was a source of support pain (re-install loops,
lost user edits); the Go port refuses to.

## bin/cli.js

The shim is ~95 lines, all in `npm/packages/zsh-history-enquirer/bin/cli.js`.
It does not load any third-party modules. Three responsibilities:

1. Resolve the platform sub-package by mapping
   `process.platform`/`process.arch` to the npm vocabulary (`darwin`,
   `linux`, `amd64`, `arm64`). Returns `null` for unsupported
   combinations (e.g. freebsd, arm32).
2. `--print-install-hint`: print the source-line hint to stderr
   (called by the postinstall script).
3. Default path: `require.resolve` the platform binary and spawn it
   with the same argv.

Critical edge case: when `require.resolve` fails (no platform
sub-package installed for the host), the shim **echoes argv back to
stdout** before exiting 0. This is the widget contract — if the
widget's `BUFFER=$(...)` falls through to a missing-binary path, the
typed input must not vanish. The fallback path is covered by
`task release:smoke` (see [design/70](./70-testing.md)).

## Homebrew formula

The release CI step opens a PR against `zthxxx/homebrew-tap`,
rewriting `Formula/zsh-history-enquirer.rb`. The script
`scripts/ci/bump-homebrew-tap.sh` is patterned after the `hams`
version.

Per-release the script:

1. Pulls `checksums.txt` from the GitHub Release (with retry — the
   asset can take up to 30 s to become reachable from the API host).
2. Extracts SHA256s for the 4 binary artifacts plus
   `zsh-history-enquirer.plugin.zsh`.
3. Either rewrites version+sha lines in the existing formula (if it
   already has the `resource "plugin"` stanza) **or** regenerates
   the formula from a heredoc template (legacy formulas without the
   plugin resource — first migration from the v1.x layout).
4. Force-pushes a `bump-zsh-history-enquirer-v<version>` branch and
   opens a PR via `gh`.

The formula installs:

- `bin/zsh-history-enquirer` (the binary).
- `share/zsh-history-enquirer/plugin.zsh` (the widget file, fetched
  from the release as a `resource "plugin"`).

Documentation tells users to source the plugin themselves:

```bash
source "$(brew --prefix)/share/zsh-history-enquirer/plugin.zsh"
```

The formula does **not** auto-edit `~/.zshrc`. A `caveats` block
prints the source-line hint after install, mirroring the npm
postinstall behaviour.

## Versioning

- One source of truth: the git tag (e.g. `v2.0.0`).
- `task ci:build` injects the version into the Go binary via
  `-ldflags "-X .../version.version=v2.0.0"`.
- The same value is written into the rendered `package.json`s by the
  release script.
- The Homebrew bump uses the same tag.

So a user running `zsh-history-enquirer --version` sees the same string
that's in `npm view`, `brew info`, and the GitHub Release title.
