import type Events from 'events'
import type tty from 'tty'
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
  /** shell prompt takes columns, different with custom shell theme */
  initCol?: number,
}

export const SIGINT_CODE = 3
const { stringify } = JSON

// @TODO: pointer length from Prompt.pointer()
const POINTER_LENGTH = 2
const POINTER_PLACEHOLDER = ' '.repeat(POINTER_LENGTH)


/**
 * calculate how many rows the text takes up within terminal columns
 */
const calcTextTakeRows = (text: string, columns: number): number => {
  return `${POINTER_PLACEHOLDER}${text}`
    .split('\n')
    // empty line is also considered take a row
    .map(line => Math.ceil((line.length || 1) / columns))
    .reduce((a, b) => a + b, 0)
}

/**
 * performance optimization with
 * enquirer/lib/utils.js scrollUp
 */
const scrollUpInPlace = <T = any>(list: T[], size: number = 1) => {
  for (let i = 0; i < size; i ++) {
    list.unshift(list.pop())
  }
}

/**
 * performance optimization with
 * enquirer/lib/utils.js scrollDown
 */
const scrollDownInPlace = <T = any>(list: T[], size: number = 1) => {
  for (let i = 0; i < size; i ++) {
    list.push(list.shift())
  }
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
 *   - this.format()
 *   - this.renderChoices() (use this.visible)
 *      - this.choiceMessage()
 *        this.pointer()
 *   - this.clear()
 *      - ansi.clear()
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
  private index: number
  private pastingStep: null | 'starting' | 'started' | 'ending' | 'ended'
  private alert: () => void
  public keypress: (char: string | number, key?: Keypress) => void
  public run: PromptInstance['run']
  public once: PromptInstance['once']
  private emit: Events['emit']

  /**
   * throttle render() for combo and paste
   * skip AutoComplete.render highlight for performance
   */
  render = throttle(Select.prototype.render, 72, { leading: true })

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

    signale.debug('HistorySearcher size', { width: this.width, height: this.height })
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
    signale.debug('HistorySearcher reset()', { input: this.input })
  }

  /**
   * calc and change `this.limit`,
   * to dynamic limit choices rendered to avoid long and multiline choices
   * overflow the terminal container
   *
   * rewrite `enquirer/lib/prompts/select.js`
   */
  async renderChoices(): Promise<string> {
    const { options, index, width, height } = this
    /**
     * prompt occupy 3 lines reserved
     * be consistent in `this.down()`
     */
    const heightLimit = height - 3

    let limit = 0
    let rows = 0

    for (let choice of this.state.visible) {
      const choiceRows = calcTextTakeRows(choice, width)
      signale.debug('HistorySearcher: renderChoices()', 'choiceRows', choiceRows, { choice })
      const nextRows = rows + choiceRows

      signale.info('HistorySearcher renderChoices()', {
        choiceRows,
        choice: choice.slice(0, 10),
        rows: nextRows,
        width,
        height: this.height,
        limit,
      })

      if (nextRows >= heightLimit) {
        break
      }

      rows += choiceRows
      limit += 1

      if (limit >= options.limit) {
        signale.debug('HistorySearcher renderChoices() trigger max limit', { limit })
        break
      }
    }

    this.limit = limit

    /** when pageDown, origin index will greater than limit */
    if (index >= limit) {
      this.index = limit -1
    }

    /**
     * if empty choices to display, rows will be 0,
     * but it will show `No matching choices` message in next line,
     * so actually the minimum size is 1
     */
    this.state.size = Math.max(1, rows)

    return super.renderChoices()
  }

  choiceMessage(choice: ChoiceItem, index: number) {
    const input = this.input
    // @TODO: this.options.highlight
    const shader = this.styles.placeholder

    let message = choice
    for (const item of new Set(input.toLowerCase().split(' ').filter(Boolean))) {
      message = message.replace(item, shader(item))
    }
    return choice
  }

  /** format input area output */
  format() {
    const { input } = this
    const { submitted, cancelled } = this.state
    signale.debug('HistorySearcher format()', { input, submitted, cancelled })

    if (input) {
      return input
    }

    return ansi.code.show
  }

  /**
   * overwrite, replace erase first line with erasePrompt (only erase from initial to end)
   * used in this.render() - this.clear()
   */
  clear(rows: number = 0) {
    signale.debug('HistorySearcher clear()', { rows })
    const { options } = this

    const erasePrompt: string = ansi.cursor.to(options.initCol, null) + ansi.erase.lineEnd

    this.stdout.write(
      ansi.cursor.down(rows)
      + (ansi.erase.line + ansi.cursor.prevLine()).repeat(rows)
      + erasePrompt,
    )
  }

  /** restore cursor position */
  restore() {
    const { width, limit, options, cursor } = this

    // `state.size` computed in `this.renderChoices()`
    const rows = this.state.size

    signale.debug('HistorySearcher restore()', { limit, rows, width, cursor })

    this.stdout.write(
      ansi.cursor.up(rows)
      + ansi.cursor.to(options.initCol + cursor, null),
    )
  }

  number(ch: string) {
    return this.dispatch(ch)
  }

  dispatch(ch: string, key?: Keypress) {
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L104
    // https://github.com/enquirer/enquirer/blob/2.3.2/lib/keypress.js#L209
    const { sequence, ctrl, meta, option } = key || {} as Keypress

    // [BUG enquirer] bracketed paste mode
    // content will be wrapped by the sequences `\e[200~` and `\e[201~`
    // https://www.xfree86.org/current/ctlseqs.html#Bracketed%20Paste%20Mode
    if (sequence === '\u001b[200') {
      this.pastingStep = 'starting'
      signale.debug('Keypress start pasting \\e[200~')
      return
    } else if (this.pastingStep === 'starting' && sequence === '~') {
      this.pastingStep = 'started'
      signale.debug('Keypress in pasting')
      return
    } else if (this.pastingStep === 'started' && sequence === '\u001b[201') {
      this.pastingStep = 'ending'
      signale.debug('Keypress ending pasting \\e[201~')
      return
    } else if (this.pastingStep === 'ending' && sequence === '~') {
      this.pastingStep = 'ended'
      signale.debug('Keypress end pasted')
      return
    }

    signale.debug(
      'HistorySearcher dispatch',
      {
        char: ch,
        key,
      },
    )

    if (ch && !ctrl && !meta && !option) {
      return super.dispatch(ch)
    }
  }

  async complete() {
    this.visible = this.suggest(this.input, this.choices)
    this.index = Math.min(Math.max(this.visible.length - 1, 0), this.index)
    await this.render()
  }

  suggest(input: string, choices: ChoiceItem[]) {
    signale.debug('HistorySearcher suggest', { input })

    let result = choices

    const keywords = input.toLowerCase().split(' ').filter(Boolean)

    if (!keywords.length) {
      return result.slice()
    }

    for (const keyword of keywords) {
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

    signale.debug(
      'HistorySearcher up()',
      { len, vis, idx },
    )

    if (!len) {
      this.alert()
      return
    }

    if (len > vis && idx === 0) {
      scrollUpInPlace(choices)
      idx += 1
    }

    this.index = ((idx - 1 % len) + len) % len

    this.render()
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

    signale.debug(
      'HistorySearcher down()',
      { len, vis, idx },
    )

    if (!vis) {
      this.alert()
      return
    }

    if (len > vis && idx === vis - 1) {
      const nextChoice: ChoiceItem = choices[idx + 1]
      const nextRows: number = calcTextTakeRows(nextChoice, this.width)
      const visibleRowList: number[] = visible.map(
        choice => calcTextTakeRows(choice, this.width),
      )
      let totalRows = visibleRowList.reduce((a, b) => a + b, 0) + nextRows

      do {
        totalRows = totalRows - visibleRowList.shift()
        scrollDownInPlace(choices)
        idx -= 1
      } while (totalRows >= heightLimit)
    }

    this.index = (idx + 1) % len

    this.render()
  }

  pageUp() {
    const { limit } = this
    const choices = this.state.visible

    if (!choices.length) {
      this.alert()
      return
    }

    scrollUpInPlace(choices, limit)
    this.render()
  }

  pageDown() {
    const { limit } = this
    const choices = this.state.visible

    if (!choices.length) {
      this.alert()
      return
    }

    scrollDownInPlace(choices, limit)
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

  /**
   * when submit, restore cursor from output row to input row
   *
   * when cancel, erase and leave origin input
   */
  async close() {
    const { focused, input } = this
    const { submitted, cancelled } = this.state

    signale.debug('HistorySearcher close()', { input, focused, submitted, cancelled })
    this.emit('close')
  }

  async submit() {
    signale.debug('HistorySearcher submit()')
    this.state.submitted = true

    await this.close()

    const result = this.focused ?? this.input
    signale.debug('HistorySearcher submit()', { result })

    this.emit('submit', result)
  }

  async cancel(err?: string) {
    signale.debug('HistorySearcher cancel()', { err })
    this.state.cancelled = true

    await this.close()

    this.emit('cancel', await this.error(err))
  }

  error(err?: string): string {
    if (err !== undefined) {
      if (err === String.fromCharCode(SIGINT_CODE)) {
        signale.debug('HistorySearcher cancel, terminated with SIGINT')
      } else {
        signale.error('HistorySearcher ERROR', stringify(err), new Error(err).stack)
      }
    }
    return this.state.cancelled ? this.input : super.error(err)
  }
}
