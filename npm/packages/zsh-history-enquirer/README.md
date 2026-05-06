# zsh-history-enquirer

Inline, multi-line, deduplicated, multi-word fuzzy `Ctrl+R` for zsh.

> Distributed as a static Go binary; the npm package is the install
> shim. After installing, source the plugin file from your `~/.zshrc`
> (the package never edits your shell config).

## Install

```bash
npm install -g zsh-history-enquirer
# or:
pnpm add -g zsh-history-enquirer
# or:
yarn global add zsh-history-enquirer
```

Then add this line to `~/.zshrc`:

```bash
source "$(npm root -g)/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh"
```

Restart your shell. Press <kbd>Ctrl</kbd>+<kbd>R</kbd>.

## Why no postinstall edit?

The Node.js predecessor of this package edited `~/.zshrc` and the
oh-my-zsh plugin list automatically. That caused subtle re-install
loops, surprising diffs in dotfiles repos, and conflicts with users'
own plugin managers. The Go rewrite refuses to touch shell config —
sourcing the plugin file is the only thing under your control.

## Cross-platform binaries

The binary is a static Go executable, shipped via per-platform sub-
packages declared as `optionalDependencies`:

* `@zsh-history-enquirer/darwin-arm64`
* `@zsh-history-enquirer/darwin-amd64`
* `@zsh-history-enquirer/linux-arm64`  (works on glibc, musl, OpenWrt)
* `@zsh-history-enquirer/linux-amd64`  (works on glibc, musl, OpenWrt)

npm only installs the one matching your machine.

See [the project README](https://github.com/zthxxx/zsh-history-enquirer)
for full documentation.
