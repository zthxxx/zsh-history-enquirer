const path = require('path')
const searchHistory = require('..')

const pasteStartKey = {
  name: 'undefined',
  ctrl: false,
  meta: false,
  shift: false,
  option: false,
  sequence: '\u001b[200',
  raw: '',
  code: '[200',
  action: undefined
}

const pasteEndKey = {
  name: 'undefined',
  ctrl: false,
  meta: false,
  shift: false,
  option: false,
  sequence: '\u001b[201',
  raw: '',
  code: '[201',
  action: undefined
}

test('search `echo` in history', async () => {
  const searcher = await searchHistory({
    input: 'author',
    historyFile: path.join(__dirname, 'history.txt'),
  })

  searcher.once('run', async() => {
    await searcher.render();
    await searcher.submit();
  });

  const result = await searcher.run()
  expect(result).toBe('echo author zthxxx')
})

test('search multiple words in history', async () => {
  const searcher = await searchHistory({
    input: 'log iso',
    historyFile: path.join(__dirname, 'history.txt'),
  })

  searcher.once('run', async() => {
    await searcher.render();
    await searcher.submit();
  });

  const result = await searcher.run()
  expect(result).toBe('git log --pretty=fuller --date=iso -n 1')
})

test('paste text in terminal', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: path.join(__dirname, 'history.txt'),
  })

  searcher.once('run', async() => {
    await searcher.render();

    await searcher.keypress(null, pasteStartKey);
    await searcher.keypress('~');
    await searcher.keypress('s');
    await searcher.keypress('t');
    await searcher.keypress(null, pasteEndKey);
    await searcher.keypress('~');
    await searcher.keypress('a');
    await searcher.keypress('t');

    await searcher.render();
    await searcher.submit();
  });

  const result = await searcher.run()
  expect(result).toBe('git status')
})

test('number and paste number in terminal', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: path.join(__dirname, 'history.txt'),
  })

  searcher.once('run', async() => {
    await searcher.render();

    await searcher.keypress(2);
    await searcher.keypress(3);

    await searcher.keypress(null, pasteStartKey);
    await searcher.keypress('~');
    await searcher.keypress(3);
    await searcher.keypress(3);
    await searcher.keypress(null, pasteEndKey);
    await searcher.keypress('~');

    await searcher.render();
    await searcher.submit();
  });

  const result = await searcher.run()
  expect(result).toBe('233333')
})
