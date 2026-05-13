# 01-basic-pick.tape
#
# The hero demo: open the picker, search by substring, submit the
# focused entry. Matches the assertions in
# e2e-v2/scenarios/basic_pick_test.go (TestBasicPick).

Output out/01-basic-pick.mp4
Output out/01-basic-pick.gif

Set Shell zsh
Set FontSize 14
Set FontFamily "JetBrainsMono Nerd Font"
Set Width 960
Set Height 540
Set Theme "Dracula"
Set TypingSpeed 75ms

# Setup — invisible in the recording. Sources the plugin so ^R is
# wired, then seeds a small history so the picker has something
# to show. Adjust the path if running outside the project image.
Hide
Type "export HISTFILE=$HOME/.zsh_history_demo"
Enter
Type "cat > $HISTFILE <<'EOF'"
Enter
Type ": 1568797124:0;echo zsh-history-enquirer"
Enter
Type ": 1568797125:0;echo ok"
Enter
Type ": 1568797121:0;pwgen --help"
Enter
Type ": 1568797118:0;md5sum --help"
Enter
Type ": 1568797115:0;echo author zthxxx"
Enter
Type ": 1568797112:0;echo earlier command"
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

# The recording starts here.
Ctrl+R
Sleep 1.5s

# Type a multi-word fuzzy filter.
Type "echo"
Sleep 1.2s

# Submit the focused entry (most-recent matching = "echo ok").
Enter
Sleep 800ms

# The submitted command appears on the prompt line ready to run.
Sleep 1.5s
