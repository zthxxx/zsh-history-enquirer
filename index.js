const path = require('path');
const execa = require('execa')
const { AutoComplete } = require('enquirer');


const historyScript = path.join(__dirname, 'history.zsh')

async function history (historyCommand=historyScript, historyFile) {
  const historyPath = historyFile ? [historyFile] : []
  const { stdout } = await execa(historyCommand, historyPath)
  const lines = stdout.trim().split('\n')

  const dedup = []
  const linesSet = new Set()

  for (const line of lines.reverse()) {
    if (!linesSet.has(line)) {
      dedup.push(line)
      linesSet.add(line)
    }
  }

  return dedup
};

async function searchHistory (input='', historyCommand, historyFile) {
  const lines = await history(historyCommand, historyFile)

  const searcher = new AutoComplete({
    name: 'history',
    message: 'reverse search history',
    limit: 15,
    choices: lines,
    onRun (prompt) {
      if (input && input.length) {
        prompt.input = input
        prompt.cursor = input.length
        prompt.choices = prompt.suggest()
      }
    }
  })

  return await searcher.run()
}


module.exports = searchHistory
module.exports.history = history
