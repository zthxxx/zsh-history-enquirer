const path = require('path');
const execa = require('execa')
const { AutoComplete } = require('enquirer');

async function history(historyPath) {
  const { stdout } = await execa(path.join(__dirname, 'history.zsh'), historyPath ? [historyPath] : [])
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


history()
  .then(lines => {
    const prompt = new AutoComplete({
      name: 'history',
      message: 'reverse search history',
      limit: 12,
      choices: lines,
    })

    return prompt.run()
  })
  .then(console.log)
  .catch(console.error)
