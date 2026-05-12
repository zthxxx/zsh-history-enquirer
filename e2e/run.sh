#!/usr/bin/env sh
#
# e2e/run.sh — entrypoint inside the docker image for the
# automated e2e test harness.
#
# For each *.exp under /scenarios/, reset $HOME (fresh .zshrc +
# fresh .zsh_history via write_seed_history) and run the scenario.
# Aggregate pass/fail; non-zero exit if any failed.
#
# Mounts (provided by `task ci:e2e:run`):
#   /usr/local/bin/zsh-history-enquirer       — Go binary
#   /opt/zsh-history-enquirer/plugin.zsh      — widget plugin
#   /scenarios/                               — *.exp files
#   /run.sh                                   — this script
#   /seed-home.sh                             — shared HOME seeders
#
# For interactive testing, see /dev.sh (entered via `task dev`).
#
# Note: e2e/fixtures/ is reserved for scenarios that need an
# alternate seed (none currently). The default history is the
# heredoc in /seed-home.sh's write_seed_history.
set -eu

# shellcheck source=seed-home.sh
. /seed-home.sh

cd "${USER_HOME}"

PASS=0
FAIL=0
FAILED=""

for scenario in /scenarios/*.exp; do
  [ -e "${scenario}" ] || continue
  name="$(basename "${scenario}")"

  # Reset state per scenario so cross-test contamination cannot mask
  # bugs that only appear with a fresh history.
  write_zshrc
  write_seed_history

  echo ""
  echo "==== ${name} ===="

  if expect -f "${scenario}"; then
    PASS=$((PASS + 1))
    echo "==== ${name}: PASS ===="
  else
    FAIL=$((FAIL + 1))
    FAILED="${FAILED} ${name}"
    echo "==== ${name}: FAIL ===="
  fi
done

echo ""
echo "summary: ${PASS} passed, ${FAIL} failed"
if [ "${FAIL}" -gt 0 ]; then
  echo "failed scenarios:${FAILED}"
  exit 1
fi
