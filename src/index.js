import getCursorPos from './cursorPosition'
import history from './zshHistory'
import HistorySearcher from './historySearcher'
import { getStdin, getStdout } from './tty'


export default async function searchHistory({ input = '', historyCommand, historyFile }) {
  const [
    cursor,
    lines,
    stdin,
    stdout
  ] = await Promise.all([
    getCursorPos(),
    history(historyCommand, historyFile),
    process.stdin.isTTY ? process.stdin : getStdin(),
    process.stdout.isTTY ? process.stdout : getStdout(),
  ])

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
      if (input.length) {
        prompt.input = input
        prompt.cursor += input.length
        prompt.choices = prompt.suggest()
      }
    },
    stdin,
    stdout,
  })

  return searcher
}

