#!/usr/bin/env node

/**
 * Usage
 *
 * when cwd is project root
 *
 * ```
 *   npm run build
 *   ./scripts/create-benchmark-history.zsh
 *   time ./bin/benchmark.js
 * ```
 */
const searchHistory = require('../dist')

searchHistory({
  input: '',
  // history file created by `scripts/create-benchmark-history.zsh`
  historyFile: 'tests/benchmark-history.data',
})
  .then((searcher) => {
    searcher.once('run', () => {
      searcher.submit()
    })
    return searcher.run()
  })
  .then(console.log)
  .then(() => process.exit(0))
