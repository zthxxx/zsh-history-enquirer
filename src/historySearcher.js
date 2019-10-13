import colors from 'ansi-colors'
import ansi from 'enquirer/lib/ansi'
import AutoComplete from 'enquirer/lib/prompts/autocomplete'
import Select from 'enquirer/lib/prompts/select'
import signale from './signale'


AutoComplete.prototype.pointer = Select.prototype.pointer
AutoComplete.prototype.render = Select.prototype.render

const SIGINT_CODE = 3
const { stringify } = JSON

export default class HistorySearcher extends AutoComplete {
  constructor(options) {
    super(options)

    signale.info('HistorySearcher size', { width: this.width, height: this.height })

    /**
     * isPasting: null | true | false
     *
     * true - bracketed paste mode \e[200
     * false - ending but not complete of bracketed paste mode \e[201
     * null - finished paste mode
     */
    this.isPasting = null

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
    return this.dispatch(ch)
  }

  dispatch(ch, key) {
    const { sequence } = key
    // [BUG enquirer] bracketed paste mode
    // content will be wrapped by the sequences `\e[200~` and `\e[201~`
    // https://cirw.in/blog/bracketed-paste
    if (sequence === '\u001b[200') {
      this.isPasting = true
      signale.info('Keyperss start pasting \\e[200~')
      return
    } else if (sequence === '~' && this.isPasting){
      signale.info('Keyperss in pasting')
      return
    } else if (sequence === '\u001b[201') {
      signale.info('Keyperss ending pasting \\e[201~')
      this.isPasting = false
      return
    } else if (sequence === '~' && this.isPasting === false) {
      this.isPasting = null
      signale.info('Keyperss end pasted')
      return
    }

    signale.info('HistorySearcher dispatch', stringify(ch), key)
    return super.dispatch(ch);
  }

  suggest(input = this.input, choices = this.state._choices) {
    let result = choices
    for (const item of input.toLowerCase().split(' ').filter(Boolean)) {
      result = result.filter(({ message }) => message.toLowerCase().includes(item))
    }
    return result
  }

  choiceMessage(choice, i) {
    const input = this.input
    const shader = this.options.highlight
      ? this.options.highlight.bind(this)
      : this.styles.placeholder

    let message = choice.message
    for (const item of new Set(input.toLowerCase().split(' ').filter(Boolean))) {
      message = message.replace(item, shader(item))
    }
    return super.choiceMessage({ ...choice, message }, i)
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

    // [BUG enquirer]`prompt.restore` dont calculate if line width more than termainal columns
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
      signale.info('HistorySearcher submitted input', stringify(input))
    }
  }

  error(err) {
    if (err !== undefined) {
      if (err === String.fromCharCode(SIGINT_CODE)) {
        signale.info('HistorySearcher cancel')
      } else {
        signale.error('HistorySearcher ERROR', stringify(err), new Error(err).stack)
      }
    }
    return this.state.cancelled ? this.input : super.error(err)
  }
}

