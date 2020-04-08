#!/usr/bin/env zsh
# https://stackoverflow.com/questions/2575037/how-to-get-the-cursor-position-in-bash

# ./cursor.zsh
# output --> <row> <col>

echo -ne '\u001b[6n' > /dev/tty

read -t 1 -s -d 'R' pos < /dev/tty
pos="${pos##*\[}"

row="$(( ${pos%;*} -1 ))"
col="$(( ${pos#*;} -1 ))"

echo $row $col
