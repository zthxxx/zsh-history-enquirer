import path from 'path'
import execa from 'execa'


export default async function history(
  historyScript?: string,
  historyFile?: string
): Promise<string[]> {
  const historyCommand = historyScript || path.join(__dirname, 'history.zsh')
  const historyPath = historyFile ? [historyFile] : []
  const { stdout } = await execa(historyCommand, historyPath)
  const lines = stdout.trim().split('\n')

  const linesSet = new Set()

  return lines
    .reverse()
    .filter(
      line => !linesSet.has(line) && linesSet.add(line)
    )
    /**
     * replace '\\n' to support multiline command
     *
     * Example:
     *
     *   echo '\n  multi-line command\n  in history\n'
     *
     *   TO ->
     *
     *   echo '
     *     multi-line command
     *     in history
     *   '
     */
    .map(line => line.replaceAll('\\n', '\n'))
}
