#!/usr/bin/env zsh

# https://www.tcl.tk/man/expect5.31/expect.1.html
# https://zh.wikipedia.org/wiki/%E6%8E%A7%E5%88%B6%E5%AD%97%E7%AC%A6
# https://donsnotes.com/tech/charsets/ascii.html
# https://en.wikipedia.org/wiki/ANSI_escape_code


# WARNING: this expect can run only  with zsh theme `jovial`

# because use cursor.zsh will display cursor stdout in expect
# assume default cursor in col 6 in jovial ("╰──➤  ")
echo "echo 0 6" > ./dist/cursor.zsh

local ORIGIN_HISTFILE="${HISTFILE}"


ZDOTDIR="`pwd`/scripts" \
expect -c '
  spawn -noecho zsh -il
  stty raw

  set ESC "\u001b\["

  # Cursor Up        <ESC>[{COUNT}A
  # Cursor Down      <ESC>[{COUNT}B
  # Cursor Right     <ESC>[{COUNT}C
  # Cursor Left      <ESC>[{COUNT}D
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

  ##################################
  # zsh-history-enquirer

  expect "──➤" {
    slowly "echo zsh-"
  }

  expect history {
    sleep .2
    # Ctrl + E
    send "\x05"
  }

  expect enquirer {
    sleep .3
    send "\n"
  }

  ##################################
  # echo multiline

  expect "──➤" {
    sleep .5
    # Ctrl + R
    send "\x12"
  }

  expect "earlier" {
    global ESC CURSOR_DOWN
    sleep .4
    send "${ESC}${CURSOR_DOWN}"
  }

  expect "earlier" {
    sleep .6
    send "\n"
  }

  expect supported {
    sleep .5
    send "\n"
  }

  ##################################
  # scroll down and choose author zthxxx

  expect "──➤" {
    sleep 1
    # Ctrl + L
    send "\x0c"
    sleep .2
  }

  expect "──➤" {
    sleep 1
    # Ctrl + R
    send "\x12"
  }

  for { set i 0}  {$i < 9} {incr i} {
    expect "earlier" {
      if {$i == 0} {
        sleep .5
      }

      global ESC CURSOR_DOWN
      sleep .13
      send "${ESC}${CURSOR_DOWN}"
    }
  }

  expect "author zthxxx" {
    sleep .9
    send "\n"
  }

  ##################################
  # inpurt zthxxxxxxxxxxxxxxxxxx

  expect "author zthxxx" {
    sleep .5
    slowly "xxxxxxxxxxxxxxx"
    send "\n"
  }

  ##################################
  # search and input echo

  expect "──➤" {
    sleep 1
    # Ctrl + L
    send "\x0c"
    sleep .2
  }

  expect "──➤" {
    sleep 1.2
    # Ctrl + R
    send "\x12"
    sleep .2
  }

  foreach chr {e c h o} {
    expect "command" {
      if {$chr == "e"} {
        sleep .25
      }

      sleep .25
      send "$chr"
    }
  }


  for { set i 0}  {$i < 4} {incr i} {
    expect "command" {
      global ESC CURSOR_DOWN
      sleep .3
      send "${ESC}${CURSOR_DOWN}"
    }
  }

  expect "command" {
    sleep .5
    send "\n"
  }

  expect "command" {
    sleep .5
    send "\n"
  }


  ##################################
  # input git & choose where git

  expect "──➤" {
    sleep .5
    slowly "git"
  }

  # because aleady input `git`
  # cursor in col 6 + 3 = 9 ("╰──➤  git")
  exec echo "echo 0 9" > ./dist/cursor.zsh

  expect "status" {
    sleep .5
    # Ctrl + R
    send "\x12"
    sleep .2
  }

  expect "iso" {
    global ESC CURSOR_DOWN
    sleep .5
    send "${ESC}${CURSOR_DOWN}"
  }

  expect "iso" {
    sleep .7
    send "\n"
  }

  expect "where git" {
    sleep .8
    send "\n"
  }

  ##################################
  # exit

  expect "──➤" {
    sleep 3

    set timeout 1
    send -- " "
    expect NOTTHING
    send -- "\x08"

    # Ctrl+D
    send "\x04"
  }

#  interact
'


cp -f ./src/cursor.zsh ./dist/cursor.zsh
git checkout -q ./tests/history.txt
export HISTFILE="${ORIGIN_HISTFILE}"
