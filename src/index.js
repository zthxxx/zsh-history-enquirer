import getCursorPos from './cursorPosition'
import history from './zshHistory'
import HistorySearcher from './HistorySearcher'
import { getStdin, getStdout } from './tty'


export default async function searchHistory (input='', historyCommand, historyFile) {
  const cursor = await getCursorPos()
  const lines = await history(historyCommand, historyFile)

  const searcher = new HistorySearcher({
    name: 'history',
    message: 'reverse search history',
    promptLine: false,
    limit: 15,
    choices: lines,
    // shell prompt start col without input buffer
    initCol: cursor.x - input.length,
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

