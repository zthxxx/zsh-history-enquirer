#!/usr/bin/env node

const searchHistory = require('..')

const input = process.argv.slice(2).join(' ')

searchHistory({
  input,
})
  .then(searcher => searcher.run())
  .then(console.log)
  .then(() => process.exit(0))
  .catch(console.log)
  .then(() => process.exit(0))
