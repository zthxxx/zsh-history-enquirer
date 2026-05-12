#!/usr/bin/env sh
#
# e2e/dev.sh — entrypoint for `task dev`.
#
# Seeds the container's $HOME (write_zshrc + write_seed_history from
# /seed-home.sh — same fixture every e2e scenario starts from) and
# drops into an interactive `zsh -i` so a developer can press
# Ctrl+R live and exercise the picker by hand.
#
# Why a docker shell at all? The e2e test fixtures and the running
# binary must match what CI exercises — otherwise an "it works on my
# machine" reproduction is one tool-version drift away. Reusing the
# same debian-slim image + the same seed-home.sh guarantees that
# `task dev` and `task ci:e2e:run` operate against byte-identical
# baselines.
#
# Mounts (provided by `task dev`):
#   /usr/local/bin/zsh-history-enquirer       Go binary (linux amd64)
#   /opt/zsh-history-enquirer/plugin.zsh      widget plugin
#   /seed-home.sh                             shared HOME seeders
#   /dev.sh                                   this script
#
# The docker image's default ENTRYPOINT is /run.sh (the scenario
# harness); `task dev` overrides it with `--entrypoint /dev.sh` so
# the same image serves both jobs without rebuilding.
set -eu

# shellcheck source=seed-home.sh
. /seed-home.sh

write_zshrc
write_seed_history

cd "${USER_HOME}"

cat <<'BANNER'
=================================================================
  task dev — interactive zsh shell in debian (e2e baseline image)

  $HISTFILE is seeded with the same fixture every e2e scenario
  starts from (~33 entries including two multi-line commands).
  Press Ctrl+R to invoke the zsh-history-enquirer widget against
  this fixture.

  Notes:
   - This shell is ephemeral. Anything you type or any history
     entry you add is destroyed when you `exit`.
   - The plugin file and the binary are bind-mounted from the
     host, so editing them on the host + rebuilding the binary
     (`task build:linux`) takes effect on the next `^R` press.
   - To exit: type `exit` or press Ctrl+D.
=================================================================
BANNER

exec zsh -i
