import colors from 'ansi-colors'
import ansi from 'enquirer/lib/ansi'
import { AutoComplete, Select } from 'enquirer'


AutoComplete.prototype.pointer = Select.prototype.pointer

export default class HistorySearcher extends AutoComplete {
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

