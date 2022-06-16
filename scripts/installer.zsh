#!/usr/bin/env zsh

# bash strict mode (https://gist.github.com/mohanpedala/1e2ff5661761d3abd0385e8223e16425)
set -xo pipefail

local package_name="zsh-history-enquirer"

# install via nvm if node not found
if [[ ! $commands[node] ]]; then
  if [[ -z "${NVM_DIR}" ]]; then
    NVM_DIR="$HOME/.nvm"
  fi

  if [[ ! -d ${NVM_DIR} ]]; then
    # https://github.com/nvm-sh/nvm
    set +x
    echo '[info] not found node or nvm, will install them'
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | zsh
    set -x
  fi

  export NVM_DIR="$HOME/.nvm"
  \. "$NVM_DIR/nvm.sh"  # This loads nvm

  # install node lts if nvm haven't default version
  if ! nvm list default; then
    nvm install --lts
  fi

  nvm use default
fi

npm i -g ${package_name}
