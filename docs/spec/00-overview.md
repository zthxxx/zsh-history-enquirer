# spec/00-overview — what `zsh-history-enquirer` is

> **Spec layer** — describes user-visible behaviour, framed without reference
> to any specific implementation. The Go port and the legacy Node.js
> implementation must both pass the same spec; that is the whole point of
> separating spec from design.

## Intent

`zsh-history-enquirer` is a **zsh widget** bound to <kbd>Ctrl</kbd>+<kbd>R</kbd>.
It replaces zsh's native `history-incremental-search-backward` with an
inline list picker that:

- previews up to *N* (default 15) deduplicated history entries at once,
- supports multi-word AND filtering,
- reverses to "most recent first",
- renders multi-line history entries in full,
- auto-shrinks below *N* if entries are long, so the list never overflows,
- preserves typed input on cancel,
- never takes over the terminal (no full-screen mode).

It is a **history picker, not a general-purpose fuzzy finder.** Anything
that does not exist to make `^R` better is out of scope.

## Non-goals

- Replacing `fzf` for SSH hosts, k8s contexts, directory jumping, etc.
- Editing history entries.
- Persisting any state of its own (no cache, no config file).
- Auto-modifying `.zshrc` or oh-my-zsh plugin lists. The Node.js port did
  this; the Go port deliberately does not. Users wire the plugin into
  their shell themselves.

## Distribution model

- **Primary**: npm package `zsh-history-enquirer` that ships a
  cross-platform Go binary via `optionalDependencies` of platform-specific
  packages (`@zsh-history-enquirer/<os>-<arch>`), the same shape `esbuild`
  uses.
- **Secondary**: Homebrew tap (`zthxxx/homebrew-tap`), updated via release
  PR.
- **Tertiary**: raw GitHub Release binaries.

The npm distribution exists so existing Node-ecosystem users
(antigen → `npm i -g …`, oh-my-zsh users with `npm i -g …`) can keep their
muscle memory.

## Success criteria

- Every behaviour in `spec/` is exercised by either a unit test (Go,
  reading fixture files only) or an e2e test (running the real binary
  against a real zsh inside Docker).
- The plugin file works against the v1.x Node.js binary as well as the
  new Go binary — the contract is `argv[1..] = LBUFFER`, stdout = chosen
  line, exit 0 always.
- A user who installs the plugin into a Starship / Powerlevel10k / git-
  decorated prompt sees their existing prompt untouched on the left when
  the picker opens.
