import path from 'path'
import searchHistory from '..'
import type { SearchFunction } from '../src'


(searchHistory as any as SearchFunction)({
  input: 'zsh',
  historyFile: path.join(__dirname, 'history.txt'),
})
  .then(searcher => {
    searcher.once('run', async() => {
      await searcher.render()
      await searcher.submit()
    })

    return searcher.run()
  })
  .then(console.log)
