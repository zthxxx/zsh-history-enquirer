#!/usr/bin/env bash
#
# scripts/release/build-npm.sh — render the per-platform npm
# packages from templates/platform/ into npm/build/, copy
# the matching Go binary into each, and (optionally) publish.
#
# Usage:
#   scripts/release/build-npm.sh <version>            # render only
#   scripts/release/build-npm.sh <version> --publish  # render + publish
#
# Inputs:
#   $1                      semver tag, with or without leading 'v'
#   bin/zsh-history-enquirer-<os>-<arch>  produced by `task build:all`
#
# Output:
#   npm/build/<os>-<arch>/    rendered platform package
#   npm/build/zsh-history-enquirer/  rendered umbrella with
#                                              correct optionalDependencies
#
# Design notes:
#   - We render into npm/build/ rather than into
#     npm/packages/ so the rendered output is gitignored
#     and re-creating from scratch never tracks junk.
#   - Each platform package's package.json declares `os` and `cpu`
#     in node's vocabulary (`x64`, `darwin`, ...) so npm's
#     optional-dependency resolver only installs the one matching
#     the host.
#   - The umbrella package's optionalDependencies map is regenerated
#     from this script at every release so its versions never lag
#     behind what we actually published.
#
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $(basename "$0") <version> [--publish]" >&2
  exit 2
fi

cd "$(dirname "$0")/../.."

VERSION="${1#v}"
PUBLISH=0
if [ "${2:-}" = "--publish" ]; then
  PUBLISH=1
fi

if [ -z "${VERSION}" ]; then
  echo "error: empty version" >&2
  exit 2
fi

WORKSPACE="npm"
TEMPLATE="${WORKSPACE}/templates/platform"
BUILD="${WORKSPACE}/build"
UMBRELLA_SRC="${WORKSPACE}/packages/zsh-history-enquirer"
BIN_SRC="bin"

PLATFORMS=(
  # node-os  node-cpu  go-os    go-arch
  "darwin   arm64    darwin   arm64"
  "darwin   x64      darwin   amd64"
  "linux    arm64    linux    arm64"
  "linux    x64      linux    amd64"
)

mkdir -p "${BUILD}"
# Defensive: refuse to expand to '/' if ${BUILD} is somehow empty.
# `rm -rf /*` would be catastrophic; the :? guard makes the shell
# error out instead.
rm -rf "${BUILD:?}"/*

render() {
  # render <src> <dst> <pairs...>
  local src="$1" dst="$2"
  shift 2
  local content
  content="$(cat "$src")"
  while [ "$#" -gt 0 ]; do
    local key="$1" val="$2"
    shift 2
    content="${content//\{\{${key}\}\}/${val}}"
  done
  printf '%s' "$content" > "$dst"
}

# ---- Per-platform packages ----------------------------------------
for platform in "${PLATFORMS[@]}"; do
  # shellcheck disable=SC2206
  parts=( $platform )
  NODE_OS="${parts[0]}"
  NODE_CPU="${parts[1]}"
  GO_OS="${parts[2]}"
  GO_ARCH="${parts[3]}"
  NPM_PLATFORM="${GO_OS}-${GO_ARCH}"

  pkg_dir="${BUILD}/${NPM_PLATFORM}"
  mkdir -p "${pkg_dir}/bin"

  render "${TEMPLATE}/package.json.tmpl" "${pkg_dir}/package.json" \
    NPM_PLATFORM "${NPM_PLATFORM}" \
    NODE_OS      "${NODE_OS}" \
    NODE_CPU     "${NODE_CPU}" \
    VERSION      "${VERSION}"

  render "${TEMPLATE}/README.md.tmpl" "${pkg_dir}/README.md" \
    NPM_PLATFORM "${NPM_PLATFORM}"

  cp "${TEMPLATE}/LICENSE.tmpl" "${pkg_dir}/LICENSE"

  binary="${BIN_SRC}/zsh-history-enquirer-${GO_OS}-${GO_ARCH}"
  if [ ! -f "${binary}" ]; then
    echo "error: missing binary ${binary}; run 'task build:all' first" >&2
    exit 1
  fi
  cp "${binary}" "${pkg_dir}/bin/zsh-history-enquirer"
  chmod +x "${pkg_dir}/bin/zsh-history-enquirer"

  echo "==> rendered ${pkg_dir}"
done

# ---- Umbrella package ---------------------------------------------
umbrella="${BUILD}/zsh-history-enquirer"
mkdir -p "${umbrella}/bin" "${umbrella}/plugin"
cp "${UMBRELLA_SRC}/bin/cli.js"  "${umbrella}/bin/cli.js"
# Source the plugin from the project root, not from a stale copy
# inside npm/packages/. Single source of truth: any plugin fix lands
# in plugin/ and flows through to the npm release at build time.
cp "plugin/zsh-history-enquirer.plugin.zsh" "${umbrella}/plugin/"
# README is short and npm-specific (the full README has too many
# images/badges to render well on npmjs.com); LICENSE comes from the
# project root so the legally-binding MIT text is always shipped.
cp "${UMBRELLA_SRC}/README.md"   "${umbrella}/README.md"
cp "LICENSE"                     "${umbrella}/LICENSE"
chmod +x "${umbrella}/bin/cli.js"

# Build optionalDependencies map.
opt_deps=""
for platform in "${PLATFORMS[@]}"; do
  # shellcheck disable=SC2206
  parts=( $platform )
  GO_OS="${parts[2]}"
  GO_ARCH="${parts[3]}"
  if [ -n "$opt_deps" ]; then opt_deps="${opt_deps},"; fi
  opt_deps="${opt_deps}\n    \"@zsh-history-enquirer/${GO_OS}-${GO_ARCH}\": \"${VERSION}\""
done

cat > "${umbrella}/package.json" <<EOF
{
  "name": "zsh-history-enquirer",
  "version": "${VERSION}",
  "description": "Replace zsh's Ctrl+R with an inline, multi-line, dedup'd, multi-word-fuzzy history picker. Static Go binary, esbuild-style cross-platform npm distribution.",
  "license": "MIT",
  "author": "zthxxx",
  "homepage": "https://github.com/zthxxx/zsh-history-enquirer",
  "repository": {
    "type": "git",
    "url": "git+https://github.com/zthxxx/zsh-history-enquirer.git"
  },
  "bugs": {
    "url": "https://github.com/zthxxx/zsh-history-enquirer/issues"
  },
  "publishConfig": {
    "access": "public",
    "registry": "https://registry.npmjs.org"
  },
  "keywords": [
    "zsh", "zsh-history", "history", "search", "history-search",
    "history-enhance", "plugin", "zsh-plugin"
  ],
  "engines": { "node": ">=18.0.0" },
  "main": "bin/cli.js",
  "bin": { "zsh-history-enquirer": "bin/cli.js" },
  "scripts": {
    "postinstall": "node bin/cli.js --print-install-hint || true"
  },
  "files": [
    "bin/cli.js",
    "plugin/zsh-history-enquirer.plugin.zsh",
    "README.md",
    "LICENSE"
  ],
  "optionalDependencies": {$(printf '%b' "${opt_deps}")
  }
}
EOF

echo "==> rendered ${umbrella}"

# ---- Publish (optional) -------------------------------------------
if [ "${PUBLISH}" -eq 1 ]; then
  # Decide the npm dist-tag from the version string. Pre-release
  # versions (containing "-", e.g. 1.0.0-rc.1) publish under
  # `next` so `npm install zsh-history-enquirer@latest` keeps
  # serving the previous stable. Stable versions take `latest`.
  # Matches the description in release.yml's publish-npm job.
  if [[ "${VERSION}" == *-* ]]; then
    NPM_TAG="next"
  else
    NPM_TAG="latest"
  fi

  echo ""
  echo "==> Publishing platform packages first, umbrella last (dist-tag: ${NPM_TAG})"
  for platform in "${PLATFORMS[@]}"; do
    # shellcheck disable=SC2206
    parts=( $platform )
    GO_OS="${parts[2]}"
    GO_ARCH="${parts[3]}"
    NPM_PLATFORM="${GO_OS}-${GO_ARCH}"
    pkg_dir="${BUILD}/${NPM_PLATFORM}"
    echo ""
    echo "---- @zsh-history-enquirer/${NPM_PLATFORM}@${VERSION} (tag: ${NPM_TAG})"
    (cd "${pkg_dir}" && npm publish --access public --tag "${NPM_TAG}")
  done

  echo ""
  echo "---- zsh-history-enquirer@${VERSION} (tag: ${NPM_TAG})"
  (cd "${umbrella}" && npm publish --access public --tag "${NPM_TAG}")
fi

echo ""
echo "==> done"
