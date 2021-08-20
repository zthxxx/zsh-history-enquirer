try {
  // cjs module `dist` for rollup bundle
  const index = require('./dist')
  module.exports = index
} catch (err) {
  if(err.code !== 'MODULE_NOT_FOUND'){
    throw err
  }
  // ts module `src` for dev
  if (!require.extensions['.ts']) {
    require('ts-node').register({
      project: require.resolve('./tsconfig.json'),
    })
  }
  const index = require('./src').default
  module.exports = index
}
