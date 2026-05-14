# 16-narrow-wrap.tape
#
# Picker on a 40-column terminal — proves the inline overlay
# survives line-wrap without breaking. Long history entries
# wrap across multiple rows; the dynamic-limit math keeps the
# visible window stable.

Output out/16-narrow-wrap.mp4
Output out/16-narrow-wrap.gif

Set Shell zsh
Set FontSize 16
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 480
Set Height 720
Set Theme "Dracula"
Set TypingSpeed 75ms

Hide
Type "export HISTFILE=$HOME/.zsh_history_demo"
Enter
Type "cat > $HISTFILE <<'EOF'"
Enter
Type ": 1568797200:0;echo zsh-history-enquirer"
Enter
Type ": 1568797210:0;echo this line is fifty plus characters long sample"
Enter
Type ": 1568797220:0;echo ok"
Enter
Type ": 1568797230:0;cd Documents"
Enter
Type ": 1568797240:0;git status"
Enter
Type "EOF"
Enter
Type "fc -R"
Enter
Type "source /opt/zsh-history-enquirer/plugin.zsh"
Enter
Type "clear"
Enter
Show

Sleep 1s

Ctrl+R
Sleep 1.5s

# Demonstrates that the long entry wraps inside the 40-col window
# without truncating, while the picker continues to render
# correctly above/below it.
Type "echo"
Sleep 2s

Escape
Sleep 600ms
