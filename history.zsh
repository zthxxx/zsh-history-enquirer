#!/usr/bin/env zsh

# ./history.zsh <historyPath>

# Some good zsh history options to try
# and get as much history as possible.
# The default is 30 lines.
export HISTFILE=${1:-"${HOME}/.zsh_history"}
export HISTSIZE=100000

fc -R
fc -ln 1
