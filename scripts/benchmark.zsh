#!/usr/bin/env zsh

# cwd is project root
# $ scripts/create-benchmark-history.zsh
# $ scripts/benchmark.zsh

function benchmark-run {
  #   https://stackoverflow.com/questions/11023929/using-the-alternate-screen-in-a-bash-script
  tput smcup
  for i in $(seq 1 ${1}); do
    bin/benchmark.js
  done
  tput rmcup
}

# init test data
if [[ ! -f tests/benchmark-history.data ]]; then
  scripts/create-benchmark-history.zsh
fi

# run first for nodejs cache parse
tput smcup
bin/benchmark.js
tput rmcup

time ( benchmark-run 10 times )
