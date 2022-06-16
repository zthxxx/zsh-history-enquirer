# for antigen bundle init pipepine
# https://github.com/zsh-users/antigen/blob/v2.2.3/bin/antigen.zsh#L518
#
#   install by npm, but do not run npm postinstall

# `zsh-history-enquirer` is also the bin name of package
if [[ ! -e $commands[zsh-history-enquirer] ]]; then
  if [[ ! -e $commands[npm] ]]; then
    echo 'Cannot install `zsh-history-enquirer` due to not found `npm`.' >&2
  else
    export _INIT_ZSH_HISTORY_ENQUIRER_INSTALL=true
    npm i -g zsh-history-enquirer
  fi
fi


