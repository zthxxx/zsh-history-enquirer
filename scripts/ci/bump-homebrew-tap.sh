#!/usr/bin/env bash
#
# scripts/ci/bump-homebrew-tap.sh — bump the zsh-history-enquirer
# Formula in zthxxx/homebrew-tap.
#
# Triggered by the release workflow (.github/workflows/release.yml)
# after a successful GitHub Release publish. Downloads the release's
# checksums.txt, rewrites the per-platform sha256 fields and the
# top-level version in Formula/zsh-history-enquirer.rb, then opens a
# pull request against zthxxx/homebrew-tap.
#
# Usage:
#   scripts/ci/bump-homebrew-tap.sh <version>
#
# Arguments:
#   <version>   Semantic version with optional leading "v"
#               (e.g. "2.0.0" or "v2.0.0").
#
# Environment:
#   HOMEBREW_TAP_TOKEN   Required. PAT with `repo` scope on
#                        zthxxx/homebrew-tap. The CI runner's default
#                        GITHUB_TOKEN cannot reach another repo, so a
#                        dedicated secret is mandatory.
#   GITHUB_REPOSITORY    Optional. Source repo, defaults to
#                        zthxxx/zsh-history-enquirer; used only in PR
#                        body links.
#
# Exit codes:
#   0   success (PR opened or already in sync)
#   1   runtime error (missing args, fetch failure, bad checksums, ...)
set -euo pipefail

if [ "$#" -lt 1 ]; then
  echo "usage: $(basename "$0") <version>" >&2
  exit 1
fi

VERSION="${1#v}"

if [ -z "${VERSION}" ]; then
  echo "error: empty version" >&2
  exit 1
fi

: "${HOMEBREW_TAP_TOKEN:?HOMEBREW_TAP_TOKEN is required (PAT with repo scope on zthxxx/homebrew-tap)}"

SOURCE_REPO="${GITHUB_REPOSITORY:-zthxxx/zsh-history-enquirer}"
TAP_REPO="zthxxx/homebrew-tap"
BRANCH="bump-zsh-history-enquirer-v${VERSION}"
RELEASE_BASE="https://github.com/${SOURCE_REPO}/releases/download/v${VERSION}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "==> Fetching checksums from ${RELEASE_BASE}/checksums.txt"
attempts=0
max_attempts=6
until curl -fsSL -o "${work}/checksums.txt" "${RELEASE_BASE}/checksums.txt"; do
  attempts=$((attempts + 1))
  if [ "${attempts}" -ge "${max_attempts}" ]; then
    echo "error: checksums.txt still 404 after ${max_attempts} attempts" >&2
    exit 1
  fi
  echo "    attempt ${attempts} failed; retrying in 5s"
  sleep 5
done

sha_for() {
  local filename="$1"
  awk -v f="$filename" '$2 == f { print $1; found=1 } END { exit !found }' "${work}/checksums.txt"
}

SHA_DARWIN_ARM64="$(sha_for zsh-history-enquirer-darwin-arm64)" \
  || { echo "error: zsh-history-enquirer-darwin-arm64 missing from checksums.txt" >&2; exit 1; }
SHA_DARWIN_AMD64="$(sha_for zsh-history-enquirer-darwin-amd64)" \
  || { echo "error: zsh-history-enquirer-darwin-amd64 missing from checksums.txt" >&2; exit 1; }
SHA_LINUX_ARM64="$(sha_for zsh-history-enquirer-linux-arm64)" \
  || { echo "error: zsh-history-enquirer-linux-arm64 missing from checksums.txt" >&2; exit 1; }
SHA_LINUX_AMD64="$(sha_for zsh-history-enquirer-linux-amd64)" \
  || { echo "error: zsh-history-enquirer-linux-amd64 missing from checksums.txt" >&2; exit 1; }

# The plugin file ships as a separate release asset so the formula
# can install it into share/. Without this, a brew-installed binary
# has no plugin.zsh on disk and the README's `source ...` line points
# at a non-existent path.
SHA_PLUGIN="$(sha_for zsh-history-enquirer.plugin.zsh)" \
  || { echo "error: zsh-history-enquirer.plugin.zsh missing from checksums.txt" >&2; exit 1; }

echo "    darwin-arm64: ${SHA_DARWIN_ARM64}"
echo "    darwin-amd64: ${SHA_DARWIN_AMD64}"
echo "    linux-arm64:  ${SHA_LINUX_ARM64}"
echo "    linux-amd64:  ${SHA_LINUX_AMD64}"
echo "    plugin.zsh:   ${SHA_PLUGIN}"

echo "==> Cloning ${TAP_REPO}"
TAP_DIR="${work}/tap"
git clone "https://x-access-token:${HOMEBREW_TAP_TOKEN}@github.com/${TAP_REPO}.git" "${TAP_DIR}"

cd "${TAP_DIR}"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

# Always reset the bump branch to the latest main so re-runs amend
# rather than duplicate.
git fetch origin main
git checkout -B "${BRANCH}" origin/main

FORMULA="Formula/zsh-history-enquirer.rb"

if [ ! -f "${FORMULA}" ]; then
  cat > "${FORMULA}" <<EOF
class ZshHistoryEnquirer < Formula
  desc "Replace zsh's Ctrl+R with an inline, multi-line, dedup'd, multi-word fuzzy history picker"
  homepage "https://github.com/zthxxx/zsh-history-enquirer"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/zthxxx/zsh-history-enquirer/releases/download/v#{version}/zsh-history-enquirer-darwin-arm64"
      sha256 "${SHA_DARWIN_ARM64}"
    else
      url "https://github.com/zthxxx/zsh-history-enquirer/releases/download/v#{version}/zsh-history-enquirer-darwin-amd64"
      sha256 "${SHA_DARWIN_AMD64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/zthxxx/zsh-history-enquirer/releases/download/v#{version}/zsh-history-enquirer-linux-arm64"
      sha256 "${SHA_LINUX_ARM64}"
    else
      url "https://github.com/zthxxx/zsh-history-enquirer/releases/download/v#{version}/zsh-history-enquirer-linux-amd64"
      sha256 "${SHA_LINUX_AMD64}"
    end
  end

  resource "plugin" do
    url "https://github.com/zthxxx/zsh-history-enquirer/releases/download/v#{version}/zsh-history-enquirer.plugin.zsh"
    sha256 "${SHA_PLUGIN}"
  end

  def install
    bin.install Dir["zsh-history-enquirer-*"].first => "zsh-history-enquirer"
    resource("plugin").stage do
      pkgshare.install "zsh-history-enquirer.plugin.zsh" => "plugin.zsh"
    end
  end

  def caveats
    <<~EOS
      Add the following line to your ~/.zshrc to enable Ctrl+R picker:

        source #{opt_share}/zsh-history-enquirer/plugin.zsh

      The plugin file is intentionally NOT auto-sourced — sourcing is
      under your control.
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/zsh-history-enquirer --version 2>&1")
  end
end
EOF
else
  python3 - <<PY
import re, pathlib
p = pathlib.Path("${FORMULA}")
text = p.read_text()
text = re.sub(r'version "[^"]*"', f'version "${VERSION}"', text)
text = re.sub(r'(zsh-history-enquirer-darwin-arm64".*?sha256 ")[^"]+', r'\g<1>${SHA_DARWIN_ARM64}', text, flags=re.S)
text = re.sub(r'(zsh-history-enquirer-darwin-amd64".*?sha256 ")[^"]+', r'\g<1>${SHA_DARWIN_AMD64}', text, flags=re.S)
text = re.sub(r'(zsh-history-enquirer-linux-arm64".*?sha256 ")[^"]+', r'\g<1>${SHA_LINUX_ARM64}', text, flags=re.S)
text = re.sub(r'(zsh-history-enquirer-linux-amd64".*?sha256 ")[^"]+', r'\g<1>${SHA_LINUX_AMD64}', text, flags=re.S)
text = re.sub(r'(zsh-history-enquirer\.plugin\.zsh".*?sha256 ")[^"]+', r'\g<1>${SHA_PLUGIN}', text, flags=re.S)
p.write_text(text)
PY
fi

git add "${FORMULA}"
if git diff --staged --quiet; then
  echo "==> No changes to ${FORMULA}; skipping PR."
  exit 0
fi

git commit -m "zsh-history-enquirer ${VERSION}

Auto-bump from ${SOURCE_REPO} release ${VERSION}." \
  --author "github-actions[bot] <41898282+github-actions[bot]@users.noreply.github.com>"

git push --force-with-lease origin "${BRANCH}"

if command -v gh >/dev/null 2>&1; then
  GH_TOKEN="${HOMEBREW_TAP_TOKEN}" gh pr create \
    --repo "${TAP_REPO}" \
    --base main \
    --head "${BRANCH}" \
    --title "zsh-history-enquirer ${VERSION}" \
    --body "Auto-bump from [${SOURCE_REPO} v${VERSION}](https://github.com/${SOURCE_REPO}/releases/tag/v${VERSION})." \
    || echo "==> PR may already exist; ignoring create error."
else
  echo "==> gh CLI not available; skipping PR creation. Branch pushed: ${BRANCH}"
fi
