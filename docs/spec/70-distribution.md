# spec/70-distribution — how users get the binary

## Build constraints

The Go binary must:

- Be **statically linked**. No `libc` / `libSystem` dynamic dependency,
  no `glibc`/`musl` ABI requirement. This is achieved by building
  with `CGO_ENABLED=0` and only pure-Go imports.
- Run unchanged on each of these targets:

  | OS family | Architectures we ship |
  | --- | --- |
  | macOS (darwin) | `arm64`, `amd64` |
  | Debian / Ubuntu / glibc Linux | `arm64`, `amd64` |
  | Alpine / musl Linux | `arm64`, `amd64` |
  | Arch / glibc Linux | `arm64`, `amd64` |
  | OpenWrt / musl Linux | `arm64`, `amd64` |

Static linking makes the same `linux-arm64` artifact run on glibc and
musl distros alike, which is the only sane way to ship to OpenWrt.

## npm (primary)

The npm package `zsh-history-enquirer` is the user-facing entrypoint.
Its `bin` field is a small JavaScript shim that:

1. Detects the host platform / arch.
2. Locates the binary inside the matching `@zsh-history-enquirer/<os>-<arch>`
   sub-package (installed via `optionalDependencies`).
3. `execve`s the binary with the same argv.

This is the same shape `esbuild` uses; it works because npm's
`optionalDependencies` are installed only on matching platforms, leaving
the user with exactly one platform binary and ~50 KB of overhead.

The platform packages are **not** committed to git. They are
rendered at release time from a template in `npm/templates/platform/`.
The release CI flow:

1. `task ci:build` (matrix over GOOS/GOARCH) cross-compiles every
   target into an artifact named `zsh-history-enquirer-<os>-<arch>`.
2. The release job downloads all 4 artifacts, runs
   `task ci:release:package` to flatten into `release/` and append
   `checksums.txt`, then uploads as a GitHub Release.
3. The publish-npm job:
   - For each platform: render the template into
     `npm/build/<os>-<arch>/`, copy the binary in, set the correct
     `os`/`cpu` fields, bump the version, and `npm publish`.
   - Render and publish the umbrella under `npm/build/zsh-history-enquirer/`
     with a matching version and the correct `optionalDependencies`
     map. The umbrella publishes **last**, after all platform
     packages are live.

The umbrella source (`npm/packages/zsh-history-enquirer/`) and the
template directory (`npm/templates/platform/`) **are** committed
to git so the CI step is deterministic. The render output
(`npm/build/`) is gitignored.

## Homebrew (secondary)

A release tag triggers a workflow that:

1. Builds all four targets — `darwin-{arm64,amd64}` and
   `linux-{arm64,amd64}`. Homebrew picks the right one at install
   time via `on_macos`/`on_linux` + `Hardware::CPU` checks.
2. Uploads them to a GitHub Release with `checksums.txt` (5 entries:
   the four binaries plus `zsh-history-enquirer.plugin.zsh`).
3. Opens a PR against `zthxxx/homebrew-tap` rewriting
   `Formula/zsh-history-enquirer.rb` with the new version + four
   binary sha256s + the plugin sha256. The PR shape mirrors the
   `hams` formula bumper.

## Plugin file location

The plugin file (`zsh-history-enquirer.plugin.zsh`) is shipped with both
distributions:

- npm: under `<package>/plugin/zsh-history-enquirer.plugin.zsh`.
- homebrew: into `#{opt_share}/zsh-history-enquirer/plugin.zsh`. The
  formula's `resource "plugin"` block downloads the file from the
  same GitHub Release that supplies the binary, then installs it
  into pkgshare. Users source it via
  `source "$(brew --prefix)/share/zsh-history-enquirer/plugin.zsh"`.

Users source it themselves. The Go port deliberately does **not**
auto-modify `.zshrc` or oh-my-zsh's plugin list (the legacy did, and
that caused subtle re-install loops and unwanted edits).

## Install instructions (user-facing)

The README documents three flows. Each flow ends with one source line in
`.zshrc`. None of the flows write to the user's shell config without
permission.
