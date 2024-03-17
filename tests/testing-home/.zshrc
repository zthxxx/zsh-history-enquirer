# https://zsh.sourceforge.io/Doc/Release/Options.html#History
# for ensures that commands are added to the history immediately
setopt INC_APPEND_HISTORY
# records the timestamp of each command
setopt EXTENDED_HISTORY

# load plugin
source "${PWD}/zsh-history-enquirer.plugin.zsh"



