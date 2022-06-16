#!/usr/bin/env zsh

if [[ -n ${_INIT_ZSH_HISTORY_ENQUIRER_INSTALL} ]]; then
  # means the install called by `init.zsh` maybe via `antigen`,
  # so, no need to modify any config file

  exit 0
fi


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

ln -fs "`pwd`/${package_name}.plugin.zsh" "${plugins_dir}/${package_name}/"

# it's same as `realpath`, but `realpath` is GNU only and not builtin
prel-realpath() {
  perl -MCwd -e 'print Cwd::realpath($ARGV[0]),qq<\n>' $1
}

local zsh_config_file="$(prel-realpath ${HOME}/.zshrc)"

perl -i -pe "s/^[ \t]*${package_name}[ \t\n]*//gms" "${zsh_config_file}"
perl -i -pe "s/^[ \t]*plugins=\(/plugins=(\n  ${package_name}\n/gms" "${zsh_config_file}"
