try {
  // cjs `dist` for rollup bundle
  const searchHistory = require('./dist')
  module.exports = searchHistory
} catch {
  // esm `src` for dev
  require = require('esm')(module)
  const searchHistory = require('./src').default
  module.exports = searchHistory
}
