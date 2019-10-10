import colors from 'ansi-colors'
import ansi from 'enquirer/lib/ansi'
import AutoComplete from 'enquirer/lib/prompts/autocomplete'
import Select from 'enquirer/lib/prompts/select'
import signale from './signale'


AutoComplete.prototype.pointer = Select.prototype.pointer

const SIGINT_CODE = 3

export default class HistorySearcher extends AutoComplete {
  constructor(options) {
    super(options)

    signale.info('HistorySearcher size', { width: this.width, height: this.height })

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

  dispatch(ch) {
    signale.info('HistorySearcher dispatch', ch)
    return super.dispatch(ch);
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
    const { rest } = this.sections()
    super.restore()

    // [BUG]`prompt.restore` dont calculate if line width more than termainal columns
    const rows = rest
      .map(line => colors.unstyle(line).length)
      .map(width => Math.max(width - 2, 0))
      .map(width => Math.ceil(width / this.width))
      .reduce((a, b) => a + b, 0)
    this.state.size = rows
    this.stdout.write(ansi.cursor.up(rows - rest.length))

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
      signale.info('HistorySearcher submitted')
    }
  }

  error(err) {
    if (err !== undefined) {
      if (err === String.fromCharCode(SIGINT_CODE)) {
        signale.info('HistorySearcher cancel')
      } else {
        signale.error('HistorySearcher ERROR', err, new Error(err).stack)
      }
    }
    return this.state.cancelled ? this.input : super.error(err)
  }
}

