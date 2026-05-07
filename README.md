<h1 align="center">zsh-history-enquirer</h1>

<p align="center">
  <a href="https://github.com/zthxxx/zsh-history-enquirer/actions/workflows/ci.yml"><img src="https://github.com/zthxxx/zsh-history-enquirer/actions/workflows/ci.yml/badge.svg?branch=master" alt="CI" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer"><img src="https://badgen.net/npm/v/zsh-history-enquirer" alt="NPM Version" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer"><img src="https://badgen.net/npm/dt/zsh-history-enquirer" alt="NPM Downloads" /></a>
  <a href="https://github.com/zthxxx/zsh-history-enquirer/blob/master/LICENSE"><img src="https://badgen.net/github/license/zthxxx/zsh-history-enquirer" alt="License" /></a>
</p>

<p align="center"><strong>English</strong> · <a href="./README.zh-CN.md">简体中文</a></p>

A zsh plugin that **enhances zsh history search**, with review and choose
in a multiline menu. Replaces the default <kbd>Ctrl</kbd>+<kbd>R</kbd>
with an **inline** picker that previews 15 deduplicated commands at
once, supports multi-word fuzzy matching, full multi-line command
rendering, page navigation, and preserves your typed input on cancel.

Implemented as a static Go binary (`CGO_ENABLED=0`) — the same
artifact runs on macOS, glibc Linux, musl Linux, and OpenWrt without
recompiling.

## Why

### The two problems with native `Ctrl+R`

1. **It only shows one match at a time.**
2. **It only does literal substring match — no multi-word search.**

Imagine you press <kbd>Ctrl</kbd>+<kbd>R</kbd> and type `foo`. The
first match isn't what you want — say it's an old typo. So you press
<kbd>Ctrl</kbd>+<kbd>R</kbd> again. Still wrong. Again. Wrong. By the
third press, you start to lose trust:

- *Which match number is the one I actually want?*
- *How many more presses until I get there?*
- *Did I press too fast and skip past it?*
- *Or does this command not exist in history at all?*

Most people give up and fall back to `history | grep -i xxx | tail` —
typing a command to search for a command, every time.

### The fix: preview, then cut your losses

This plugin shows **15 candidates per search**, reverse-ordered (most
recent first), in place under your prompt. Your eye scans the list
once. If your keyword isn't pulling up the right thing, you *know*
immediately and add another keyword to narrow it down — instead of
gambling on "maybe the next one." That's the entire idea: replace
the slot machine with a list.

If the answer is in those 15 lines (it usually is), arrow down and
press <kbd>Enter</kbd>. The selected command lands back in your
prompt buffer where you can still edit it before hitting
<kbd>Enter</kbd> a second time to run it.

### What you get

- **Inline, on your existing prompt.** No full-screen takeover. The
  picker captures your prompt's real column with a DSR cursor query
  before rendering, so multi-segment / colored / git-aware prompts
  (Starship, Powerlevel10k, Spaceship, …) are left untouched on the
  left.
- **Pre-filtered from what you'd already typed.** If your prompt says
  `git log ` when you press <kbd>Ctrl</kbd>+<kbd>R</kbd>, the picker
  opens already filtered to those words.
- **Reverse-ordered.** Most recent first.
- **Deduplicated.** No paging through ten copies of `gco .`.
- **Multi-word fuzzy match.** Space-separated tokens are AND-matched
  (case-insensitive), so `log iso` finds
  `git log --pretty=fuller --date=iso -n 1`.
- **Multi-line commands rendered in full.** Long heredocs and
  backslash-continued commands show as multiple lines (not truncated,
  not middle-elided). The picker shrinks below 15 entries when
  entries are long so it never overflows your terminal height.
- **Up to 100,000 entries deep.** Reads `$HISTFILE` with
  `HISTSIZE=100000` (vs. zsh's 30-line default for <kbd>Ctrl</kbd>+<kbd>R</kbd>),
  and refreshes from disk (`fc -R`) on every open — commands from
  sibling shells appear immediately.
- **<kbd>PageUp</kbd> / <kbd>PageDown</kbd> / <kbd>Home</kbd> / <kbd>End</kbd>**
  for fast skimming.
- **Bracketed paste works correctly** — pasting a keyword from your
  clipboard doesn't trigger spurious key handlers.
- **CJK and emoji-aware.** Display-width math goes through Unicode's
  East Asian Width tables (`mattn/go-runewidth`), so CJK ideographs
  and emoji each take their actual 2 cells in cursor / wrap math —
  the picker aligns correctly against the prompt for `cd 我的文档`,
  `echo café`, `git commit -m "🚀 ship"`, etc.
- **Cancel *and* submitting a no-match preserve input.** Whatever
  you typed in the search box lands back in your shell prompt either
  way, never retyped.
- **Graceful fallback.** If the binary happens to be missing
  (mid-install, broken `$PATH`, etc.), <kbd>Ctrl</kbd>+<kbd>R</kbd>
  degrades to native `history-incremental-search-backward` instead
  of breaking.
- **Vi-mode aware.** `^R` is bound in the `emacs`, `viins`, and
  `vicmd` keymaps explicitly, so vi-mode users keep the picker
  whether they're in insert or normal mode.

## Install

> **The plugin file is sourced manually.** The Go port deliberately
> does *not* edit your `~/.zshrc` or oh-my-zsh's plugin list. The
> source line is one line you control.

### npm

```bash
npm install -g zsh-history-enquirer
# or:
pnpm add -g zsh-history-enquirer
# or:
yarn global add zsh-history-enquirer
```

For pre-release versions (alpha / beta / rc tags) use the
`next` dist-tag:

```bash
npm install -g zsh-history-enquirer@next
```

Add to `~/.zshrc`:

```bash
source "$(npm root -g)/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh"
```

### Homebrew

```bash
brew install zthxxx/tap/zsh-history-enquirer
```

Add to `~/.zshrc`:

```bash
source "$(brew --prefix)/share/zsh-history-enquirer/plugin.zsh"
```

### Antigen / oh-my-zsh

Both are supported, but you still wire the source line in by hand —
the auto-modification of `.zshrc` from the v1.x npm package has been
removed.

For oh-my-zsh:

```bash
mkdir -p "$ZSH_CUSTOM/plugins/zsh-history-enquirer"
ln -sf "$(npm root -g)/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh" \
       "$ZSH_CUSTOM/plugins/zsh-history-enquirer/"
# then add zsh-history-enquirer to plugins=(...) in ~/.zshrc
```

### Manual / from source

```bash
git clone https://github.com/zthxxx/zsh-history-enquirer
cd zsh-history-enquirer
task build
sudo install bin/zsh-history-enquirer /usr/local/bin/
echo 'source '"$PWD"'/plugin/zsh-history-enquirer.plugin.zsh' >> ~/.zshrc
```

## Usage

| Key | Action |
| --- | --- |
| any text | Multi-word fuzzy filter — every space-separated token must appear in the line (case-insensitive); matches are highlighted in the rendered list |
| <kbd>↑</kbd> / <kbd>↓</kbd> | Move selection one line |
| <kbd>Ctrl+P</kbd> / <kbd>Ctrl+N</kbd> | Aliases for ↑ / ↓ (zsh emacs-keymap muscle memory) |
| <kbd>PageUp</kbd> / <kbd>PageDown</kbd> | Jump a page |
| <kbd>Home</kbd> / <kbd>End</kbd> | Jump to first / last match |
| <kbd>Backspace</kbd> | Delete one character (rune-aware — works on CJK / emoji input) |
| <kbd>Ctrl+W</kbd> | Delete the previous word (matches zsh's `backward-kill-word`) |
| <kbd>Ctrl+U</kbd> | Clear the input |
| <kbd>Enter</kbd> | Put the selected line into the prompt buffer (still editable — press <kbd>Enter</kbd> again to run it) |
| <kbd>Esc</kbd> / <kbd>Ctrl+C</kbd> | Cancel; your typed input is preserved |

## CLI flags

You can run the binary directly to inspect or pin behaviour:

```text
zsh-history-enquirer [flags] [initial input...]
  --version          print the version and exit
  --histfile PATH    override $HISTFILE (default $HISTFILE or ~/.zsh_history)
  --histsize N       override HISTSIZE (default 100000)
  --max-limit N      cap the number of choices rendered (default 15)
```

`--version` prints to stdout, so `zsh-history-enquirer --version | grep` works.
The other flags are most useful for debugging and for the e2e harness.

## Power-user notes

- **`npm install -g`** writes to `$(npm root -g)`. On many Linux
  distros that path is owned by root; use `sudo npm i -g` or set up
  a user-local global prefix (`npm config set prefix ~/.npm-global`).
  Without write access the install silently lands the binary
  somewhere `$PATH` will not see, and the plugin's graceful fallback
  kicks in — pressing <kbd>Ctrl</kbd>+<kbd>R</kbd> will give you
  zsh's native search instead of the picker.
- **`$HISTFILE` on macOS** defaults to `~/.zsh_sess_history` for
  non-interactive zsh and `~/.zsh_history` for interactive shells.
  If your prompt uses a custom `$HISTFILE`, export it *before*
  sourcing this plugin so the picker reads the same history file
  zsh writes to.
- **oh-my-zsh users** must source the plugin file from
  `$(npm root -g)/zsh-history-enquirer/plugin/...`; oh-my-zsh's
  `plugins=(...)` array does not auto-discover npm-installed plugins.
- **`NO_COLOR=1`** suppresses the bold-cyan token highlight (and
  the per-entry SGR reset) so the picker emits plain text only.
  Honors the [no-color.org](https://no-color.org) convention.
  Search / filtering / multi-word matching are unaffected — only
  the visual highlight is removed.

## Implementation

This is a **Go rewrite** of the original Node.js implementation. The
binary is a single static ELF/Mach-O — no runtime dependency on
node, no library version mismatch headaches.

| Layer | Package |
| --- | --- |
| Widget integration | `plugin/zsh-history-enquirer.plugin.zsh` |
| Process entrypoint | `cmd/zsh-history-enquirer` |
| DI graph | `internal/app` (uber-go/fx) |
| TTY + cursor probe | `internal/tty` |
| Key/paste parser | `internal/keys` |
| History loader | `internal/history` |
| Search filter | `internal/search` |
| UI model + render | `internal/ui` |

See [`AGENTS.md`](./AGENTS.md) for the per-package contract and the
gotchas accumulated through development.

## Compared to `fzf` / `peco` / `percol`

`fzf` is a great general-purpose picker, but as a history search it
has a few rough edges:

- **No deduplication** — the same `gco .` repeats across pages.
- **Not inline** — opens ~13 rows below your prompt; eyes pinball
  between two areas.
- **Sort is chronological ascending**, not "most recent first."
- **Long commands display poorly** — the middle gets shown rather
  than the start or end.
- **Cancel discards your input.**

`peco` and `percol` open a separate full-screen window: heavier,
breaks focus, and visually busier than an inline picker.

This plugin is intentionally narrow — it's a history picker, not a
general fuzzy finder. If you already use `fzf` for SSH hosts, k8s
contexts, or directory jumping, keep doing that; this just makes the
<kbd>Ctrl</kbd>+<kbd>R</kbd> corner of that workflow nicer.

## License

[MIT LICENSE](./LICENSE)

## Author

**zsh-history-enquirer** © [zthxxx](https://github.com/zthxxx).
Released under the [MIT](./LICENSE) License.
