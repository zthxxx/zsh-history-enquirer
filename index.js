try {
  // cjs module `dist` for rollup bundle
  const searchHistory = require('./dist')
  module.exports = searchHistory
} catch {
  // ts module `src` for dev
  if (!require.extensions['.ts']) {
    require('ts-node').register({
      project: require('path').join(__dirname, 'tsconfig.json'),
    })
  }
  const searchHistory = require('./src').default
  module.exports = searchHistory
}
