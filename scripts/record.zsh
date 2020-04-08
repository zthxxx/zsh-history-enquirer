#!/usr/bin/env zsh

# WORKDIR: ../ (project root)
# ZDOTDIR: ../scripts/ (current dir)
# HISTFILE: ../tests/history.txt (set in `.zlogin`)

# pre-record
ln -fs ~/.zshrc ./scripts/.zshrc

# asciinema & control record
stty rows 20 columns 72
clear
asciinema rec \
  --overwrite ./images/zsh-record.cast \
  -c './scripts/control-record.zsh'

# post-record
rm -rf ./srcipts/.zshrc ./srcipts.zcompdump-*
git checkout -q ./tests/history.txt
