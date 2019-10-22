import colors from 'ansi-colors'
import tty from 'tty'
import { Prompt } from 'enquirer'
import ansi from 'enquirer/lib/ansi'
import AutoComplete from 'enquirer/lib/prompts/autocomplete'
import Select from 'enquirer/lib/prompts/select'
import signale from './signale'


export interface Keyperss {
  name: string,
  ctrl: boolean,
  meta: boolean,
  shift: boolean,
  option: boolean,
  sequence: string,
  raw: string,
  code: string,
  action: string,
}

export interface ChoiceItem {
  name: string,
  normalized: boolean,
  message: string,
  value: string,
  input: string,
  index: number,
  cursor: number,
  level: number,
  indent: string,
  path: string,
  enabled: boolean,
  reset: Function[],
}

export interface PromptState {
  name: string,
  message: string,
  header: string,
  footer: string,
  error: string,
  hint: string,
  input: string,
  cursor: number,
  index: number,
  lines: number,
  size: number,
  tick: number,
  prompt: string,
  buffer: string,
  width: number,
  promptLine: boolean,
  choices: ChoiceItem[],
  initCol: number,
  stdin: tty.ReadStream,
  stdout: tty.WriteStream,
  limit: number,
  onRun: Function,
  cancelled: boolean,
  submitted: boolean,
  loadingChoices: boolean,
}

export type PromptOptions = ConstructorParameters<typeof Prompt>[0]
export type PromptInstance = InstanceType<typeof Prompt>

export interface ExtraOptions {
  limit?: number,
  highlight?: () => void,
  initCol?: number,
}

AutoComplete.prototype.pointer = Select.prototype.pointer
AutoComplete.prototype.render = Select.prototype.render

export const SIGINT_CODE = 3
const { stringify } = JSON

export default class HistorySearcher extends AutoComplete {
  private state: PromptState
  private options: PromptOptions & ExtraOptions
  private width: number
  private height: number
  private input: string
  private stdout: tty.WriteStream
  private styles: any
  private pastingStep: null | 'starting' | 'started' | 'ending' | 'ended'
  private shiftUp: () => void
  private shiftDown: () => void
  private sections: () => any
  public submit: () => void
  public cancel: (err?: string) => void
  public keypress: (char: string | number, key?: Keyperss) => void
  public render: PromptInstance['render']
  public run: PromptInstance['run']
  public once: PromptInstance['once']


  constructor(options) {
    super(options)

    signale.info('HistorySearcher size', { width: this.width, height: this.height })

    /**
     * pastingStep: null | starting | started | ending | ended
     *
     * starting - bracketed paste mode \e[200
     * started - get first ~ after \e[200
     * ending - ending but not complete of bracketed paste mode \e[201
     * ended | null - finished paste, get first ~ after \e[201
     */
    this.pastingStep = null

    // start with initial col position rather than 0 default
    this.stdout.write(ansi.cursor.to(options.initCol))

    // overwrite, replace erase first line with erasePrompt (only erase from initial to end)
    ansi.clear = (input = '', columns = process.stdout.columns) => {
      // [BUG enquirer] cursor not always at the beginning when prompt start
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

  number(ch: string) {
    return this.dispatch(ch)
  }

  dispatch(ch: string, key?: Keyperss) {
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L104
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L209
    const { sequence } = key || {}
    // [BUG enquirer] bracketed paste mode
    // content will be wrapped by the sequences `\e[200~` and `\e[201~`
    // https://cirw.in/blog/bracketed-paste
    if (sequence === '\u001b[200') {
      this.pastingStep = 'starting'
      signale.info('Keyperss start pasting \\e[200~')
      return
    } else if (this.pastingStep === 'starting' && sequence === '~'){
      this.pastingStep = 'started'
      signale.info('Keyperss in pasting')
      return
    } else if (this.pastingStep === 'started' && sequence === '\u001b[201') {
      this.pastingStep = 'ending'
      signale.info('Keyperss ending pasting \\e[201~')
      return
    } else if (this.pastingStep === 'ending' && sequence === '~') {
      this.pastingStep = 'ended'
      signale.info('Keyperss end pasted')
      return
    }

    signale.info('HistorySearcher dispatch', stringify(ch), key)
    return super.dispatch(ch)
  }

  suggest(input = this.input, choices = this.state.choices) {
    let result = choices
    for (const item of input.toLowerCase().split(' ').filter(Boolean)) {
      result = result.filter(({ message }) => message.toLowerCase().includes(item))
    }
    return result
  }

  choiceMessage(choice: ChoiceItem, index: number) {
    signale.info('HistorySearcher choiceMessage', choice, index)

    const input = this.input
    const shader = this.options.highlight
      ? this.options.highlight.bind(this)
      : this.styles.placeholder

    let message = choice.message
    for (const item of new Set(input.toLowerCase().split(' ').filter(Boolean))) {
      message = message.replace(item, shader(item))
    }
    return super.choiceMessage({ ...choice, message }, index)
  }

  format() {
    const { input, cancelled } = this.state
    if (cancelled) {
      return input
    }
    if (input) return super.format()
    return ansi.code.show
  }

  restore() {
    // @TODO: pointer length from Prompt.pointer()
    const POINTER_LENGTH = 2
    const { rest } = this.sections()
    super.restore()

    // [BUG enquirer]`prompt.restore` dont calculate if line width more than termainal columns
    const rows = rest
      .map(line => colors.unstyle(line).length)
      .map(width => width + POINTER_LENGTH)
      .map(width => Math.ceil(width / this.width))
      .reduce((a, b) => a + b, 0)

    this.state.size = rows
    this.stdout.write(ansi.cursor.up(rows - rest.length))

    signale.info('HistorySearcher restore [rows, rest.length]', [rows, rest.length])

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

