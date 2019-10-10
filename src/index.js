import getCursorPos from './cursorPosition'
import history from './zshHistory'
import HistorySearcher from './historySearcher'
import { getStdin, getStdout } from './tty'
import signale from './signale'


const { stringify } = JSON

export default async function searchHistory({ input = '', historyCommand, historyFile }) {
  signale.info('searchHistory start')

  const [
    cursor,
    lines,
  ] = await Promise.all([
    getCursorPos(),
    history(historyCommand, historyFile),
  ])

  signale.info('searchHistory cursor', cursor)
  signale.info(
    'searchHistory lines',
    lines.length,
    stringify(lines[0]),
    stringify(lines[lines.length - 1]),
  )

  const stdin = process.stdin.isTTY ? process.stdin : getStdin()
  const stdout = process.stdout.isTTY ? process.stdout : getStdout()
  signale.info(
    'searchHistory stdin',
    stdin.constructor.name,
    stdin.isTTY,
  )
  signale.info(
    'searchHistory stdout',
    stdout.constructor.name,
    stdout.isTTY,
  )

  const searcher = new HistorySearcher({
    name: 'history',
    message: 'reverse search history',
    promptLine: false,
    get limit() {
      return Math.min(15, stdout.rows - 2)
    },
    choices: lines,
    // shell prompt start col without input buffer
    initCol: cursor.x - input.length,
    onRun(prompt) {
      signale.info('HistorySearcher onRun start')
      signale.info('HistorySearcher start input', input)

      if (input.length) {
        prompt.input = input
        prompt.cursor += input.length
        prompt.choices = prompt.suggest()
      }

      signale.info(
        'HistorySearcher onRun choices',
        prompt.choices.length,
        stringify(prompt.choices[0].value),
      )
    },
    stdin,
    stdout,
  })

  return searcher
}

