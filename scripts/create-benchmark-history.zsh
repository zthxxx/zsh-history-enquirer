#!/usr/bin/env zsh

# Usage
#
# create test history data for `bin/benchmark.js`
# when cwd is project root
#
# ./scripts/create-benchmark-history.zsh

createHistory() {
  local -i i=0
  local -i timestamp=1600000000

  for i in $(seq 1 10000); do
    echo ": $((timestamp + i)):0;command-${i}"
  done
}

createHistory > tests/benchmark-history.data
