# design/80-release-process — concrete release wiring

> Spec: [spec/80-release-process.md](../spec/80-release-process.md)
> Companion: [design/60-distribution.md](./60-distribution.md)

The release pipeline is one GitHub Actions workflow
(`.github/workflows/release.yml`) with three sequential job groups:

```
push tag vX.Y.Z
   │
   ▼
build  ─── matrix: darwin/{arm64,amd64}, linux/{arm64,amd64}
   │       runs `task ci:build` per (GOOS, GOARCH)
   │       uploads artifact `zsh-history-enquirer-<os>-<arch>`
   ▼
release  download all 4 artifacts, run `task ci:release:package`,
   │     create GitHub Release with files + checksums.txt
   ▼
publish-npm   ┐
              │ both depend on `release`; both pull binaries from
              │ artifacts; run in parallel.
bump-homebrew-tap ┘
```

## build job

`task ci:build` reads `GOOS`, `GOARCH` from env, sets
`CGO_ENABLED=0`, links with `-ldflags "-s -w -X .../version.version=VERSION ..."`,
emits `bin/zsh-history-enquirer-<os>-<arch>`.

The static-linkage assertion in `scripts/build-all.sh` is **not**
enforced here per-target — that script is for the all-at-once local
build. The release CI relies on `CGO_ENABLED=0` + the Go toolchain's
guarantees, which are sufficient.

## release job

Downloads all 4 artifacts into `dist/<artifact-name>/<binary>`, then
`task ci:release:package`:

```
mkdir -p release
find dist -type f -name 'zsh-history-enquirer-*' -exec cp {} release/ \;
# The plugin file ships as a separate release asset so the Homebrew
# formula's `resource "plugin"` stanza can fetch it.
cp plugin/zsh-history-enquirer.plugin.zsh release/
cd release && (sha256sum zsh-history-enquirer-* zsh-history-enquirer.plugin.zsh 2>/dev/null
                 || shasum -a 256 zsh-history-enquirer-* zsh-history-enquirer.plugin.zsh) > checksums.txt
```

Five files end up in `release/` after this step: 4 binaries plus
the plugin file. `checksums.txt` covers all of them (passed by
name rather than `*` glob so `checksums.txt` itself does not get
its own line as we write to it).

Checksums fall back to `shasum -a 256` for non-Linux runners (the
CI matrix is ubuntu-latest, so `sha256sum` works; the fallback
exists for parity with `task build:release` on macOS dev machines).

`softprops/action-gh-release@v2` then uploads `release/*` and
auto-generates release notes. The `prerelease:` field is computed
as `contains(github.ref_name, '-')` — semver pre-release tags
(`v1.0.0-rc.1`) flag as pre-release; stable tags (`v1.0.0`) don't.

## publish-npm job

Re-downloads artifacts (the dependency on `release` is for ordering,
not data), packages them into `release/`, copies the four binaries
into `bin/zsh-history-enquirer-<os>-<arch>`, then runs
`scripts/release/build-npm.sh "${VERSION}" --publish` which:

1. For each platform:
   - Renders `npm/templates/platform/{package.json,README.md,LICENSE}.tmpl`
     into `npm/build/<os>-<arch>/` with substitutions for
     `{{NPM_PLATFORM}}`, `{{NODE_OS}}`, `{{NODE_CPU}}`,
     `{{VERSION}}`.
   - Copies the matching binary into `bin/zsh-history-enquirer`.
   - `npm publish --access public`.
2. Renders the umbrella package (`npm/build/zsh-history-enquirer/`)
   with the four `optionalDependencies` resolving to the just-
   published versions, then `npm publish --access public`.

The umbrella publishes **last** so a user installing during the
release window either gets the previous version (consistent) or the
new version with all sub-packages already live.

`NPM_TOKEN` is required as a GitHub secret with `automation` scope
on `zsh-history-enquirer` and `@zsh-history-enquirer/*`.

## bump-homebrew-tap job

Runs `bash scripts/ci/bump-homebrew-tap.sh "${VERSION}"`, adapted
from [hams/scripts/ci/bump-homebrew-tap.sh](https://github.com/zthxxx/hams).
The script:

1. Downloads `${RELEASE_BASE}/checksums.txt` with retry loop (CDN
   sometimes lags 5–10s behind the release event).
2. Extracts the four sha256s with `awk`.
3. Clones `zthxxx/homebrew-tap` using `HOMEBREW_TAP_TOKEN` as the
   PAT (default `GITHUB_TOKEN` cannot reach a different repo).
4. Checks out a deterministic branch `bump-zsh-history-enquirer-vX.Y.Z`
   based on `origin/main` — re-runs amend rather than duplicate.
5. Rewrites `Formula/zsh-history-enquirer.rb` via Python sed-like
   substitution (`re.sub` with capture groups) for the four
   `sha256` lines + the top-level `version`.
6. Force-with-leases the bump branch and `gh pr create`s.

The job's `if: !contains(github.ref_name, '-')` skips pre-release
tags from Homebrew — Homebrew users only see stable versions; early
adopters install pre-releases via `npm install zsh-history-enquirer@next`.

## Idempotency guarantees

| Step | Re-runnable? | Failure mode |
| --- | --- | --- |
| `build` | yes | Same artifact name → upload-artifact replaces. |
| `release` | yes | `softprops/action-gh-release@v2` upserts the release; existing files are overwritten. |
| `publish-npm` (per platform) | NO | npm rejects re-publishing the same version. Re-running requires a `vX.Y.Z+1` tag. |
| `bump-homebrew-tap` | yes | The branch name is deterministic; force-push amends rather than duplicates. |

The non-idempotent step is `publish-npm`. If a release fails *after*
publishing some platforms but before all, the only remediation is
to bump the patch version and tag again. This is the same trade-off
esbuild and biome make with the same pattern.

## Local dry-run

```bash
task build:all
bash scripts/release/build-npm.sh 2.0.0-test   # render only, no --publish
ls npm/build/zsh-history-enquirer/             # umbrella
ls npm/build/darwin-arm64/                     # one platform
```

Inspect the rendered `package.json`s; verify `os`/`cpu` and
`optionalDependencies` are right. The actual publish requires
`NPM_TOKEN` and is not run locally.
