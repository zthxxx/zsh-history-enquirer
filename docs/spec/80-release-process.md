# spec/80-release-process — how a release is cut

> **Spec layer** — describes the *user-visible* effect of a release.
> Concrete YAML / shell wiring lives in `docs/design/60-distribution.md`.

## Trigger

A release is triggered by **pushing a `v*` annotated tag** to
`master`. Pre-release semver markers (`v1.0.0-rc.1`, `v1.0.0-alpha.2`,
`v1.0.0-beta.3`) are recognised; they go to npm with `next` dist-tag
and skip the Homebrew bump.

Tags must be:

- Annotated (`git tag -a v1.0.0 -m "..."`), not lightweight.
- Created on `master`. Tags on feature branches are ignored by the
  release workflow.
- Strictly increasing semver. No re-tagging an already-published
  version.

## What a release produces

For tag `vX.Y.Z`:

1. **GitHub Release** at the tag with:
   - The four cross-platform binaries
     (`zsh-history-enquirer-{darwin,linux}-{arm64,amd64}`).
   - `zsh-history-enquirer.plugin.zsh` (the widget file, separate
     asset so the Homebrew formula's `resource "plugin"` stanza
     can fetch it).
   - `checksums.txt` (sha256, one entry per asset — 5 lines).
   - Auto-generated release notes from PR titles since the previous
     tag.
   - Pre-release flag if the tag contains `-`.
2. **npm packages**:
   - The four platform sub-packages
     (`@zsh-history-enquirer/<os>-<arch>`) with the binary in `bin/`
     and the right `os` / `cpu` constraints.
   - The umbrella `zsh-history-enquirer` with
     `optionalDependencies` pointing at the four sub-packages at
     the same version.
   - Stable releases publish under `latest`; pre-releases under
     `next`.
3. **Homebrew tap PR** (stable releases only):
   - A PR against `zthxxx/homebrew-tap` rewriting
     `Formula/zsh-history-enquirer.rb` with the new version, four
     per-platform sha256s, and the plugin-file sha256.
   - Formula's `def install` stages the plugin into `pkgshare`
     (`share/zsh-history-enquirer/plugin.zsh`) so users source it
     via `$(brew --prefix)/share/...`. Without this stanza the
     README path would be a 404.
   - Branch name `bump-zsh-history-enquirer-vX.Y.Z` so re-runs
     amend the same PR rather than duplicating.

## Failure invariants

If any step fails, the release is **partially published**. The user
will see:

- GitHub Release exists, but a platform's npm package is missing.
- npm umbrella references a sub-package version that 404s.
- Homebrew formula points at a release that has no checksum.

Mitigation: each release step is **idempotent**. A re-run with the
same tag publishes any missing artifacts and overwrites diverged
ones (npm's `--force` is *not* used; the script aborts and asks for
human intervention if a version conflict is detected).

## How to cut a release

1. Land all merges to `master`.
2. Update `CHANGELOG.md` — move `[Unreleased]` heading content into
   a new `[X.Y.Z] — YYYY-MM-DD` section. Commit.
3. `git tag -a vX.Y.Z -m "vX.Y.Z" && git push origin vX.Y.Z`.
4. Watch [GitHub Actions](https://github.com/zthxxx/zsh-history-enquirer/actions/workflows/release.yml).
   The pipeline takes ~5–8 minutes.
5. After release: verify `npm view zsh-history-enquirer@latest
   version`, `brew info zsh-history-enquirer`, and the
   `zthxxx/homebrew-tap` PR.

## Required GitHub secrets

| Secret | Used by | Notes |
| --- | --- | --- |
| `NPM_TOKEN` | `publish-npm` job | npm automation token with publish scope on `zsh-history-enquirer` and `@zsh-history-enquirer/*`. |
| `HOMEBREW_TAP_TOKEN` | `bump-homebrew-tap` job | PAT with `repo` scope on `zthxxx/homebrew-tap`. The default `GITHUB_TOKEN` cannot reach another repo. |

`GITHUB_TOKEN` (always provided) is enough for the release-asset
upload job.

## What is NOT done at release time

- No automatic `~/.zshrc` edit. Users source the plugin file
  themselves; the legacy v1 port did this and the rewrite refuses to.
- No oh-my-zsh plugin-list mutation.
- No browser open / GitHub Pages deploy / coverage upload. Those
  belong to the push-CI workflow if at all.
