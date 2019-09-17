const fs = require('fs');
const path = require('path');
const execa = require('execa')
const tty = require('tty')
const { AutoComplete } = require('enquirer');


const historyScript = path.join(__dirname, 'history.zsh')


function getStdin () {
  const fd = fs.openSync('/dev/tty', 'r')
  const stdin = new tty.ReadStream(fd, {
    highWaterMark: 0,
    readable: true,
    writable: false
  });

  return stdin
}

function getStdout () {
  const fd = fs.openSync('/dev/tty', 'w')
  const stream = new tty.WriteStream(fd);
  return stream
}

async function history (historyCommand=historyScript, historyFile) {
  const historyPath = historyFile ? [historyFile] : []
  const { stdout } = await execa(historyCommand, historyPath)
  const lines = stdout.trim().split('\n')

  const dedup = []
  const linesSet = new Set()

  for (const line of lines.reverse()) {
    if (!linesSet.has(line)) {
      dedup.push(line)
      linesSet.add(line)
    }
  }

  return dedup
};

async function searchHistory (input='', historyCommand, historyFile) {
  const lines = await history(historyCommand, historyFile)

  const searcher = new AutoComplete({
    name: 'history',
    prefix: '\n',
    message: 'reverse search history',
    limit: 15,
    choices: lines,
    onRun (prompt) {
      if (input && input.length) {
        prompt.input = input
        prompt.cursor += input.length
        prompt.choices = prompt.suggest()
      }
    },
    stdin: process.stdin.isTTY ? process.stdin : getStdin(),
    stdout: process.stdout.isTTY ? process.stdout : getStdout(),
  })

  return await searcher.run()
}


module.exports = searchHistory
module.exports.history = history
