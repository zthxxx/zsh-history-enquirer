#!/usr/bin/env zsh

# bash strict mode (https://gist.github.com/mohanpedala/1e2ff5661761d3abd0385e8223e16425)
set -xo pipefail

local package_name="zsh-history-enquirer"

# install via nvm if node/npm not found
if [[ ! $commands[npm] ]]; then
  if [[ -z "${NVM_DIR}" ]]; then
    NVM_DIR="$HOME/.nvm"
  fi

  if [[ ! -d ${NVM_DIR} ]]; then
    # https://github.com/nvm-sh/nvm
    set +x
    echo '[info] not found node or nvm, will install them'
    curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/master/install.sh | bash
    set -x
  fi

  export NVM_DIR="$HOME/.nvm"
  \. "$NVM_DIR/nvm.sh"  # This loads nvm

  set +x

  # install node lts if nvm haven't default version
  if ! nvm list default; then
    echo '[info] nvm install --lts'
    nvm install --lts
  fi

  echo '[info] nvm install --lts'
  nvm use default

  set -x
fi

npm i -g ${package_name}
