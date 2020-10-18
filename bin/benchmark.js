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
const searchHistory = require('..')

searchHistory({
  input: '',
  // history file created by `scripts/create-benchmark-history.zsh`
  historyFile: 'tests/benchmark-history.data',
})
  .then((searcher) => {
    searcher.once('run', async () => {
      await searcher.render()

      await searcher.submit()
    })
    return searcher.run()
  })
  .then(console.log)
