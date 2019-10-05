const path = require('path')
const searchHistory = require('..')


searchHistory({
  input: 'echo',
  historyFile: path.join(__dirname, 'history.txt'),
})
  .then(searcher => {
    searcher.once('run', async() => {
      await searcher.render();
      await searcher.submit();
    });

    return searcher.run()
  })
  .then(console.log)
