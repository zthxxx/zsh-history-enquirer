#!/usr/bin/env bash

# https://www.tcl.tk/man/expect5.31/expect.1.html
# https://zh.wikipedia.org/wiki/%E6%8E%A7%E5%88%B6%E5%AD%97%E7%AC%A6
# https://donsnotes.com/tech/charsets/ascii.html
# https://en.wikipedia.org/wiki/ANSI_escape_code


# Cursor Up        <ESC>[{COUNT}A
# Cursor Down      <ESC>[{COUNT}B
# Cursor Right     <ESC>[{COUNT}C
# Cursor Left      <ESC>[{COUNT}D


# because use cursor.zsh will display cursor stdout in expect
# assume default cursor in col 7
echo "#!/usr/bin/env zsh
  # row col (only col for use)
  echo 0 7
" > ./dist/cursor.zsh


ZDOTDIR="`pwd`/scripts" \
stdbuf -i0 -o0 -e0 expect -c '
  spawn -noecho stdbuf -i0 -o0 -e0 zsh -il

  set ESC "\u001b\["
  set CURSOR_UP "A"
  set CURSOR_DOWN "B"
  set CURSOR_RIGHT "C"
  set CURSOR_LEFT "D"

  set send_slow {1 .15}

  # send slow
  proc slowly {arg} {
    global ESC CURSOR_LEFT

    set list1 [split "$arg" ""]
    set len [llength "$list1"]

    foreach chr "$list1" {
      send_tty -- "$chr"
      send -- "$chr"
      sleep .15
    }
    # left cursor to wait `send`
    send_tty -- "${ESC}${len}${CURSOR_LEFT}"
  }

  expect zthxxx {
    slowly "echo zsh-"
  }

  expect history {
    sleep .5
    # Ctrl + E
    send -s "\x05"
  }

  expect enquirer {
    sleep .3
    send -s "\n"
  }

  expect zthxxx {
    sleep .3
    # Ctrl + R
    send -s "\x12"
  }

  expect author {
    global ESC CURSOR_DOWN
    sleep .3
    send -s "${ESC}${CURSOR_DOWN}"
  }

  expect echo {
    sleep 0.2
    send -s "\n"
  }

  expect zthxxx {
    sleep 0.2
    send -s "\n"
  }

  expect zthxxx {
    sleep 0.2
    send -s "\n"
  }
  expect zthxxx {
    sleep 0.2
    send -s "\n"
  }
  expect zthxxx {
    sleep 1
    # Ctrl+D
    send "\x04"
  }

#  interact
'


cp -f ./src/cursor.zsh ./dist/cursor.zsh
git checkout -q ./tests/history.txt
