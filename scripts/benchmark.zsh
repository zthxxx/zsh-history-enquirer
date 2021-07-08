#!/usr/bin/env zsh

# cwd is project root
# $ ./scripts/benchmark.zsh

function benchmark-run {
  #   https://stackoverflow.com/questions/11023929/using-the-alternate-screen-in-a-bash-script
  tput smcup
  for i in $(seq 1 ${1}); do
    ./bin/benchmark.js
  done
  tput rmcup
}

# run first for nodejs cache parse
tput smcup
./bin/benchmark.js
tput rmcup

time ( benchmark-run 10 times )
