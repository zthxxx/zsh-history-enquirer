# zsh-history-enquirer — drop-in replacement for ^R history search.
#
# Source this file from your ~/.zshrc:
#
#   source /path/to/zsh-history-enquirer.plugin.zsh
#
# This file is intentionally tiny: it defines a single zle widget that
# shells out to the `zsh-history-enquirer` binary, captures the chosen
# command via $(...), and pastes it into BUFFER. If the binary is not
# on $PATH (mid-install, broken Homebrew prefix, etc.) we fall back to
# zsh's native history-incremental-search-backward for that single key
# press so ^R is never dead.
#
# This plugin does NOT modify ~/.zshrc, oh-my-zsh's plugin list, or any
# other shell config. Add the source line yourself; uninstall is the
# reverse.

function history_enquire() {
  if [[ -n ${commands[zsh-history-enquirer]} ]]; then
    BUFFER=$(zsh-history-enquirer "$LBUFFER")
    CURSOR=$#BUFFER
    zle -R -c
  else
    bindkey '^R' history-incremental-search-backward
    zle history-incremental-search-backward
    bindkey '^R' history_enquire
  fi
}

zle -N history_enquire
bindkey '^R' history_enquire
