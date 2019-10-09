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
    # nvm is not compatible with the npm config "prefix" option
    # Run `nvm use --delete-prefix` to unset it
    nvm use --delete-prefix --lts
    npm i -g npm
  fi

  # access to install for root
  # https://stackoverflow.com/questions/49084929/npm-sudo-global-installation-unsafe-perm
  # https://docs.npmjs.com/misc/config#unsafe-perm
  npm i -g ${package_name} --unsafe-perm
}
