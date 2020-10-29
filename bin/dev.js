#!/usr/bin/env node

const searchHistory = require('..')

const input = process.argv.slice(2).join(' ')

searchHistory({
  input,
  historyFile: 'tests/history.txt',
})
  .then((searcher) => {
    searcher.stdout.rows = 15
    searcher.stdout.columns = 80
    return searcher.run()
  })
  .then(console.log)
  .then(() => process.exit(0))
  .catch(console.log)
  .then(() => process.exit(0))
