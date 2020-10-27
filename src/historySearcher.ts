import type Events from 'events'
import type tty from 'tty'
import colors from 'ansi-colors'
import throttle from 'lodash/throttle'
import type { Prompt } from 'enquirer'
import ansi from 'enquirer/lib/ansi'
import AutoComplete from 'enquirer/lib/prompts/autocomplete'
import Select from 'enquirer/lib/prompts/select'
import signale from './signale'


export interface Keypress {
  sequence: string,
  name: string,
  ctrl?: boolean,
  meta?: boolean,
  shift?: boolean,
  option?: boolean,
  raw?: string,
  code?: string,
  action?: string,
}

export type ChoiceItem = string

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
  visible: ChoiceItem[],
  initCol: number,
  stdin: tty.ReadStream,
  stdout: tty.WriteStream,
  limit: number,
  onRun: Function,
  cancelled: boolean,
  submitted: boolean,
  loadingChoices: boolean,
}

export type PromptOptions = ConstructorParameters<typeof Prompt>[0] & {
  /** used but not list in enquirer types */
  promptLine: boolean,
  onRun: (prompt: AutoComplete) => void,
}
export type PromptInstance = InstanceType<typeof Prompt>

export interface ExtraOptions {
  input: string,
  choices: string[],
  limit?: number,
  highlight?: () => void,
  initCol?: number,
}

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
 * enquirer/lib/utils.js scrollUp
 */
const scrollUpInPlace = <T = any>(list: T[]): T[] => {
  const last = list.pop()
  list.unshift(last)
  return list
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

/**
 * @TODO: pipeline docs
 * this.run()
 * this.initialize()
 *   - this.reset()
 *     - set this.state.choices this.state.visible
 * this.start()
 *   - keypress.listen()
 * this.render()
 *   - this.renderChoices() (use this.visible)
 *      - this.choiceMessage()
 *        this.format()
 *        this.pointer()
 *   - this.restore
 *
 * this.keypress() - dispatch
 *   - this.append()
 *     this.delete()
 *     this.deleteForward()
 *       - this.complete()
 *         this.suggest
 *         set visible
 *     this.up()
 *     this.down()
 *     this.pageUP()
 *     this.pageDown()
 *     this.home()
 *     this.end()
 *        - scroll visible
 *          set index
 *
 * this.submit()
 * this.close()
 * this.emit('submit', this.focused())
 *
 */
export default class HistorySearcher extends AutoComplete {
  public options: PromptOptions & ExtraOptions
  private state: PromptState
  private width: number
  private height: number
  private stdout: tty.WriteStream
  private styles: any
  private input: string
  private cursor: number
  private limit: number
  private visible: ChoiceItem[]
  private value: ChoiceItem
  private index: number
  private pastingStep: null | 'starting' | 'started' | 'ending' | 'ended'
  private isDisabled: () => boolean
  private alert: () => void
  private scrollUp: () => void
  private scrollDown: () => void
  private sections: () => any
  public cancel: (err?: string) => void
  public keypress: (char: string | number, key?: Keypress) => void
  public run: PromptInstance['run']
  public once: PromptInstance['once']
  private emit: Events['emit']

  /**
   * throttle render() for combo and paste
   * skip AutoComplete.render highlight for performance
   */
  render = throttle(Select.prototype.render, 81, { leading: true })

  /**
   * need pointer style like Select, while AutoComplete is empty
   */
  pointer = Select.prototype.pointer

  constructor(options: Partial<PromptOptions> & ExtraOptions) {
    super(options)

    this.input = options.input
    this.cursor += options.input.length

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
    ansi.clear = (input: string, columns = process.stdout.columns) => {
      // [BUG enquirer] cursor not always at the beginning when prompt start
      const erasePrompt = ansi.cursor.to(options.initCol) + ansi.erase.lineEnd
      let width = str => [...colors.unstyle(str)].length
      let lines = input.split(/\r?\n/)
      let rows = 0
      for (let line of lines) {
        rows += 1 + Math.floor(Math.max(width(line) - 1, 0) / columns)
      }
      return (ansi.erase.line + ansi.cursor.prevLine()).repeat(rows - 1) + erasePrompt
    }

    signale.info('HistorySearcher size', { width: this.width, height: this.height })
  }

  get choices() {
    return this.state.choices
  }

  set choices(choices: ChoiceItem[]) {
    this.state.choices = choices
  }

  reset() {
    const { choices } = this.options
    this.choices = choices
    this.visible = this.suggest(this.input, choices)
    signale.info('HistorySearcher reset()', { input: this.input })
  }

  /**
   * calc and change `this.limit`,
   * to dynamic limit choices rendered to avoid long and multiline choices
   * overflow the terminal container
   *
   * rewrite `enquirer/lib/prompts/select.js`
   */
  async renderChoices(): Promise<string> {
    const { options, width, height } = this
    /**
     * prompt occupy 3 lines reserved
     * be consistent in `this.down()`
     */
    const heightLimit = height - 3

    let limit = 0
    let rows = 0

    for (let choice of this.state.visible) {
      const choiceRows = calcStringRowsTerminal(choice, width)
      rows += choiceRows

      signale.info('HistorySearcher renderChoices()', {
        choiceRows,
        choice: choice.slice(0, 10),
        rows,
        width,
        height: this.height,
        limit,
      })

      if (rows >= heightLimit) {
        break
      }

      limit += 1

      if (limit >= options.limit) {
        signale.info('HistorySearcher renderChoices() trigger max limit', { limit })
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

    let message = choice
    for (const item of new Set(input.toLowerCase().split(' ').filter(Boolean))) {
      message = message.replace(item, shader(item))
    }
    return choice
  }

  /** format input area output */
  format() {
    const { input, cancelled } = this.state
    signale.info('HistorySearcher format()', { input, cancelled })

    if (cancelled) {
      return input
    }

    if (input) return super.format()
    return ansi.code.show
  }

  /** clean screen output */
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

  number(ch: string) {
    return this.dispatch(ch)
  }

  dispatch(ch: string, key?: Keypress) {
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L104
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L209
    const { sequence } = key || {}
    // [BUG enquirer] bracketed paste mode
    // content will be wrapped by the sequences `\e[200~` and `\e[201~`
    // https://cirw.in/blog/bracketed-paste
    if (sequence === '\u001b[200') {
      this.pastingStep = 'starting'
      signale.info('Keypress start pasting \\e[200~')
      return
    } else if (this.pastingStep === 'starting' && sequence === '~') {
      this.pastingStep = 'started'
      signale.info('Keypress in pasting')
      return
    } else if (this.pastingStep === 'started' && sequence === '\u001b[201') {
      this.pastingStep = 'ending'
      signale.info('Keypress ending pasting \\e[201~')
      return
    } else if (this.pastingStep === 'ending' && sequence === '~') {
      this.pastingStep = 'ended'
      signale.info('Keypress end pasted')
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

  async complete() {
    this.visible = this.suggest(this.input, this.choices)
    this.index = Math.min(Math.max(this.visible.length - 1, 0), this.index)
    await this.render()
  }

  suggest(input = this.input, choices = this.choices) {
    signale.info('HistorySearcher suggest', { input })

    let result = choices

    const keywords = input.toLowerCase().split(' ').filter(Boolean)

    if (!keywords.length) {
      return result.slice()
    }

    for (const keyword of input.toLowerCase().split(' ').filter(Boolean)) {
      result = result.filter((message) => message.toLowerCase().includes(keyword))
    }
    return result
  }

  /**
   * scroll optimize performance for in place
   */
  up() {
    const choices = this.state.visible
    const len = choices.length
    const vis = this.visible.length
    let idx = this.index

    signale.info(
      'HistorySearcher up()',
      { len, vis, idx },
    )

    if (!len) {
      return this.alert()
    }

    if (len > vis && idx === 0) {
      scrollUpInPlace(choices)
      idx += 1
    }

    this.index = ((idx - 1 % len) + len) % len

    return this.render()
  }

  /**
   * scroll when multiline command
   * and optimize performance for in place
   */
  down() {
    const choices = this.state.visible
    const visible = this.visible
    const len = choices.length
    const vis = visible.length
    let idx = this.index

    /**
     * prompt occupy 3 lines reserved
     * be consistent in `this.renderChoices()`
     */
    const heightLimit = this.height - 3

    signale.info(
      'HistorySearcher down()',
      { len, vis, idx },
    )

    if (!vis) {
      return this.alert()
    }

    if (len > vis && idx === vis - 1) {
      const nextChoice: ChoiceItem = choices[idx + 1]
      const nextRows: number = calcStringRowsTerminal(nextChoice, this.width)
      const visibleRowList: number[] = visible.map(
        choice => calcStringRowsTerminal(choice, this.width),
      )
      let totalRows = visibleRowList.reduce((a, b) => a + b, 0) + nextRows

      do {
        totalRows = totalRows - visibleRowList.shift()
        scrollDownInPlace(choices)
        idx -= 1
      } while (totalRows >= heightLimit)
    }

    this.index = (idx + 1) % len

    return this.render()
  }

  pageUp() {
    const { limit } = this
    this.visible = [
      ...this.state.visible.slice(-limit),
      ...this.state.visible.slice(0, -limit),
    ]
    this.render()
  }

  pageDown() {
    const { limit } = this
    this.visible = [
      ...this.state.visible.slice(limit),
      ...this.state.visible.slice(0, limit),
    ]
    this.render()
  }

  home() {
    this.visible = this.suggest(this.input, this.choices)
    this.index = 0
    return this.render()
  }

  end() {
    const choices = this.suggest(this.input, this.choices)
    const pos = choices.length - this.limit
    this.visible = choices.slice(pos).concat(choices.slice(0, pos))
    this.index = this.limit - 1
    return this.render()
  }

  get focused() {
    return this.visible[this.index]
  }

  async submit() {
    this.state.submitted = true
    await this.render()
    await this.close()

    const result = this.focused ?? this.input
    signale.info('HistorySearcher submit()', { result })

    this.emit('submit', result)
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
