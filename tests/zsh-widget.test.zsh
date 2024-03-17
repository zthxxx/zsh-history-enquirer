ZDOTDIR="`pwd`/tests/testing-home" \
expect -c '
  spawn -noecho zsh -il
  stty raw
  set timeout 20

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

  # to match zsh prompt
  expect "%" {
    sleep .5
    # to match "echo ok"
    slowly "ech"
    sleep .5
    # Ctrl + R
    send "\x12"
  }

  expect "echo ok" {
    sleep .5
    # to select "echo ok"
    send "\n"
    sleep .5
    # to run command "echo ok"
    send "\n"
  }

  # to match command output "ok"
  expect "ok" {
    sleep .5
  }

  # to match zsh prompt
  expect "%" {
    sleep .5
    # Ctrl + R
    send "\x12"
    sleep .5
  }

  for { set i 0}  {$i < 4} {incr i} {
    expect "command" {
      global ESC CURSOR_DOWN
      send "${ESC}${CURSOR_DOWN}"
      sleep .5
    }
  }

  expect "command-4" {
    sleep .5
    # to select "echo command-4"
    send "\n"
  }

  expect "echo command-4" {
    sleep .5
    # to run "echo command-4"
    send "\n"
  }

  # to match command output "command-4"
  expect "command-4" {
    sleep .2
  }

  # to match zsh prompt and done
  expect "%" {
    sleep .2
  }
'
