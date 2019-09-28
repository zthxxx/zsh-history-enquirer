#!/usr/bin/env zsh

# curl -sSL https://github.com/zthxxx/zsh-history-enquirer/raw/master/installer.zsh | zsh
{
  local package_name="zsh-history-enquirer"

  # install nodejs
  if [[ ! $commands[node] ]]; then
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | zsh

    export NVM_DIR="$HOME/.nvm"
    [ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # This loads nvm
    [ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"  # This loads nvm bash_completion

    nvm install --lts
    nvm use --lts
    npm i -g npm
  fi

  npm i -g ${package_name}
}
