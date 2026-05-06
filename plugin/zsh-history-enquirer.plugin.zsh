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
    # Fall back to the builtin widget directly via `zle .` — this
    # invokes the original zsh widget regardless of whatever the
    # current keymap binds ^R to. The previous implementation swapped
    # `bindkey '^R'` around the call, which left transient
    # inconsistent state across the emacs/viins/vicmd keymaps.
    zle .history-incremental-search-backward
  fi
}

zle -N history_enquire

# Bind ^R explicitly in every keymap a typical zsh user might land
# in. Without this, vi-mode users lose the picker the moment they
# hit Esc to switch to vicmd.
bindkey -M emacs '^R' history_enquire
bindkey -M viins '^R' history_enquire
bindkey -M vicmd '^R' history_enquire
