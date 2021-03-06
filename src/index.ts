import getCursorPos from './cursorPosition'
import history from './zshHistory'
import HistorySearcher from './historySearcher'
import { getStdin, getStdout } from './tty'
import signale from './signale'


export interface SearchOptions {
  input?: string,
  historyCommand?: string,
  historyFile?: string,
}

export type SearchFunction = (options: SearchOptions) => Promise<HistorySearcher>


export default async function searchHistory(options: SearchOptions): ReturnType<SearchFunction> {
  signale.debug('searchHistory start')
  const { input = '', historyCommand, historyFile } = options

  const [
    cursor,
    lines,
  ] = await Promise.all([
    getCursorPos(),
    history(historyCommand, historyFile),
  ])

  signale.debug('searchHistory cursor', cursor)
  signale.debug(
    'searchHistory lines',
    {
      lines: lines.length,
      first: lines[0]?.slice(0, 50),
    },
  )

  const stdin = process.stdin.isTTY ? process.stdin : getStdin()
  const stdout = process.stdout.isTTY ? process.stdout : getStdout()
  signale.debug(
    'searchHistory stdin',
    stdin.constructor.name,
    ['isTTY', stdin.isTTY],
  )
  signale.debug(
    'searchHistory stdout',
    stdout.constructor.name,
    ['isTTY', stdout.isTTY],
  )

  return new HistorySearcher({
    name: 'history',
    message: 'reverse search history',
    promptLine: false,
    input,
    choices: lines,
    // shell prompt start col without input buffer
    initCol: cursor.x - input.length,
    stdin,
    stdout,
    limit: 15,
    onRun(prompt) {
      signale.debug('HistorySearcher onRun start')
      signale.debug('HistorySearcher start', { input })

      signale.debug(
        'HistorySearcher onRun choices',
        {
          choices: prompt.choices.length,
          first: prompt.choices[0]?.value?.slice(0, 50),
        },
      )
    },
  })
}

