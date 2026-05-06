# design/60-distribution — npm + homebrew release pipeline

> Spec: [spec/70](../spec/70-distribution.md)

## npm shape (esbuild-style)

```
npm-workspace/                                 (git-tracked)
├─ pnpm-workspace.yaml
├─ packages/
│   └─ zsh-history-enquirer/
│       ├─ package.json                        (top-level wrapper)
│       ├─ bin/cli.js                          (platform detect → exec)
│       ├─ install.js                          (warn-only, no .zshrc edit)
│       ├─ plugin/zsh-history-enquirer.plugin.zsh  (the widget)
│       └─ README.md
└─ templates/                                  (rendered into platform pkgs)
    └─ platform/
        ├─ package.json.tmpl
        ├─ README.md.tmpl
        └─ bin/.gitkeep
```

At release time, `scripts/release/build-npm.sh` walks the platform list
(`darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`) — every
binary is built with `CGO_ENABLED=0` so the same `linux-*` artifact
works on glibc, musl (Alpine, OpenWrt), and uclibc distros — and for
each:

1. Renders `templates/platform/package.json.tmpl` with `{{ os, cpu,
   version, name = @zsh-history-enquirer/{os}-{cpu} }}`.
2. Renders the README.
3. Copies `bin/zsh-history-enquirer-<os>-<cpu>` into `bin/zsh-history-enquirer`
   inside the rendered package.
4. `cd npm-workspace/build/<os>-<cpu> && npm publish --access public`.

The top-level `zsh-history-enquirer` package's `package.json` has its
`optionalDependencies` map built up the same way — each platform pkg
listed at the same version. It is published *last*, after all platform
packages are live, so a user who installs while the release is in flight
either gets the previous version (consistent) or the new version with
all deps resolvable.

## install.js

The install hook in the top-level package only:

1. Detects platform/arch.
2. Verifies that the matching `@zsh-history-enquirer/<os>-<cpu>` is
   resolvable (i.e. `optionalDependencies` did its job).
3. Prints a one-line "Installed. Add `source $(...)/plugin.zsh` to your
   `~/.zshrc` to enable" tip.

It **does not** edit `.zshrc`. The legacy port did and was a source of
support pain; the Go port refuses to.

## bin/cli.js

```js
#!/usr/bin/env node
const path = require('node:path');
const { spawnSync } = require('node:child_process');
const platform = `${process.platform}-${process.arch}`;     // darwin-arm64 etc.
const pkg = `@zsh-history-enquirer/${platform}`;
const binPath = require.resolve(`${pkg}/bin/zsh-history-enquirer`);
const r = spawnSync(binPath, process.argv.slice(2), { stdio: 'inherit' });
process.exit(r.status ?? 0);
```

This shim is ~12 lines. It does not load any third-party modules.

## Homebrew formula

The release CI step also opens a PR against `zthxxx/homebrew-tap`,
rewriting `Formula/zsh-history-enquirer.rb`. The script
`scripts/ci/bump-homebrew-tap.sh` is a near-copy of the `hams` version
— see [`hams/scripts/ci/bump-homebrew-tap.sh`](https://github.com/zthxxx/hams/blob/main/scripts/ci/bump-homebrew-tap.sh)
for the source pattern.

The formula installs:

- `bin/zsh-history-enquirer` (the binary)
- `share/zsh-history-enquirer/plugin.zsh` (the widget file)

Caveat: documentation tells users to add
`source $(brew --prefix)/share/zsh-history-enquirer/plugin.zsh`
to `~/.zshrc` themselves.

## Versioning

- One source of truth: the git tag (e.g. `v2.0.0`).
- `task ci:build` injects the version into the Go binary via
  `-ldflags "-X .../version.version=v2.0.0"`.
- The same value is written into the rendered `package.json`s by the
  release script.
- The Homebrew bump uses the same tag.

So a user running `zsh-history-enquirer --version` sees the same string
that's in `npm view`, `brew info`, and the GitHub Release title.
