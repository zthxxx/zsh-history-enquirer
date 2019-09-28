#!/usr/bin/env zsh -l

if [[ -z $ZSH ]]; then
  echo "\\n\\n!! the plugin made for oh-my-zsh, but cannot find oh-my-zsh, please install it at first\\n\\n" >&2
  exit 1
fi

local package_name="zsh-history-enquirer"
local plugins_dir="${ZSH_CUSTOM:-"${ZSH}/custom"}/plugins"

mkdir -p ${plugins_dir}/${package_name}

cp -f ./${package_name}.plugin.zsh ${plugins_dir}/${package_name}/

perl -i -pe "s/^plugins=\(/plugins=(\n  ${package_name} /gms" $HOME/.zshrc
