import path from 'path'
import search from '..'
import type { SearchFunction } from '../src'
import type { default as HistorySearcher, Keypress } from '../src/historySearcher'
import {
  pasteStart,
  pasteEnd,
  ctrlC,
  esc,
} from './metaKeys'


const testHistoryFile = path.join(__dirname, 'history.txt')

const searchHistory = async (
  input: string,
  onRun: (searcher: HistorySearcher) => void,
): Promise<string> => {
  const searcher = await (search as any as SearchFunction)({
    input,
    historyFile: testHistoryFile,
  })

  searcher.once('run', () => onRun(searcher))

  return searcher.run()
}

const keypress = async (
  searcher: HistorySearcher,
  keys: (string | number | [string, Keypress] | (() => void))[],
) => {
  for (const key of keys) {
    if (key instanceof Array) {
      await searcher.keypress(...key)

    } else if (key instanceof Function) {
      await key.call(searcher)

    } else if (['string', 'number'].includes(typeof key)) {
      await searcher.keypress(key)
    }
  }
}

/**
 * normalized and mock tty for CI runner
 * because in GitHub Action CI, stdin / stdout is not tty,
 * and also cannot read /dev/ttys
 */
beforeAll(() => {
  process.stdout.rows = 15

  process.stdout.columns = 80
  process.stdin.isTTY = true
  process.stdout.isTTY = true
  process.stdin.setRawMode = () => process.stdin
})


test('search author in history', async () => {
  const result = await searchHistory(
    'author',
    async (searcher) => {
      await searcher.submit()
    },
  )

  expect(result).toBe('echo author zthxxx')
})

test('keypress author for search', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        ['a', 'u', 't', 'h'],
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('echo author zthxxx')
})

test('base submit', async () => {
  const result = await searchHistory(
    undefined,
    async (searcher) => {
      await searcher.submit()
    },
  )

  expect(result).toBe('echo zsh-history-enquirer')
})


test('keypress down', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.down()
      await searcher.down()
      await searcher.submit()
    },
  )

  expect(result).toBe('pwgen --help')
})

test('keypress down scroll', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        Array(8).fill(searcher.down),
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('cd Documents')
})

test('keypress down scroll with small limit', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      searcher.options.limit = 4

      await keypress(
        searcher,
        Array(8).fill(searcher.down),
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('cd Documents')
})

test('keypress down and up', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.down()
      await searcher.down()
      await searcher.down()
      await searcher.up()
      await searcher.submit()
    },
  )

  expect(result).toBe('pwgen --help')
})

test('keypress up scroll', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.up()
      await searcher.up()
      await searcher.submit()
    },
  )

  expect(result).toBe('command-1')
})

test('keypress up scroll with multiline command', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        Array(8).fill(searcher.up),
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('command-7')
})

test('keypress pageDown', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.down()
      await searcher.pageDown()
      await searcher.submit()
    },
  )

  expect(result).toBe('cd Documents')
})

test('keypress pageUp', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.up()
      await searcher.pageUp()
      await searcher.submit()
    },
  )

  expect(result).toBe('command-7')
})

test('keypress home', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.down()
      await searcher.pageDown()
      await searcher.home()
      await searcher.submit()
    },
  )

  expect(result).toBe('echo zsh-history-enquirer')
})

test('keypress end', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await searcher.up()
      await searcher.pageUp()
      await searcher.end()
      await searcher.submit()
    },
  )

  expect(result).toBe('command-0')
})

test('keypress search command in history', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        [
          'c', 'o', 'm', 'm', 'a', 'n', 'd',
          searcher.down,
        ],
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('command-15')
})

test('keypress search and scroll', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        [
          'c', 'o', 'm', 'm', 'a', 'n', 'd',
          ...Array(11).fill(searcher.down),
        ],
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('command-5')
})

test('search command and scroll', async () => {
  const result = await searchHistory(
    'command',
    async (searcher) => {
      await keypress(
        searcher,
        Array(7).fill(searcher.down),
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('command-9')
})

test('search git', async () => {
  const result = await searchHistory(
    'git',
    async (searcher) => {
      await searcher.submit()
    },
  )

  expect(result).toBe('git status')
})

test('search command and scroll', async () => {
  const result = await searchHistory(
    'command',
    async (searcher) => {
      await keypress(
        searcher,
        Array(7).fill(searcher.down),
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('command-9')
})

test('search, scroll, press home', async () => {
  const result = await searchHistory(
    'command',
    async (searcher) => {
      await keypress(
        searcher,
        Array(7).fill(searcher.down),
      )
      await searcher.home()
      await searcher.submit()
    },
  )

  expect(result).toBe('echo earlier command')
})

test('search, scroll, press end', async () => {
  const result = await searchHistory(
    'command',
    async (searcher) => {
      await keypress(
        searcher,
        Array(10).fill(searcher.up),
      )
      await searcher.end()
      await searcher.submit()
    },
  )

  expect(result).toBe('command-0')
})

test('search multiple words', async () => {
  const result = await searchHistory(
    'log iso',
    async (searcher) => {
      await searcher.submit()
    },
  )

  expect(result).toBe('git log --pretty=fuller --date=iso -n 1')
})

test('paste text in terminal', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        [
          [null, pasteStart], '~',
          's', 't',
          [null, pasteEnd], '~',
          'a', 't',
        ],
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('git status')
})

test('input number and paste number in terminal', async () => {
  const result = await searchHistory(
    '',
    async (searcher) => {
      await keypress(
        searcher,
        [
          2, 3,
          [null, pasteStart], '~',
          3, 3,
          [null, pasteEnd], '~',
        ],
      )
      await searcher.submit()
    },
  )

  expect(result).toBe('233333')
})

test('cancel error', async () => {
  try {
    await searchHistory(
      '',
      async (searcher) => {
        await keypress(
          searcher,
          [2, 3, 3, 3],
        )
        await searcher.cancel('some error message')
      },
    )
    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('2333')
  }
})

test('cancel with ctrl+c', async () => {
  try {
    await searchHistory(
      '',
      async (searcher) => {
        await keypress(
          searcher,
          [
            2, 3, 3, 3,
            [ctrlC.sequence, ctrlC],
          ],
        )
      },
    )

    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('2333')
  }
})

test('cancel with esc', async () => {
  try {
    await searchHistory(
      '',
      async (searcher) => {
        await keypress(
          searcher,
          [
            2, 3, 3, 3,
            [esc.sequence, esc],
          ],
        )
      },
    )

    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('2333')
  }
})

test('search not match and cancel', async () => {
  try {
    await searchHistory(
      '3jdfn2-9jgf',
      async (searcher) => {
        await searcher.cancel()
      },
    )

    expect(true).toBe(false)
  } catch (result) {
    expect(result).toBe('3jdfn2-9jgf')
  }
})

test('search not match, but confirm', async () => {
  const result = await searchHistory(
    '3jdfn2-9jgf',
    async (searcher) => {
      await searcher.up()
      await searcher.down()
      await searcher.up()
      await searcher.submit()
    },
  )

  expect(result).toBe('3jdfn2-9jgf')
})
