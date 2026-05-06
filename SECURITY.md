# Security Policy

## Reporting a vulnerability

Please **do not** open a public GitHub issue for security-sensitive
reports. Instead:

- File a [private security advisory](https://github.com/zthxxx/zsh-history-enquirer/security/advisories/new)
  on this repository, **or**
- Email the maintainer at the address in `git log --pretty='%ae' | sort -u`.

We will acknowledge within seven days and keep you informed as the
investigation progresses.

## What we consider in scope

`zsh-history-enquirer` is a small piece of software but it sits
directly on every history-search keystroke and runs as the user. Bug
classes that *could* matter:

- **Arbitrary command execution from history content.** The picker
  loads `$HISTFILE` and writes a chosen entry into `BUFFER`. If a
  malicious entry can cause the picker (not zsh) to run code without
  user submission, that's a vulnerability.
- **TTY escape injection.** A history line that contains crafted
  ANSI bytes should not corrupt the user's terminal beyond the
  picker frame. The renderer emits a defensive `\e[0m` after every
  rendered entry to mitigate this.
- **Path traversal in `--histfile`.** A user-supplied path is
  passed straight to zsh; we trust the user, but if there is a way
  for an attacker who controls argv (e.g. via the widget) to pivot
  to read other files, that's a vulnerability.
- **Resource exhaustion.** The picker should bound its memory and
  CPU regardless of `$HISTFILE` size.

## Out of scope

- The user's own `~/.zsh_history`. If you give the picker a
  malicious history file, the worst it should do is render
  whatever's in it and exit cleanly.
- The legacy Node.js port (`master` branch). The Go rewrite
  (`refactor/golang/dev` and onward) is the only supported version.
- Issues that require root, a hostile NPM mirror, or a compromised
  Homebrew tap.

## Disclosure

We disclose in `CHANGELOG.md` after a fix has shipped to npm and
Homebrew. Reporters are credited unless they prefer otherwise.
