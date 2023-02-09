<h1 align="center">zsh-history-enquirer</h1>

<p align="center">
  <a href="https://github.com/zthxxx/zsh-history-enquirer/actions?query=workflow%3A%22build%22" target="_blank" rel="noopener noreferrer"><img src="https://github.com/zthxxx/zsh-history-enquirer/workflows/build/badge.svg" alt="Build Status" /></a>
  <a href="https://coveralls.io/github/zthxxx/zsh-history-enquirer" target="_blank" rel="noopener noreferrer"><img src="https://badgen.net/coveralls/c/github/zthxxx/zsh-history-enquirer" alt="Build Status" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer" target="_blank" rel="noopener noreferrer"><img src="https://badgen.net/npm/v/zsh-history-enquirer" alt="NPM Version" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer" target="_blank" rel="noopener noreferrer"><img src="https://badgen.net/npm/dt/zsh-history-enquirer" alt="NPM Downloads" /></a>
  <a href="https://nodejs.org/" target="_blank" rel="noopener noreferrer"><img src="https://badgen.net/npm/node/zsh-history-enquirer" alt="Node.js" /></a>
  <a href="https://github.com/zthxxx/zsh-history-enquirer/blob/master/LICENSE" target="_blank" rel="noopener noreferrer"><img src="https://badgen.net/github/license/zthxxx/zsh-history-enquirer" alt="License" /></a>
</p>


## What's this

A plugin that **enhances zsh history search interaction**, with review and choose in a multiline menu

## Preview

### screenshot

<p align="center">
  <kbd>Ctrl</kbd> + <kbd>R</kbd>
  <br />
  <img src="./images/screenshot.png" alt="zsh-history-enquirer screenshot" />
</p>

### live demo

<p align="center">
  <img src="./images/preview.svg?sanitize=true" alt="zsh-history-enquirer preview" />
</p>

## Install

### antigen

```bash
antigen bundle zthxxx/zsh-history-enquirer
```

### oh-my-zsh

If you are using [`oh-my-zsh`](https://github.com/robbyrussell/oh-my-zsh), **all you need to do is one npm command.**

```bash
npm i -g zsh-history-enquirer
```

The install/uninstall hooks will be correctly setup in your `oh-my-zsh` plugins and config. Manually editing `.zshrc` is **no longer necessary**


### one-line command

You can use a one-line command (which will auto install node via nvm, if node command not found)

```bash
curl -#sSL https://github.com/zthxxx/zsh-history-enquirer/raw/master/scripts/installer.zsh | zsh
```

### [Homebrew](https://brew.sh)

```bash
brew install zsh-history-enquirer
```

```bash
# .zshrc
autoload -U history_enquire
history_enquire
```

### manually without oh-my-zsh

If you don't use `oh-my-zsh`, you can manually add the `source` plugin file to your `.zshrc` after npm is installed and manually remove the `source` command when it is uninstalled.

```bash
echo 'source `npm root -g`/zsh-history-enquirer/scripts/zsh-history-enquirer.plugin.zsh' >> ~/.zshrc
```

## Usage

This plugin will replace the default ZSH history search with the `^R` shortcut.

Just press <kbd>^R</kbd> (<kbd>Ctrl</kbd> + <kbd>R</kbd>) to enjoy enhanced history search!


## License

[MIT LICENSE](./LICENSE)


## Author

**zsh-history-enquirer** © [zthxxx](https://github.com/zthxxx), Released under the **[MIT](./LICENSE)** License.<br>

> Blog [@zthxxx](https://blog.zthxxx.me) · GitHub [@zthxxx](https://github.com/zthxxx)
