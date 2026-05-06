#!/usr/bin/env sh
#
# e2e/run.sh — entrypoint inside the docker image.
#
# For each *.exp under /scenarios/, reset $HOME (fresh .zshrc + fresh
# .zsh_history seeded from /fixtures/seed_history) and run the
# scenario. Aggregate pass/fail; non-zero exit if any failed.
#
# Mounts (provided by `task ci:e2e:run`):
#   /usr/local/bin/zsh-history-enquirer       — Go binary
#   /opt/zsh-history-enquirer/plugin.zsh      — widget plugin
#   /scenarios/                               — *.exp files
#   /run.sh                                   — this script
set -eu

USER_HOME="${HOME:-/home/tester}"
PLUGIN="/opt/zsh-history-enquirer/plugin.zsh"

write_zshrc() {
  cat > "${USER_HOME}/.zshrc" <<'ZSHRC'
HISTSIZE=100000
SAVEHIST=100000
HISTFILE="${HOME}/.zsh_history"

# Disable history sharing so the picker sees only the seed we wrote.
setopt EXTENDED_HISTORY
unsetopt SHARE_HISTORY
unsetopt INC_APPEND_HISTORY
unsetopt APPEND_HISTORY

PROMPT='%% '
RPROMPT=''
ZSHRC
  echo "source ${PLUGIN}" >> "${USER_HOME}/.zshrc"
}

write_seed_history() {
  cat > "${USER_HOME}/.zsh_history" <<'HIST'
: 1568797100:0;command-0
: 1568797100:0;command-1
: 1568797100:0;command-2
: 1568797100:0;command-3
: 1568797100:0;command-4
: 1568797100:0;command-5
: 1568797100:0;command-6 \
 - line 1 \
 - line 2 \
 - line 3 \
\
\
 - line 4 \
 - line 5
: 1568797100:0;command-7
: 1568797100:0;command-8 \
 - line 0 \
 - line 1 \
 - line 2 \
 - line 3 \
 - line 4 \
 - line 5 \
 - line 5
: 1568797100:0;command-9
: 1568797100:0;command-10
: 1568797100:0;command-11
: 1568797100:0;command-12
: 1568797100:0;command-13
: 1568797100:0;command-14
: 1568797100:0;command-15
: 1568797109:0;233333
: 1568797110:0;114514
: 1568797111:0;git log --pretty=fuller --date=iso -n 1
: 1568797112:0;echo earlier command
: 1568797114:0;where git
: 1568797115:0;echo author zthxxx
: 1568797116:0;cd Documents
: 1568797118:0;md5sum --help
: 1568797118:0;git status
: 1568797119:0;cat <<< 123
: 1568797121:0;pwgen --help
: 1568797113:0;echo "alpha\
beta\
gamma"
: 1568797124:0;echo zsh-history-enquirer
: 1568797125:0;echo ok
HIST
  chmod 600 "${USER_HOME}/.zsh_history"
}

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
