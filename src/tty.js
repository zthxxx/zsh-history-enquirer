import fs from 'fs'
import tty from 'tty'


export const getStdin = () => new tty.ReadStream(fs.openSync('/dev/tty', 'r'))
export const getStdout = () => new tty.WriteStream(fs.openSync('/dev/tty', 'w'))
