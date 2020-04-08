#!/usr/bin/env zsh

if [[ -z $ZSH ]]; then
  echo "
  \e[33m
  Warning: cannot find oh-my-zsh, skip setup plugin for oh-my-zsh
  \e[0m
  " >&2

  exit 0
fi

local package_name="zsh-history-enquirer"
local plugins_dir="${ZSH_CUSTOM:-"${ZSH}/custom"}/plugins"

mkdir -p ${plugins_dir}/${package_name}

ln -fs "`pwd`/scripts/${package_name}.plugin.zsh" "${plugins_dir}/${package_name}/"

perl -i -pe "s/^[ \t]*${package_name}[ \t\n]*//gms" ${HOME}/.zshrc
perl -i -pe "s/^[ \t]*plugins=\(/plugins=(\n  ${package_name}\n/gms" $HOME/.zshrc
