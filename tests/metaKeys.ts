/**
 * recode
 * process.stdin = process.stdin.setRawMode()
 * process.stdin.on('keypress', (buf, key) => { console.log({buf, key}) })
 */

import type { Keypress } from '../src/historySearcher'

export const pasteStart: Keypress = {
  name: 'undefined',
  sequence: '\u001b[200',
  raw: '',
}

export const pasteEnd: Keypress = {
  name: 'undefined',
  sequence: '\u001b[201',
  raw: '',
}

export const ctrlC: Keypress = {
  sequence: '\x03',
  name: 'c',
  ctrl: true,
}

export const esc: Keypress = {
  sequence: '\x1B',
  name: 'escape',
  meta: true,
}
