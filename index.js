try {
  // cjs `dist` for rollup bundle
  const searchHistory = require('./dist')
  module.exports = searchHistory
} catch {
  // ts `src` for dev
  require('ts-node/register')
  const searchHistory = require('./src').default
  module.exports = searchHistory
}
