if [[ $+commands[fcenquire] ]]; then
  function history_enquire() {
    BUFFER=$(fcenquire "$LBUFFER")
    CUOSOR=$#BUFFER
    zle -R -c
  }

  zle -N history_enquire
  bindkey '^R' history_enquire
fi
