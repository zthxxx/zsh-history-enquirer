<h1 align="center">zsh-history-enquirer</h1>

<p align="center">
  <a href="https://github.com/zthxxx/zsh-history-enquirer/actions/workflows/ci.yml"><img src="https://github.com/zthxxx/zsh-history-enquirer/actions/workflows/ci.yml/badge.svg?branch=master" alt="CI" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer"><img src="https://badgen.net/npm/v/zsh-history-enquirer" alt="NPM Version" /></a>
  <a href="https://www.npmjs.com/package/zsh-history-enquirer"><img src="https://badgen.net/npm/dt/zsh-history-enquirer" alt="NPM Downloads" /></a>
</p>

<p align="center"><a href="./README.md">English</a> · <strong>简体中文</strong></p>

zsh 历史搜索增强插件，把默认的 <kbd>Ctrl</kbd>+<kbd>R</kbd>
替换成「就地预览、多行展示、空格分词模糊匹配、自动去重、最近优先」的
历史选择器。

底层是一个静态编译的 Go 二进制（`CGO_ENABLED=0`）— 同一个文件就能在
macOS、glibc Linux、musl Linux 和 OpenWrt 上原样运行。

## 为什么写这个

原生 <kbd>Ctrl</kbd>+<kbd>R</kbd> 有两个老毛病：

1. **一次只能看一条匹配。**
2. **只能字面包含搜索，没有多词组合搜索。**

输入 `foo`，第一条匹配不是你想要的（可能是某次错误输入）。再按
<kbd>Ctrl</kbd>+<kbd>R</kbd>，又不对。再按。还不对。按到第三下你已经
开始怀疑：第几条才是想要的？还要按多少次？是不是按太快错过了？是不是
根本没有这条命令？

绝大多数人最后会改用 `history | grep -i xxx | tail` —— 用一条命令去
找另一条命令，每次都来一遍。

## 我们的做法：把"老虎机"换成"列表"

一次显示 **15 条候选**，按时间逆序（最近的在最上面），就在你当前
prompt 的下方原地铺开。眼睛扫一眼就够了。命中不准就再加一个关键词
继续收敛，比"再按一下试试"靠谱得多。

如果想要的命令在这 15 条里（基本都在），↓+<kbd>Enter</kbd> 就拿到
了。命令会回填到你的 prompt 里，仍然可以二次编辑，再按一次
<kbd>Enter</kbd> 才真正执行。

### 用起来是什么感觉

- **就地展开，不会全屏霸占终端。** 启动时通过 DSR 光标查询拿到当前
  prompt 的真实列号，所以 Starship / Powerlevel10k / Spaceship 这类
  多段彩色 prompt 在左侧不会被覆盖。
- **沿用你已经在 prompt 里输入的内容。** 比如你已经打了 `git log `
  再按 <kbd>Ctrl</kbd>+<kbd>R</kbd>，picker 会用 `git log` 直接预过
  滤一遍。
- **逆序展示**，最近的最上面。
- **自动去重**，避免一连串 `gco .` 把你淹没。
- **多词模糊。** 空格分词后做 AND 匹配（不区分大小写）—— 输入
  `log iso` 能命中 `git log --pretty=fuller --date=iso -n 1`。
- **多行命令完整展开。** heredoc / 反斜杠续行的长命令直接铺开成多
  行，不被截断也不在中间打省略号。如果某条命令太长导致 15 行装不下，
  picker 会自动收缩，永远不会顶破终端高度。
- **HISTSIZE=100000。** 比 zsh 默认的 30 条多了一万倍。每次打开都执
  行 `fc -R` 重新读盘，所以并行 shell 里的命令会立刻出现。
- **<kbd>PageUp</kbd> / <kbd>PageDown</kbd> / <kbd>Home</kbd> / <kbd>End</kbd>**
  快速翻页。
- **正确处理 bracketed paste**，从剪贴板粘关键词不会触发奇怪的按键。
- **取消（Esc / Ctrl+C）和"无匹配回车"都保留你输入的字。** 永远不会
  让你白打。
- **优雅降级。** 如果这个二进制临时不在 PATH 上（比如 npm install
  途中），<kbd>Ctrl</kbd>+<kbd>R</kbd> 会回退到 zsh 自带的
  `history-incremental-search-backward`，绝不死按。
- **支持 vi-mode。** `^R` 显式绑定在 `emacs` / `viins` / `vicmd`
  三套 keymap 上，vi 模式用户在插入模式和普通模式下都能用。

## 安装

> **plugin 文件需要你自己 source。** Go 重写版**不会**修改你的
> `~/.zshrc` 或 oh-my-zsh 的 plugins 列表 —— 这一行 `source` 完全
> 在你掌控之下。

### npm

```bash
npm install -g zsh-history-enquirer
```

抢鲜体验预发布版本（alpha / beta / rc tag）请使用
`next` dist-tag：

```bash
npm install -g zsh-history-enquirer@next
```

加到 `~/.zshrc`：

```bash
source "$(npm root -g)/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh"
```

### Homebrew

```bash
brew install zthxxx/tap/zsh-history-enquirer
```

加到 `~/.zshrc`：

```bash
source "$(brew --prefix)/share/zsh-history-enquirer/plugin.zsh"
```

### oh-my-zsh

```bash
mkdir -p "$ZSH_CUSTOM/plugins/zsh-history-enquirer"
ln -sf "$(npm root -g)/zsh-history-enquirer/plugin/zsh-history-enquirer.plugin.zsh" \
       "$ZSH_CUSTOM/plugins/zsh-history-enquirer/"
# 然后在 ~/.zshrc 的 plugins=(...) 里加上 zsh-history-enquirer
```

### 源码编译

```bash
git clone https://github.com/zthxxx/zsh-history-enquirer
cd zsh-history-enquirer
task build
sudo install bin/zsh-history-enquirer /usr/local/bin/
echo 'source '"$PWD"'/plugin/zsh-history-enquirer.plugin.zsh' >> ~/.zshrc
```

## 操作方式

| 按键 | 动作 |
| --- | --- |
| 任意可见字符 | 空格分词 AND 模糊筛选（不区分大小写） |
| <kbd>↑</kbd> / <kbd>↓</kbd> | 移动选中行 |
| <kbd>Ctrl+P</kbd> / <kbd>Ctrl+N</kbd> | ↑ / ↓ 的别名（对齐 zsh emacs keymap 的肌肉记忆） |
| <kbd>PageUp</kbd> / <kbd>PageDown</kbd> | 翻页 |
| <kbd>Home</kbd> / <kbd>End</kbd> | 跳到首条 / 末条匹配 |
| <kbd>Backspace</kbd> | 删除一个字符（按 rune，能正确处理中文 / emoji 多字节字符） |
| <kbd>Ctrl+W</kbd> | 删除前一个单词（对齐 zsh 的 `backward-kill-word`） |
| <kbd>Ctrl+U</kbd> | 清空输入 |
| <kbd>Enter</kbd> | 把选中条目回填到 prompt（仍可编辑，再按一次 <kbd>Enter</kbd> 才执行） |
| <kbd>Esc</kbd> / <kbd>Ctrl+C</kbd> | 取消，保留已输入的关键词 |

## 跟 fzf / peco / percol 的差异

`fzf` 作为通用 picker 很强，但当历史搜索用的话有几个不舒服的点：

- **不去重** —— 同一条 `gco .` 在不同页面反复出现。
- **不是 inline** —— 在 prompt 下方约 13 行处展开，眼睛要在两个区域
  之间跳。
- **默认时间正序**，跟"最近用过的优先"反着来。
- **长命令展示得不好** —— 中间被省略，看不出是哪条。
- **取消会丢掉已经打进去的字。**

`peco` 和 `percol` 走的是另一条全屏窗口的路线 —— 更重，焦点切换更
明显。

本插件就是干一件事：把 `^R` 这一段做好。如果你已经在用 `fzf` 选 SSH
host / k8s context / 切换目录，继续用就好，这两件事互不冲突。

## 实现

完全的 Go 重写。整个交付物是一个静态 ELF/Mach-O，没有 node 运行时、
没有 npm 跨版本兼容问题。

详细架构和每个 package 的契约见 [`AGENTS.md`](./AGENTS.md)。

## 协议

[MIT LICENSE](./LICENSE)

## 作者

**zsh-history-enquirer** © [zthxxx](https://github.com/zthxxx)，以
[MIT](./LICENSE) 协议发布。
