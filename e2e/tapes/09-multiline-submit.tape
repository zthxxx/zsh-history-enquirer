# 09-multiline-submit.tape
#
# Showcases the multi-line entry rendering — a 3-line printf is
# rendered with literal newlines visible in the picker body, then
# submitted as a single logical BUFFER. Mirrors
# scenarios/multiline_submit_test.go.

Output out/09-multiline-submit.mp4
Output out/09-multiline-submit.gif

Set Shell zsh
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 960
Set Height 600
Set Theme "Dracula"
Set TypingSpeed 75ms

Hide
Type "export HISTFILE=$HOME/.zsh_history_demo"
Enter
Type "cat > $HISTFILE <<'EOF'"
Enter
Type ": 1568797200:0;echo single-line entry"
Enter
Type ": 1568797201:0;printf '%s\\\\n%s\\\\n%s\\\\n' alpha beta gamma"
Enter
Type ": 1568797202:0;echo trailing"
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
Sleep 1.2s

# Narrow to the multi-line entry by typing a token only it contains.
Type "alpha"
Sleep 1.5s

# Submit.
Enter
Sleep 800ms

# The BUFFER now holds the multi-line printf. Run it — three lines
# of output appear underneath.
Enter
Sleep 2s
