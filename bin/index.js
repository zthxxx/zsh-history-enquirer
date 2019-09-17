#!/usr/bin/env node

const searchHistory = require('..')

const input = process.argv.slice(2).join(' ')

searchHistory(input)
  .then(console.log)
  .catch(console.error)
