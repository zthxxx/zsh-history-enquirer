#!/usr/bin/env node

require = require("esm")(module)

const searchHistory = require('..').default

const input = process.argv.slice(2).join(' ')

searchHistory(input)
  .then(console.log)
  .then(() => process.exit(0))
  .catch(console.log)
  .then(() => process.exit(0))
  .catch(() => process.exit(1))
