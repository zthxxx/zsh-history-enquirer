#!/usr/bin/env sh
#
# e2e/dev.sh — entrypoint for `task dev`.
#
# Renders a scratch $HOME from the SAME canonical seed sources the
# Go harness consumes (e2e/testdata/), then drops into an
# interactive `zsh -i` so a developer can press Ctrl+R live and
# exercise the picker by hand.
#
# Single source of truth: the .zshrc template and the history
# fixtures are the exact files mounted into the harness container
# (writeScratchHome in harness/session.go renders them identically).
# So `task dev` and `task ci:e2e:run` always operate against
# byte-identical baselines — no "works in the dev shell but fails in
# CI" drift. There is deliberately no separate seed script to keep
# in sync.
#
# Mounts (provided by `task dev`):
#   /usr/local/bin/zsh-history-enquirer       Go binary (linux amd64)
#   /opt/zsh-history-enquirer/plugin.zsh      widget plugin
#   /seed/history/                            history fixtures (ro)
#   /seed/zshrc.template                      .zshrc template (ro)
#   /dev.sh                                   this script
#
# Env (set by `task dev`, override with FIXTURE=<name>):
#   SEED_HISTORY    container path to the .history fixture to load.
#                   Defaults to the shared scenario seed.
#   PLUGIN          container path to the plugin file substituted
#                   into the .zshrc template's {{PLUGIN}} placeholder.
#
# The docker image's default ENTRYPOINT is /runner.sh (the Go
# harness driver); `task dev` overrides it with `--entrypoint
# /dev.sh` so the same image serves both jobs without rebuilding.
set -eu

SEED_HISTORY="${SEED_HISTORY:-/seed/history/seed.history}"
ZSHRC_TEMPLATE="${ZSHRC_TEMPLATE:-/seed/zshrc.template}"
PLUGIN="${PLUGIN:-/opt/zsh-history-enquirer/plugin.zsh}"
USER_HOME="${HOME:-/home/tester}"

if [ ! -f "${SEED_HISTORY}" ]; then
  echo "dev.sh: history fixture not found: ${SEED_HISTORY}" >&2
  echo "dev.sh: available fixtures:" >&2
  for f in /seed/history/*.history; do
    [ -e "${f}" ] || continue
    echo "  - $(basename "${f}" .history)" >&2
  done
  exit 1
fi

# Render .zshrc from the template — same {{PLUGIN}} substitution the
# harness performs in writeScratchHome. `#` delimiter is safe: plugin
# paths never contain it.
sed "s#{{PLUGIN}}#${PLUGIN}#g" "${ZSHRC_TEMPLATE}" > "${USER_HOME}/.zshrc"

# Copy the history fixture verbatim (0600 — zsh refuses a world-
# readable $HISTFILE under some configs).
cp "${SEED_HISTORY}" "${USER_HOME}/.zsh_history"
chmod 600 "${USER_HOME}/.zsh_history"

cd "${USER_HOME}"

fixture_name=$(basename "${SEED_HISTORY}" .history)
entry_count=$(grep -c '^: ' "${USER_HOME}/.zsh_history" 2>/dev/null || echo "?")

cat <<BANNER
=================================================================
  task dev — interactive zsh shell in the e2e baseline image

  Fixture : ${fixture_name} (${entry_count} entries)
  HISTFILE: ${USER_HOME}/.zsh_history  (seeded, ephemeral)

  Press Ctrl+R to invoke the zsh-history-enquirer widget against
  this fixture — the exact history every matching e2e scenario
  starts from.

  Notes:
   - This shell is ephemeral. Anything you type or any history
     entry you add is destroyed when you exit.
   - Pick a different fixture with: task dev FIXTURE=<name>
     (e.g. 25-multiline-stick, 14-long-line, 13-unicode, empty).
   - The plugin file and binary are bind-mounted from the host,
     so editing them + rebuilding (task build:linux) takes effect
     on the next ^R press.
   - To exit: type exit or press Ctrl+D.
=================================================================
BANNER

exec zsh -i
