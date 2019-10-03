# replace '^R' search of zsh-history-enquirer in zsh

function history_enquire() {
  # fcenquire is the bin name of zsh-history-enquirer
  if [[ $commands[fcenquire] ]]; then
    BUFFER=$(fcenquire "$LBUFFER")
    CUOSOR=$#BUFFER
    zle -R -c
  else
    # fallback to use zsh default history search
    # http://zsh.sourceforge.net/Doc/Release/Zsh-Line-Editor.html#History-Control
    bindkey '^R' history-incremental-search-backward
    zle history-incremental-search-backward
    bindkey '^R' history_enquire
  fi
}

zle -N history_enquire
bindkey '^R' history_enquire
