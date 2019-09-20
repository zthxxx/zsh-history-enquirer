const fs = require('fs')
const path = require('path')
const execa = require('execa')
const tty = require('tty')
const colors = require('ansi-colors');
const { AutoComplete, Select } = require('enquirer')
const ansi = require('enquirer/lib/ansi')


const historyScript = path.join(__dirname, 'history.zsh')
const cursorPosScript = path.join(__dirname, 'cursor.zsh')

getStdin = () => new tty.ReadStream(fs.openSync('/dev/tty', 'r'))
getStdout = () => new tty.WriteStream(fs.openSync('/dev/tty', 'w'))


async function getCursorPos () {
  const { stdout: position } = await execa(cursorPosScript)
  const [row, col] = position.split(' ')
  const [x, y] = [+col, +row]
  return { x, y }
}

async function history (historyCommand=historyScript, historyFile) {
  const historyPath = historyFile ? [historyFile] : []
  const { stdout } = await execa(historyCommand, historyPath)
  const lines = stdout.trim().split('\n')

  const linesSet = new Set()
  
  return lines
    .reverse()
    .filter(
      line => !linesSet.has(line) && linesSet.add(line)
    )
}

AutoComplete.prototype.pointer = Select.prototype.pointer

class HistorySearcher extends AutoComplete {
  constructor(options) {
    super(options)

    // start with initial col position rather than 0 default
    this.stdout.write(ansi.cursor.to(options.initCol))

    // overwrite, replace erase first line with erasePrompt (only erase from initial to end)
    ansi.clear = (input = '', columns = process.stdout.columns) => {
      const erasePrompt = ansi.cursor.to(options.initCol) + ansi.erase.lineEnd
      if (!columns) return erasePrompt
      let width = str => [...colors.unstyle(str)].length
      let lines = input.split(/\r?\n/)
      let rows = 0
      for (let line of lines) {
        rows += 1 + Math.floor(Math.max(width(line) - 1, 0) / columns)
      }
      return (ansi.erase.line + ansi.cursor.prevLine()).repeat(rows - 1) + erasePrompt
    }
  }
  
  number(ch) {
    return this.append(ch)
  }

  format() {
    const { input, cancelled } = this.state
    if (cancelled) {
      return input
    }
    if (input) return super.format();
    return ansi.code.show
  }

  restore() {
    super.restore()
    // append initial position
    this.stdout.write(ansi.cursor.right(this.options.initCol))
  }

  pageUp() {
    const { limit = 10 } = this.options
    for (let i = 0; i < limit; i++) {
      this.shiftUp()
    }
  }

  pageDown() {
    const { limit = 10 } = this.options
    for (let i = 0; i < limit; i++) {
      this.shiftDown()
    }
  }

  /**
   * when submit, restore curcor from output row to input row
   * 
   * when cancel, erase and leave origin input
   */
  async close() {
    const { input, submitted } = this.state
    await super.close()
    if (!input) return

    if (submitted) {
      this.stdout.write(ansi.erase.line + ansi.cursor.up())
    }
  }

  /**
   * @TODO: when cancel leave origin input to send
   */
  error(err) {
    return this.state.cancelled ? this.input : super.error(err)
  }
}

async function searchHistory (input='', historyCommand, historyFile) {
  const cursor = await getCursorPos()
  const lines = await history(historyCommand, historyFile)

  const searcher = new HistorySearcher({
    name: 'history',
    message: 'reverse search history',
    limit: 15,
    choices: lines,
    // shell prompt start col without input buffer
    initCol: cursor.x - input.length,
    promptLine: false,
    scroll: true,
    onRun (prompt) {
      if (input.length) {
        prompt.input = input
        prompt.cursor += input.length
        prompt.choices = prompt.suggest()
      }
    },
    stdin: process.stdin.isTTY ? process.stdin : getStdin(),
    stdout: process.stdout.isTTY ? process.stdout : getStdout(),
  })

  return searcher.run()
}


module.exports = searchHistory
module.exports.history = history
