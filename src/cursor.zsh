#!/usr/bin/env zsh

# ./cursor.zsh 
# output --> <row> <col>

echo -ne '\033[6n' > /dev/tty

read -t 1 -s -d 'R' pos < /dev/tty
pos="${pos##*\[}"

row="$(( ${pos%;*} -1 ))"
col="$(( ${pos#*;} -1 ))"

echo $row $col
