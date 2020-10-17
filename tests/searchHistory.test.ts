import path from 'path'
import search from '..'
import { Keyperss, SIGINT_CODE } from '../src/historySearcher'
import type { SearchFunction } from '../src'


const searchHistory = search as any as SearchFunction
const testHistoryFile = path.join(__dirname, 'history.txt')

const pasteStartKey: Keyperss = {
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

const pasteEndKey: Keyperss = {
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

/**
 * normalized and mock tty for CI runner
 * because in GitHub Action CI, stdin / stdout is not tty,
 * and also cannot read /dev/ttys
 */
beforeAll(() => {
  process.stdout.rows = 30
  process.stdout.columns = 80
  process.stdin.isTTY = true
  process.stdout.isTTY = true
  process.stdin.setRawMode = () => process.stdin
})

test('search `echo` in history', async () => {
  const searcher = await searchHistory({
    input: 'author',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async() => {
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('echo author zthxxx')
})

test('search multiple words in history', async () => {
  const searcher = await searchHistory({
    input: 'log iso',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async() => {
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('git log --pretty=fuller --date=iso -n 1')
})

test('paste text in terminal', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.keypress(null, pasteStartKey)
    await searcher.keypress('~')
    await searcher.keypress('s')
    await searcher.keypress('t')
    await searcher.keypress(null, pasteEndKey)
    await searcher.keypress('~')
    await searcher.keypress('a')
    await searcher.keypress('t')

    await searcher.render()
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('git status')
})

test('number and paste number in terminal', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.keypress(2)
    await searcher.keypress(3)

    await searcher.keypress(null, pasteStartKey)
    await searcher.keypress('~')
    await searcher.keypress(3)
    await searcher.keypress(3)
    await searcher.keypress(null, pasteEndKey)
    await searcher.keypress('~')

    await searcher.render()
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('233333')
})

test('pageUp', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.pageUp()
    await searcher.pageUp()
    await searcher.render()
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('pwgen --help')
})

test('pageDown', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.pageDown()
    await searcher.render()
    await searcher.submit()
  })

  const result = await searcher.run()
  expect(result).toBe('233333')
})

test('test cancel', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.keypress(2)
    await searcher.keypress(3)
    await searcher.keypress(3)
    await searcher.keypress(3)
    await searcher.cancel('some error message')
  })
  try {
    await searcher.run()
    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('2333')
  }
})

test('test cancel ctrl+c', async () => {
  const searcher = await searchHistory({
    input: '',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.keypress(2)
    await searcher.keypress(3)
    await searcher.keypress(3)
    await searcher.keypress(3)
    await searcher.cancel(String.fromCharCode(SIGINT_CODE))
  })
  try {
    await searcher.run()
    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('2333')
  }
})


test('search not match in history', async () => {
  const searcher = await searchHistory({
    input: '3jdfn2-9jgf',
    historyFile: testHistoryFile,
  })

  searcher.once('run', async () => {
    await searcher.submit()
    await searcher.cancel()
  })

  try {
    await searcher.run()
    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('3jdfn2-9jgf')
  }
})
