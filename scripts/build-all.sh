#!/usr/bin/env bash
#
# scripts/build-all.sh — cross-compile zsh-history-enquirer for every
# supported platform.
#
# Targets:
#   darwin/arm64 darwin/amd64
#   linux/arm64  linux/amd64
#
# All targets are built with CGO_ENABLED=0 → the output is a fully
# static binary that runs on glibc, musl (Alpine, OpenWrt), and uclibc
# without recompilation. There is no per-libc variant.
#
# Output files: bin/zsh-history-enquirer-<os>-<arch>
#
set -euo pipefail

cd "$(dirname "$0")/.."

APP_NAME="zsh-history-enquirer"
BUILD_DIR="bin"
PKG_PATH="github.com/zthxxx/zsh-history-enquirer"
CMD_PATH="./cmd/zsh-history-enquirer"

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
# DATE uses the HEAD commit's ISO-8601 timestamp rather than wall-clock
# build time, so a release tag produces byte-identical binaries across
# CI re-runs (must agree with Taskfile.yml's host-build DATE; both
# surfaces converge on the commit timestamp).
DATE="${DATE:-$(git log -1 --format=%cI HEAD 2>/dev/null || date -u '+%Y-%m-%dT%H:%M:%SZ')}"

LDFLAGS="-s -w \
  -X ${PKG_PATH}/pkg/version.version=${VERSION} \
  -X ${PKG_PATH}/pkg/version.commit=${COMMIT} \
  -X ${PKG_PATH}/pkg/version.date=${DATE}"

mkdir -p "${BUILD_DIR}"

PLATFORMS=(
  "darwin/arm64"
  "darwin/amd64"
  "linux/arm64"
  "linux/amd64"
)

for platform in "${PLATFORMS[@]}"; do
  GOOS="${platform%%/*}"
  GOARCH="${platform##*/}"
  out="${BUILD_DIR}/${APP_NAME}-${GOOS}-${GOARCH}"
  echo "==> ${out}"
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -trimpath -ldflags "${LDFLAGS}" -o "${out}" "${CMD_PATH}"

  # Verify the linux artifacts are statically linked. macOS binaries
  # always link against /usr/lib/dyld, which is part of the OS ABI,
  # so we skip the assertion there.
  if [ "${GOOS}" = "linux" ]; then
    if file "${out}" | grep -q 'dynamically linked'; then
      echo "error: ${out} is dynamically linked — CGO leaked into the build" >&2
      exit 1
    fi
  fi
done

echo "==> all platforms built"
