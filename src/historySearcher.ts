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

// @TODO: pointer length from Prompt.pointer()
const POINTER_LENGTH = 2
const POINTER_PLACEHOLDER = Array.from({ length: POINTER_LENGTH })
  .fill(' ')
  .join('')


const calcStringRowsTerminal = (input: string, columns: number): number => {
  return `${POINTER_PLACEHOLDER}${input}`
    .split('\n')
    .map(line => Math.ceil(line.length / columns))
    .reduce((a, b) => a + b, 0)
}

/**
 * performance optimization with
 * enquirer/lib/utils.js scrollDown
 */
const scrollDownInPlace = <T = any>(list: T[]): T[] => {
  const first = list.shift()
  list.push(first)
  return list
}

export default class HistorySearcher extends AutoComplete {
  private state: PromptState
  private options: PromptOptions & ExtraOptions
  private width: number
  private height: number
  private stdout: tty.WriteStream
  private styles: any
  private input: string
  private limit: number
  private choices: ChoiceItem[]
  private visible: ChoiceItem[]
  private focused: ChoiceItem
  private value: ChoiceItem['name']
  private index: number
  private pastingStep: null | 'starting' | 'started' | 'ending' | 'ended'
  private isDisabled: () => boolean
  private alert: () => void
  private up: () => void
  private scrollUp: () => void
  private scrollDown: () => void
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
    } else if (this.pastingStep === 'starting' && sequence === '~') {
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

    signale.info(
      'HistorySearcher dispatch',
      {
        char: stringify(ch),
        key,
      },
    )
    return super.dispatch(ch)
  }

  suggest(input = this.input, choices = this.state.choices) {
    let result = choices
    for (const item of input.toLowerCase().split(' ').filter(Boolean)) {
      result = result.filter(({ message }) => message.toLowerCase().includes(item))
    }
    return result
  }

  /**
   * calc and change `this.limit`,
   * to dynamic limit choices rendered to avoid long and multiline choices
   * overflow the terminal container
   *
   * rewrite `enquirer/lib/prompts/select.js`
   */
  async renderChoices(): Promise<string> {
    const { options } = this

    let limit = 0
    let rows = 0
    const { width } = this

    for (let choice of this.choices) {
      rows += calcStringRowsTerminal(choice.message, width)

      // prompt occupy about 3 lines
      if (rows > this.height - 3) {
        break
      }

      limit += 1

      if (limit >= options.limit) {
        break
      }
    }

    this.limit = limit

    return super.renderChoices()
  }

  choiceMessage(choice: ChoiceItem, index: number) {
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
    const { width, limit } = this
    const { rest } = this.sections()
    super.restore()

    // [BUG enquirer]`prompt.restore` dont calculate if line width more than termainal columns
    const rows = rest
      .map(line => colors.unstyle(line))
      .map(line => calcStringRowsTerminal(line, width))
      .reduce((a, b) => a + b, 0)

    this.state.size = rows
    this.stdout.write(ansi.cursor.up(rows - rest.length))

    signale.info(
      'HistorySearcher restore()',
      {
        limit,
        rows,
        'rest.length': rest.length,
      },
    )

    // append initial position
    this.stdout.write(ansi.cursor.right(this.options.initCol))
  }

  pageUp() {
    const { limit } = this
    this.choices = [...this.choices.slice(-limit), ...this.choices.slice(0, -limit)]
    while (this.isDisabled()) {
      this.up()
    }
    this.render()
  }

  pageDown() {
    const { limit } = this
    this.choices = [...this.choices.slice(limit), ...this.choices.slice(0, limit)]
    while (this.isDisabled()) {
      this.down()
    }
    this.render()
  }

  /**
   * scroll when multiline command
   */
  down() {
    const len = this.choices.length
    const vis = this.visible.length
    let idx = this.index

    // prompt occupy about 3 lines
    const heightLimit = this.height - 3

    signale.info(
      'HistorySearcher down()',
      { len, vis, idx },
    )

    if (len > vis && idx === vis - 1) {
      const nextChoice: ChoiceItem = this.choices[idx + 1]
      const nextRows: number = calcStringRowsTerminal(nextChoice.message, this.width)
      const visibleRowList: number[] = this.visible.map(
        choice => calcStringRowsTerminal(choice.message, this.width),
      )
      let totalRows = visibleRowList.reduce((a, b) => a + b, 0) + nextRows

      do {
        if (!visibleRowList.length) {
          return this.alert()
        }
        totalRows = totalRows - visibleRowList.shift()
        scrollDownInPlace(this.choices)
        idx -= 1
      } while (totalRows > heightLimit)
    }

    this.index = (idx + 1) % len

    return this.render()
  }

  /**
   * when submit, restore curcor from output row to input row
   *
   * when cancel, erase and leave origin input
   */
  async close() {
    const { value } = this
    const { input, submitted } = this.state
    await super.close()
    if (!input) return

    if (submitted) {
      this.stdout.write(ansi.erase.line)
      this.stdout.write(
        ansi.cursor.up()
          .repeat(calcStringRowsTerminal(
            value,
            this.width,
          )),
      )
    }
  }

  error(err) {
    if (err !== undefined) {
      if (err === String.fromCharCode(SIGINT_CODE)) {
        signale.info('HistorySearcher cancel, terminated with SIGINT')
      } else {
        signale.error('HistorySearcher ERROR', stringify(err), new Error(err).stack)
      }
    }
    return this.state.cancelled ? this.input : super.error(err)
  }
}

